package email

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

const noreply = " <noreply@inter.tube>"

var mailer *ses.SES

// TODO: make this configurable later
// for now just fail without exploding

func init() {
	sesh, err := session.NewSession()
	if err != nil {
		log.Println("email is not configured:", err)
		return
	}
	mailer = ses.New(sesh, &aws.Config{
		Region: aws.String("us-west-2"),
	})
}

func Send(from, to, subject, content string) error {
	input := &ses.SendEmailInput{
		Source: aws.String(from + noreply),
		Destination: &ses.Destination{
			ToAddresses: []*string{aws.String(to)},
		},
		Message: &ses.Message{
			Subject: &ses.Content{
				Data:    aws.String(subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &ses.Body{
				Html: &ses.Content{
					Data:    aws.String(content),
					Charset: aws.String("UTF-8"),
				},
			},
		},
	}
	_, err := mailer.SendEmail(input)
	return err
}

func IsEnabled() bool {
	return mailer != nil
}
