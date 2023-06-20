package storage

import (
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var (
	b2KeyID       = os.Getenv("B2_KEY_ID")
	b2Key         = os.Getenv("B2_KEY")
	b2UploadKeyID = b2KeyID
	b2UploadKey   = b2Key
	// b2UploadKeyID = os.Getenv("B2_UPLOAD_KEY_ID")
	// b2UploadKey   = os.Getenv("B2_UPLOAD_KEY")
	b2BucketName = "intertube"
)

var FilesBucket = S3Bucket{
	S3:   newB2("us-west-002", "https://s3.us-west-002.backblazeb2.com", b2KeyID, b2Key),
	Name: b2BucketName,
}

var UploadsBucket = S3Bucket{
	S3:   newB2("us-west-002", "https://s3.us-west-002.backblazeb2.com", b2UploadKeyID, b2UploadKey),
	Name: b2BucketName + "-upload",
}

// AWS
var AWS_FilesBucket = S3Bucket{
	S3:   newS3("us-west-2"),
	Name: "files.inter.tube",
}

var ConfigBucket = S3Bucket{
	S3:   newS3("us-west-2"),
	Name: "private.inter.tube",
}

type S3Bucket struct {
	S3   *s3.S3
	Name string
}

func (b S3Bucket) Put(contentType, key string, r io.ReadSeeker) error {
	_, err := b.S3.PutObject(&s3.PutObjectInput{
		Body:        r,
		Bucket:      aws.String(b.Name),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	return err
}

func (b S3Bucket) PresignPut(key string, size int64, disp string) (string, error) {
	req, _ := b.S3.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(b.Name),
		Key:    aws.String(key),
		// ContentType: aws.String(contentType),
		ContentLength:      aws.Int64(size),
		ContentDisposition: aws.String(disp),
	})
	url, err := req.Presign(15 * time.Minute)
	return url, err
}

func (b S3Bucket) Delete(key string) error {
	_, err := b.S3.DeleteObject(&s3.DeleteObjectInput{
		Key: aws.String(key),
	})
	return err
}

func (b S3Bucket) Keys() ([]string, error) {
	var keys []string
	err := b.S3.ListObjectsV2Pages(&s3.ListObjectsV2Input{Bucket: &b.Name}, func(out *s3.ListObjectsV2Output, _ bool) bool {
		for _, c := range out.Contents {
			keys = append(keys, *c.Key)
		}
		return true
	})
	return keys, err
}

func (b S3Bucket) Get(key string) (io.ReadCloser, error) {
	out, err := b.S3.GetObject(&s3.GetObjectInput{Bucket: &b.Name, Key: &key})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (b S3Bucket) Exists(key string) bool {
	_, err := b.S3.HeadObject(&s3.HeadObjectInput{Bucket: &b.Name, Key: &key})
	// TODO actually check the error lol
	return err == nil
}

func (b S3Bucket) Copy(dst, src string) error {
	_, err := b.S3.CopyObject(&s3.CopyObjectInput{Bucket: &b.Name, CopySource: aws.String(b.Name + "/" + src), Key: &dst})
	return err
}

func (b S3Bucket) CopyFromBucket(dst string, srcBucket S3Bucket, src string) error {
	_, err := b.S3.CopyObject(&s3.CopyObjectInput{Bucket: &b.Name, CopySource: aws.String(srcBucket.Name + "/" + src), Key: &dst})
	return err
}

type S3Head struct {
	Type string
	Size int64
}

func (b S3Bucket) Head(key string) (S3Head, error) {
	head, err := b.S3.HeadObject(&s3.HeadObjectInput{Bucket: &b.Name, Key: &key})
	if err != nil {
		return S3Head{}, err
	}
	ret := S3Head{}
	if head.ContentType != nil {
		ret.Type = *head.ContentType
	}
	if head.ContentLength != nil {
		ret.Size = *head.ContentLength
	}
	return ret, nil
}

func (b S3Bucket) List(prefix string) (map[string]S3Head, error) {
	// var objs []interface{}
	objs := make(map[string]S3Head)
	err := b.S3.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(b.Name),
		Prefix: aws.String(prefix),
	}, func(out *s3.ListObjectsV2Output, _ bool) bool {
		for _, item := range out.Contents {
			objs[*item.Key] = S3Head{Size: *item.Size}
		}
		return true
	})
	return objs, err
}

func newB2(region, endpoint string, keyID, key string) *s3.S3 {
	// println(keyID, key)
	return s3.New(session.New(), &aws.Config{
		Region:      aws.String(region),
		Endpoint:    aws.String(endpoint),
		Credentials: credentials.NewStaticCredentials(keyID, key, ""),
		// S3ForcePathStyle: aws.Bool(true),
	})
}

func newS3(region string) *s3.S3 {
	return s3.New(session.New(), &aws.Config{
		Region: aws.String(region),
	})
}
