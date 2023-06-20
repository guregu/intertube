package tube

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"

	"github.com/timshannon/badgerhold"
)

func openDB(name string) (*badgerhold.Store, error) {
	options := badgerhold.DefaultOptions
	// options.
	store, err := badgerhold.Open(options)
	return store, err
}

const regionDB = "us-west-2"

func init() {
	dynamo.RetryTimeout = 5 * time.Minute
}

var db = dynamo.New(session.New(), &aws.Config{
	Region: aws.String(regionDB),
	// LogLevel: aws.LogLevel(aws.LogDebugWithHTTPBody),
})

var ErrNotFound = dynamo.ErrNotFound

func dynamoTable(name string) dynamo.Table {
	return db.Table("Tube-" + name)
}

func NextID(ctx context.Context, class string) (n int, err error) {
	var counter struct {
		ID    string
		Count int
	}

	table := dynamoTable("Counters")
	err = table.Update("ID", class).Add("Count", 1).Value(&counter)
	return counter.Count, err
}

func IsCondCheckErr(err error) bool {
	if ae, ok := err.(awserr.Error); ok && ae.Code() == "ConditionalCheckFailedException" {
		return true
	}
	return false
}
