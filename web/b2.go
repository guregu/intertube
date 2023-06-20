package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/kurin/blazer/b2"
	"github.com/kurin/blazer/base"

	"github.com/guregu/intertube/tube"
)

var (
	b2KeyID        = os.Getenv("B2_KEY_ID")
	b2Key          = os.Getenv("B2_KEY")
	b2BucketName   = "intertube"
	b2UploadBucket = "intertube-upload"

	b2UserDirFormat = "u/tracks/%d/"

	mainBucketID   string
	uploadBucketID string
)

var b2Client *b2.Client

func initB2() {
	ctx := context.Background()
	var err error
	b2Client, err = b2.NewClient(ctx, b2KeyID, b2Key)
	if err != nil {
		panic(err)
	}
	initBucketIDs()
}

type uploadInfo struct {
	URL    string
	Token  string
	ID     string
	Secret string
}

func initBucketIDs() {
	ctx := context.Background()
	baseClient, err := base.AuthorizeAccount(ctx, b2KeyID, b2Key)
	if err != nil {
		panic(err)
	}
	buckets, err := baseClient.ListBuckets(ctx)
	if err != nil {
		panic(err)
	}
	for _, b := range buckets {
		switch b.Name {
		case b2BucketName:
			mainBucketID = b.ID
		case b2UploadBucket:
			uploadBucketID = b.ID
		}
	}
	fmt.Println("BUCKET IDS", mainBucketID, uploadBucketID)
}

func getUploadURL(ctx context.Context, u tube.User) (uploadInfo, error) {
	keyname := fmt.Sprintf("up-%d-%d", u.ID, time.Now().UTC().Unix())
	key, err := b2Client.CreateKey(ctx, keyname, b2.Capabilities("writeFiles", "listBuckets"), b2.Lifetime(120*time.Minute))
	if err != nil {
		return uploadInfo{}, err
	}

	info := uploadInfo{
		ID:     key.ID(),
		Secret: key.Secret(),
	}

	baseClient, err := base.AuthorizeAccount(ctx, key.ID(), key.Secret())
	if err != nil {
		return uploadInfo{}, err
	}

	// var buckets []*base.Bucket
	buckets, err := baseClient.ListBuckets(ctx)
	if err != nil {
		panic(err)
	}

	var uploadBucketAPI *base.Bucket
	for _, b := range buckets {
		if b.Name == b2UploadBucket {
			uploadBucketAPI = b
			break
		}
	}
	if uploadBucketAPI == nil {
		return uploadInfo{}, fmt.Errorf("couldn't find upload bucket")
	}

	uploadURL, err := uploadBucketAPI.GetUploadURL(ctx)
	if err != nil {
		return uploadInfo{}, err
	}

	// TODO: FUCK dumb hack
	rv := reflect.ValueOf(uploadURL).Elem()
	uri := rv.FieldByName("uri").String()
	token := rv.FieldByName("token").String()

	info.URL = uri
	info.Token = token

	return info, nil
}

func copyUploadToMain(ctx context.Context, dstPath string, fileID string, f tube.File) error {
	baseClient, err := base.AuthorizeAccount(ctx, b2KeyID, b2Key)
	if err != nil {
		return err
	}
	// TODO: omg this is horrible
	rv := reflect.ValueOf(baseClient).Elem()
	authToken := rv.FieldByName("authToken").String()
	apiURL := rv.FieldByName("apiURI").String()

	disp := "attachment; filename*=UTF-8''" + escapeFilename(f.Name)

	var input = struct {
		Src               string            `json:"sourceFileId"`
		DestBucket        string            `json:"destinationBucketId"`
		DestPath          string            `json:"fileName"`
		MetadataDirective string            `json:"metadataDirective"`
		ContentType       string            `json:"contentType"`
		FileInfo          map[string]string `json:"fileInfo"`
	}{
		Src:               fileID,
		DestBucket:        mainBucketID,
		DestPath:          dstPath,
		MetadataDirective: "REPLACE",
		ContentType:       f.Type,
		FileInfo: map[string]string{
			"b2-content-disposition": disp,
		},
	}

	body, err := json.Marshal(input)
	if err != nil {
		return err
	}

	url := apiURL + "/b2api/v2/b2_copy_file"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		msg, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("ERROPR", string(msg))
		return fmt.Errorf("copy error: %d", resp.StatusCode)
	}

	return nil
}

func createB2Token(ctx context.Context, userID int) (token string, expires time.Time, err error) {
	bucket, err := b2Client.Bucket(ctx, b2BucketName)
	if err != nil {
		return "", time.Time{}, err
	}
	pathname := fmt.Sprintf(b2UserDirFormat, userID)
	token, err = bucket.AuthToken(ctx, pathname, time.Hour*24)
	expires = time.Now().UTC().Add(time.Hour * 23)

	return token, expires, err
}

func b2DownloadURL(u tube.User, track tube.Track) string {
	href := fmt.Sprintf(cfFileURL, track.B2Key(), u.B2Token)
	return href
}

func GenPicAuthKey() (string, error) {
	ctx := context.Background()
	// b2Client.CreateKey(ctx, "interpix", b2.Prefix("pic/"), b2.)
	bucket, err := b2Client.Bucket(ctx, b2BucketName)
	if err != nil {
		return "", err
	}
	token, err := bucket.AuthToken(ctx, "pic/", time.Hour*24*7)
	return token, err
}

func escapeFilename(name string) string {
	const illegal = `<>@,;:\"/[]?={} 	`
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(illegal, r) {
			return '-'
		}
		return r
	}, name)
	name = url.PathEscape(name)
	if len(name) == 0 {
		return "file"
	}
	return name
}
