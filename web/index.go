package web

import (
	"context"
	"net/http"

	"github.com/guregu/intertube/tube"
)

func Index(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	data := struct {
		User tube.User
	}{
		User: u,
	}
	renderTemplate(ctx, w, "index", data, http.StatusOK)
}

func forum(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	data := struct {
		User tube.User
	}{
		User: u,
	}
	renderTemplate(ctx, w, "forum", data, http.StatusOK)
}

func moreStuff(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	data := struct {
		User tube.User
	}{
		User: u,
	}
	renderTemplate(ctx, w, "more", data, http.StatusOK)
}

func subsonicHelp(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	data := struct {
		User tube.User
	}{
		User: u,
	}
	renderTemplate(ctx, w, "subsonic", data, http.StatusOK)
}

func privacyPolicy(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	renderTemplate(ctx, w, "privacy", nil, http.StatusOK)
}

func termsOfService(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	renderTemplate(ctx, w, "terms", nil, http.StatusOK)
}
