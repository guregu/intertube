package web

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/guregu/kami"
)

var (
	Domain    = "inter.tube"
	Deployed  time.Time
	DebugMode = false
)

func init() {
	kami.PanicHandler = PanicHandler
	http.Handle("/", kami.Handler())

	kami.Use("/", discover)
	kami.Use("/", allowGuest(
		"/login", "/register", "/forgot", "/recover",
		"/terms", "/privacy", "/buy/", "/subsonic",
		"/api/v0/login",
		"/external/stripe"))
	kami.Use("/", requireLogin)

	kami.Get("/", Index)
	kami.Get("/terms", termsOfService)
	kami.Get("/privacy", privacyPolicy)

	kami.Get("/login", loginForm)
	kami.Post("/login", login)
	kami.Post("/logout", logout)

	kami.Get("/register", registerForm)
	kami.Post("/register", register)

	kami.Get("/forgot", forgotForm)
	kami.Post("/forgot", forgot)

	kami.Get("/recover", recoverForm)
	kami.Post("/recover", doRecover)

	kami.Get("/upload", uploadForm)
	kami.Post("/upload/track", uploadStart)
	kami.Post("/upload/tracks", uploadStart2)
	kami.Post("/upload/track/:id", uploadFinish)

	kami.Get("/sync", syncForm)

	kami.Use("/music", cacheHeaders)
	kami.Get("/music", showMusic)
	kami.Head("/music", showMusicHead)
	kami.Use("/music/", cacheHeaders)
	kami.Get("/music/:kind", showMusic)
	kami.Head("/music/:kind", showMusicHead)

	kami.Delete("/track/:id", deleteTrack)
	kami.Post("/track/:id/played", incPlays)
	kami.Post("/track/:id/resume", setResume)
	kami.Get("/track/:id/edit", editTrackForm)
	kami.Post("/track/:id/edit", editTrack)

	kami.Get("/dl/tracks/:id", downloadTrack)

	kami.Get("/playlist/", createPlaylistForm)
	kami.Post("/playlist/", createPlaylist)
	kami.Get("/playlist/:id", createPlaylistForm)
	kami.Post("/playlist/:id", createPlaylist)

	kami.Post("/cache/reset", resetCache)

	kami.Get("/forum", forum)
	kami.Get("/more", moreStuff)
	kami.Get("/subsonic", subsonicHelp)

	kami.Use("/settings", ensureCustomer)
	kami.Get("/settings", settingsForm)
	kami.Post("/settings", settings)
	kami.Get("/settings/password", changePasswordForm)
	kami.Post("/settings/password", changePassword)
	kami.Use("/settings/payment", ensureCustomer)
	kami.Get("/settings/payment", stripePortal)

	kami.Use("/buy/", ensureCustomer)
	kami.Get("/buy/", buyForm)
	kami.Post("/buy/checkout", stripeCheckout)
	kami.Get("/buy/success", stripeCheckoutResult)

	// kami.Use("/payment/", requireLogin)
	// kami.Get("/payment/", stripePortal)

	kami.Use("/admin/", requireAdmin)
	kami.Get("/admin/", adminIndex)

	kami.Post("/external/stripe", stripeWebhook)
}

func init() {
	var dirty bool
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, kv := range info.Settings {
			switch kv.Key {
			case "vcs.time":
				var err error
				Deployed, err = time.Parse(time.RFC3339, kv.Value)
				if err != nil {
					panic(err)
				}
			case "vcs.modified":
				dirty = kv.Value == "true"
			}
		}
	}
	if !dirty && !Deployed.IsZero() {
		return
	}
	Deployed = time.Now().UTC()
}

func Load() {
	log.Println("Loading templates")
	templates = parseTemplates()
	log.Println("Loading translations")
	loadTranslations()
	// log.Println("Init B2")
	// initB2()
	// log.Println("Init CF")
	// initCF()
	log.Println("Init Stripe")
	initStripe()
	log.Println("Loaded up")
}
