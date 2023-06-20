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
	if err := getTemplate(ctx, "index").Execute(w, data); err != nil {
		panic(err)
	}
}

func forum(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	data := struct {
		User tube.User
	}{
		User: u,
	}
	if err := getTemplate(ctx, "forum").Execute(w, data); err != nil {
		panic(err)
	}
}

func moreStuff(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	data := struct {
		User tube.User
	}{
		User: u,
	}
	if err := getTemplate(ctx, "more").Execute(w, data); err != nil {
		panic(err)
	}
}

func subsonicHelp(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	data := struct {
		User tube.User
	}{
		User: u,
	}
	if err := getTemplate(ctx, "subsonic").Execute(w, data); err != nil {
		panic(err)
	}
}

func privacyPolicy(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if err := getTemplate(ctx, "privacy").Execute(w, nil); err != nil {
		panic(err)
	}
}

func termsOfService(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if err := getTemplate(ctx, "terms").Execute(w, nil); err != nil {
		panic(err)
	}
}
