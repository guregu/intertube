package tube

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
	"golang.org/x/sync/errgroup"
)

var dynamoTables = map[string]any{
	"Counters":  counter{},
	"Files":     File{},
	"Playlists": Playlist{},
	"Sessions":  Session{},
	"Stars":     Star{},
	"Tracks":    Track{},
	"Users":     User{},
}

var ErrNotFound = dynamo.ErrNotFound

var (
	dbPrefix string = "Tube-"
	db       *dynamo.DB
	useDump  = false
)

func Init(region, prefix, endpoint string, debug bool) {
	dbPrefix = prefix
	var err error
	var sesh *session.Session
	if endpoint == "" {
		sesh, err = session.NewSession()
	} else {
		sesh, err = session.NewSession(&aws.Config{
			Endpoint:    &endpoint,
			Credentials: credentials.NewStaticCredentials("dummy", "dummy", ""),
		})
		if region == "" {
			region = "local"
		}
	}
	if err != nil {
		panic(err)
	}
	cfg := &aws.Config{
		Region: &region,
	}
	if endpoint == "" && region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if debug {
		cfg.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}
	db = dynamo.New(sesh, cfg)
}

func dynamoTable(name string) dynamo.Table {
	return db.Table(dbPrefix + name)
}

type createTabler interface {
	CreateTable(*dynamo.CreateTable)
}

func CreateTables(ctx context.Context) error {
	log.Println("Checking DynamoDB tables... prefix =", dbPrefix)

	grp, ctx := errgroup.WithContext(ctx)
	for name, model := range dynamoTables {
		name := dynamoTable(name).Name()
		model := model

		if _, err := db.Table(name).Describe().RunWithContext(ctx); err == nil {
			continue
		}

		log.Println("Creating table:", name)
		grp.Go(func() error {
			create := db.CreateTable(name, model).OnDemand(true)
			if custom, ok := model.(createTabler); ok {
				custom.CreateTable(create)
			}
			return create.RunWithContext(ctx)
		})
	}
	return grp.Wait()
}

type counter struct {
	ID    string `dynamo:",hash"`
	Count int
}

func NextID(ctx context.Context, class string) (n int, err error) {
	var ct counter

	table := dynamoTable("Counters")
	err = table.Update("ID", class).Add("Count", 1).Value(&ct)
	return ct.Count, err
}

func init() {
	dynamo.RetryTimeout = 5 * time.Minute
}
