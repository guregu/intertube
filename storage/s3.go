package storage

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var (
	FilesBucket   S3Bucket
	UploadsBucket S3Bucket
	CacheBucket   S3Bucket
)

type S3Bucket struct {
	S3   *s3.S3
	Name string
	Type StorageType
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

func (b S3Bucket) PresignPut(key string, size int64, disp string, ttl time.Duration) (string, error) {
	req, _ := b.S3.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(b.Name),
		Key:    aws.String(key),
		// ContentType: aws.String(contentType),
		ContentLength:      aws.Int64(size),
		ContentDisposition: aws.String(disp),
	})
	url, err := req.Presign(ttl)
	return url, err
}

func (b S3Bucket) PresignGet(key string, ttl time.Duration) (string, error) {
	req, _ := b.S3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(b.Name),
		Key:    aws.String(key),
	})
	url, err := req.Presign(ttl)
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

func (b S3Bucket) CopyFromBucket(dst string, srcBucket S3Bucket, src string, mime, contentDisp string) error {
	copySrc := srcBucket.Name + "/" + src
	_, err := b.S3.CopyObject(&s3.CopyObjectInput{
		Bucket:             &b.Name,
		CopySource:         &copySrc,
		Key:                &dst,
		ContentType:        &mime,
		ContentDisposition: &contentDisp,
	})
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

func newB2(region string, keyID, key string) *s3.S3 {
	endpoint := fmt.Sprintf("https://s3.%s.backblazeb2.com", region)
	return s3.New(session.Must(session.NewSession(&aws.Config{
		Region:           aws.String(region),
		Endpoint:         aws.String(endpoint),
		Credentials:      credentials.NewStaticCredentials(keyID, key, ""),
		S3ForcePathStyle: aws.Bool(true),
		Retryer:          Retryer{},
	})))
}

func newR2(accountID string, keyID, key string) *s3.S3 {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	return s3.New(session.Must(session.NewSession(&aws.Config{
		Region:      aws.String("auto"),
		Endpoint:    aws.String(endpoint),
		Credentials: credentials.NewStaticCredentials(keyID, key, ""),
		Retryer:     Retryer{},
	})))
}

func newS3(region, key, secret, endpoint string) *s3.S3 {
	cfg := &aws.Config{
		Region:  aws.String(region),
		Retryer: Retryer{},
	}
	if key != "" && secret != "" {
		cfg.Credentials = credentials.NewStaticCredentials(key, secret, "")
	}
	if endpoint != "" {
		cfg.Endpoint = &endpoint
		cfg.S3ForcePathStyle = aws.Bool(true)
	}
	return s3.New(session.Must(session.NewSession(cfg)))
}

var (
	S3Region   = "us-west-2"
	S3Endpoint string

	S3AccessKeyID     string
	S3AccessKeySecret string

	// for R2
	CFAccountID string
)

type Config struct {
	Type StorageType

	FilesBucket   string
	UploadsBucket string
	CacheBucket   string

	Region   string
	Endpoint string

	AccessKeyID     string
	AccessKeySecret string

	// for R2
	CFAccountID string

	// for SQS
	SQSURL    string
	SQSRegion string
}

type StorageType string

const (
	StorageTypeS3 StorageType = "s3"
	StorageTypeB2 StorageType = "b2"
	StorageTypeR2 StorageType = "r2"
	// StorageTypeFS StorageType = "fs"
)

func Init(cfg Config) {
	var client *s3.S3
	awsClient := newS3("us-west-2", "", "", "")
	switch cfg.Type {
	case StorageTypeS3:
		client = newS3(cfg.Region, cfg.AccessKeyID, cfg.AccessKeySecret, cfg.Endpoint)
	case StorageTypeB2:
		client = newB2(cfg.Region, cfg.AccessKeyID, cfg.AccessKeySecret)
	case StorageTypeR2:
		client = newR2(cfg.CFAccountID, cfg.AccessKeyID, cfg.AccessKeySecret)
	case "":
		panic(fmt.Errorf("missing storage.type in configuration"))
	default:
		panic(fmt.Errorf("unknown storage.type in configuration: %q", cfg.Type))
	}

	FilesBucket = S3Bucket{
		Name: cfg.FilesBucket,
		S3:   client,
		Type: cfg.Type,
	}

	UploadsBucket = S3Bucket{
		Name: cfg.UploadsBucket,
		S3:   client,
		Type: cfg.Type,
	}

	if cfg.CacheBucket != "" {
		CacheBucket = S3Bucket{
			Name: cfg.CacheBucket,
			S3:   awsClient,
			Type: StorageTypeS3,
		}
	}

	if cfg.SQSURL != "" {
		UseSQS(cfg.SQSRegion, cfg.SQSURL)
	}
}

func IsCacheEnabled() bool {
	return CacheBucket.Type != ""
}
