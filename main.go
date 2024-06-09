package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/guregu/intertube/storage"
	"github.com/guregu/intertube/tube"
	"github.com/guregu/intertube/web"
)

var (
	domainFlag = flag.String("domain", "", "domain")
	bindFlag   = flag.String("addr", ":8000", "addr to bind on")
	cfgFlag    = flag.String("cfg", "config.toml", "configuration file location")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	flag.Parse()

	if *cfgFlag != "" {
		cfg, err := readConfig(*cfgFlag)
		if err != nil {
			log.Fatalln("Failed to read config file:", *cfgFlag, "error:", err)
		}
		web.Domain = cfg.Domain

		tube.Init(cfg.DB.Region, cfg.DB.Prefix, cfg.DB.Endpoint, cfg.DB.Debug)

		storageCfg := storage.Config{
			Type:            storage.StorageType(cfg.Storage.Type),
			FilesBucket:     cfg.Storage.FilesBucket,
			UploadsBucket:   cfg.Storage.UploadsBucket,
			CacheBucket:     cfg.Storage.CacheBucket,
			AccessKeyID:     cfg.Storage.AccessKeyID,
			AccessKeySecret: cfg.Storage.AccessKeySecret,
			Region:          cfg.Storage.Region,
			Endpoint:        cfg.Storage.Endpoint,
			CFAccountID:     cfg.Storage.CloudflareAccount,
			SQSURL:          cfg.Queue.SQS,
			SQSRegion:       cfg.Queue.Region,
		}
		storage.Init(storageCfg)
	}

	if os.Getenv("LAMBDA_TASK_ROOT") != "" {
		// TODO: split these into separate binaries maybe
		mode := os.Getenv("MODE")
		log.Println("Lambda mode", mode)
		switch mode {
		case "WEB":
			// web server
			log.Println("deploy time:", web.Deployed)
			web.Load()
			startLambda()
		case "CHANGE", "FILE":
			startEventLambda(mode)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	if err := tube.CreateTables(ctx); err != nil {
		log.Fatalln("Failed to create tables:", err)
	}
	cancel()

	if *domainFlag != "" {
		web.Domain = *domainFlag
	}

	// web.MIGRATE_MAKEDUMPS()
	// os.Exit(0)

	// local server for dev
	log.Println("Build date:", web.Deployed)
	web.DebugMode = true
	web.Load()

	log.Println("Starting up local webserver at:", bindAddr())
	closeWatch := web.WatchFiles()
	if err := http.ListenAndServe(*bindFlag, nil); err != nil {
		panic(err)
	}
	closeWatch()
}

func bindAddr() string {
	addr := "http://"
	if strings.HasPrefix(*bindFlag, ":") {
		addr += "localhost"
	}
	addr += *bindFlag
	return addr
}
