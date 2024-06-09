package storage

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

var fileQueue *sqs.SQS
var fileQueueURL string

func UseSQS(region, href string) {
	client := sqs.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	})))
	fileQueue = client
	fileQueueURL = href
}

func UsingQueue() bool {
	return fileQueue != nil
}

type FileEvent struct {
	UserID int
	FileID string
	Path   string
}

func QueueAck(msgid string) error {
	if !UsingQueue() {
		return fmt.Errorf("not using queue")
	}
	_, err := fileQueue.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &fileQueueURL,
		ReceiptHandle: &msgid,
	})
	return err
}

func EnqueueFile(event FileEvent) error {
	if !UsingQueue() {
		return fmt.Errorf("not using queue")
	}

	bs, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fileQueue.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    &fileQueueURL,
		MessageBody: aws.String(string(bs)),
	})
	return err
}
