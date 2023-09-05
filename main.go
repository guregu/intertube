package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/guregu/intertube/storage"
	"github.com/guregu/intertube/tube"
	"github.com/guregu/intertube/web"
	"github.com/kardianos/osext"
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
			deployed := loadDeploydate()
			web.Deployed = deployed
			log.Println("deploy time:", deployed)
			web.Load()
			startLambda()
		case "REFRESH":
			web.Load()
			startEventLambda(mode)
		case "CHANGE":
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
	// log.Println(web.TEST_HANDLEUPLOAD())
	// os.Exit(0)

	// local server for dev
	log.Println("deploydate:", loadDeploydate())
	web.DebugMode = true
	web.Load()

	log.Println("Starting up local webserver on port 8000")
	closeWatch := web.WatchFiles()
	if err := http.ListenAndServe(*bindFlag, nil); err != nil {
		panic(err)
	}
	closeWatch()
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
