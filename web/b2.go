package web

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kurin/blazer/b2"
	"github.com/kurin/blazer/base"

	"github.com/guregu/intertube/storage"
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

func copyUploadToFiles(ctx context.Context, dstPath string, fileID string, f tube.File) error {
	disp := "attachment; filename*=UTF-8''" + escapeFilename(f.Name)
	return storage.FilesBucket.CopyFromBucket(dstPath, storage.UploadsBucket, f.Path(), f.Type, disp)
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

// TODO: get rid of this
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
	const illegal = `<>@,;:\"/+[]?={} 	`
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
