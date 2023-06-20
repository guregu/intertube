package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/guregu/dynamo"

	"github.com/guregu/intertube/tube"
)

type trackChange struct {
	tracks  []tube.Track
	deletes []string
	lastmod time.Time
}

// materialize track DB changes
func handleChange(ctx context.Context, e events.DynamoDBEvent) (string, error) {
	// lc, _ := lambdacontext.FromContext(ctx)
	changes := make(map[int64]*trackChange)
	for _, rec := range e.Records {
		fmt.Println(rec)
		// TODO: maybe ignore Continue time
		at := rec.Change.ApproximateCreationDateTime.UTC()
		userID, err := rec.Change.Keys["UserID"].Integer()
		if err != nil {
			log.Println("BAD KEY???", rec.Change.Keys)
			log.Println(rec.Change)
			continue
		}
		ch, ok := changes[userID]
		if !ok {
			ch = &trackChange{}
			changes[userID] = ch
		}
		if at.After(ch.lastmod) {
			ch.lastmod = at
		}
		switch rec.EventName {
		case "INSERT", "MODIFY":
			var track tube.Track
			if err := dynamo.UnmarshalItem(transmute(rec.Change.NewImage), &track); err != nil {
				panic(err)
			}
			ch.tracks = append(ch.tracks, track)
		case "REMOVE":
			ch.deletes = append(ch.deletes, rec.Change.Keys["ID"].String())
		}
	}

	if len(changes) == 0 {
		return "nothing to do", nil
	}

	var wg sync.WaitGroup
	errflag := new(int32)
	for uID, ch := range changes {
		uID, ch := uID, ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := tube.RefreshDump(ctx, int(uID), ch.lastmod, ch.tracks, ch.deletes)
			if err != nil {
				log.Println("ERROR:", err)
				atomic.AddInt32(errflag, 1)
			}
		}()
	}
	wg.Wait()
	if errs := atomic.LoadInt32(errflag); errs > 0 {
		return fmt.Sprintf("got %d update error(s)", errs), fmt.Errorf("update failed")
	}

	return "OK!", nil
}

func transmute(garb map[string]events.DynamoDBAttributeValue) map[string]*dynamodb.AttributeValue {
	// this is dumb
	v, _ := json.Marshal(garb)
	var item map[string]*dynamodb.AttributeValue
	if err := json.Unmarshal(v, &item); err != nil {
		panic(err)
	}
	return item
}
