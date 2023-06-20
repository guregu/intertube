package event

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudflare/cloudflare-go"

	"github.com/guregu/intertube/web"
)

// refresh b2 key in cloudflare kv store
func HandleRefresh() (string, error) {
	token, err := web.GenPicAuthKey()
	if err != nil {
		return "ERROR", err
	}

	accountID := os.Getenv("CF_ACCOUNT")
	apiKey := os.Getenv("CF_API_KEY")
	email := os.Getenv("CF_API_EMAIL")
	kvID := os.Getenv("CF_KV_NAMESPACE")
	_ = email
	api, err := cloudflare.NewWithAPIToken(apiKey)
	if err != nil {
		panic(err)
	}

	// fmt.Println("token", token)
	// fmt.Println("account", accountID)
	// fmt.Println("api key", apiKey)
	// fmt.Println("email", email)
	// fmt.Println("kv id", kvID)

	ctx := context.Background()
	resp, err := api.WriteWorkersKVEntry(ctx,
		cloudflare.AccountIdentifier(accountID),
		cloudflare.WriteWorkersKVEntryParams{
			Key:         "pickey",
			Value:       []byte(token),
			NamespaceID: kvID,
		})
	if err != nil {
		panic(err)
	}
	msg := "ok"
	if !resp.Success {
		msg = fmt.Sprintln(resp.Errors, resp.Messages)
	}
	return msg, nil
}
