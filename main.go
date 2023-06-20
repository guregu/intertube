package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/guregu/intertube/event"
	"github.com/guregu/intertube/web"
)

var (
	domainFlag = flag.String("domain", "", "domain")
	bindFlag   = flag.String("addr", ":8000", "addr to bind on")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	flag.Parse()
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
			event.StartLambda(mode)
		case "CHANGE":
			event.StartLambda(mode)
		}
		return
	}

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
