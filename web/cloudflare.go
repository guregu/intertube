package web

import (
	"log"
	"os"

	"github.com/cloudflare/cloudflare-go"
)

var cloudflareAPI *cloudflare.API

func initCF() {
	account := os.Getenv("CF_ACCOUNT")
	apiKey := os.Getenv("CF_API_KEY")
	// email := os.Getenv("CF_API_EMAIL")

	log.Println("Loading Cloudflare", account)

	api, err := cloudflare.NewWithAPIToken(apiKey, cloudflare.UsingAccount(account))
	if err != nil {
		panic(err)
	}
	cloudflareAPI = api

	log.Println("Cloudflare OK")
}

func newCFUpload() {
	// cloudflareAPI.Upload
}
