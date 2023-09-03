//go:build lambda

package main

import (
	"net/http"

	"github.com/akrylysov/algnhsa"
	"github.com/guregu/intertube/event"
	// "github.com/aws/aws-lambda-go/lambda"
)

func startLambda() {
	algnhsa.ListenAndServe(http.DefaultServeMux, nil)
}

func startEventLambda(mode string) {
	event.StartLambda(mode)
}
