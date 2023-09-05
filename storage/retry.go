package storage

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

// See: https://future-architect.github.io/articles/20211026a/

type Retryer struct {
	client.DefaultRetryer
}

func (r Retryer) ShouldRetry(req *request.Request) bool {
	if origErr := req.Error; origErr != nil {
		switch origErr.(type) {
		case interface{ Temporary() bool }:
			if isErrConnectionReset(origErr) {
				return true
			}
		}
	}
	return r.DefaultRetryer.ShouldRetry(req)
}

func isErrConnectionReset(err error) bool {
	if strings.Contains(err.Error(), "read: connection reset") {
		return false
	}

	if strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "broken pipe") {
		return true
	}

	return false
}
