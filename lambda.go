package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/akrylysov/algnhsa"
	// "github.com/aws/aws-lambda-go/lambda"
	"github.com/kardianos/osext"
)

func startLambda() {
	algnhsa.ListenAndServe(http.DefaultServeMux, nil)
}

func loadDeploydate() time.Time {
	here, err := osext.ExecutableFolder()
	if err != nil {
		panic(err)
	}

	f, err := os.Open(filepath.Join(here, "deploydate"))
	if err != nil {
		fmt.Println("deploydate load error:", err)
		return time.Time{}
	}
	defer f.Close()

	var date int64
	fmt.Fscanf(f, "%d", &date)
	return time.Unix(date, 0)
}
