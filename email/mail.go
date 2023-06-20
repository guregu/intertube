package email

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

const noreply = " <noreply@inter.tube>"

var mailer = ses.New(session.New(), &aws.Config{
	Region: aws.String("us-west-2"),
})

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
