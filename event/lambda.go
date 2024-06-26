package event

import (
	"github.com/aws/aws-lambda-go/lambda"
)

func StartLambda(mode string) {
	switch mode {
	case "CHANGE":
		lambda.Start(handleChange)
	case "FILE":
		lambda.Start(handleFileQueue)
	}
	panic("unhandled mode: " + mode)
}
