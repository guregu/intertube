package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/guregu/intertube/storage"
	"github.com/guregu/intertube/tube"
	"github.com/guregu/intertube/web"
)

func handleFileQueue(ctx context.Context, e events.SQSEvent) (string, error) {
	for _, rec := range e.Records {
		var fe storage.FileEvent
		if err := json.Unmarshal([]byte(rec.Body), &fe); err != nil {
			return "", err
		}

		u, err := tube.GetUser(ctx, fe.UserID)
		if err != nil {
			return "", err
		}

		f, err := tube.GetFile(ctx, fe.FileID)
		if err != nil {
			return "", err
		}

		if _, err := web.ProcessUpload(ctx, &f, u, fe.Path); err != nil {
			return "", err
		}

		if err := storage.QueueAck(rec.ReceiptHandle); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("processed %d event(s)", len(e.Records)), nil
}
