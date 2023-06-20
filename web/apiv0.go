package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/guregu/dynamo"
	"github.com/guregu/kami"

	"github.com/guregu/intertube/tube"
)

func init() {
	kami.Post("/api/v0/login", loginV0)

	kami.Use("/api/v0/tracks/", requireLogin)
	kami.Get("/api/v0/tracks/", listTracksV0)
}

func loginV0(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string
		Password string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		panic(err)
	}

	email := req.Email
	pass := req.Password

	user, err := tube.GetUserByEmail(ctx, email)
	if err == tube.ErrNotFound {
		panic("no user with that email")
	}
	if err != nil {
		panic(err)
	}

	if !user.ValidPassword(pass) {
		panic("bad password")
	}

	sesh, err := tube.CreateSession(ctx, user.ID, ipAddress(r))
	if err != nil {
		panic(err)
	}

	http.SetCookie(w, validAuthCookie(sesh))

	data := struct {
		Session string
	}{
		Session: sesh.Token,
	}

	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}

func listTracksV0(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	if err := refreshB2Token(ctx, &u, 12*time.Hour); err != nil {
		panic(err)
	}

	var startFrom dynamo.PagingKey
	if start := r.URL.Query().Get("start"); start != "" {
		startFrom, _ = dynamo.MarshalItem(struct {
			UserID int
			ID     string
		}{
			UserID: u.ID,
			ID:     start,
		})
	}

	data := struct {
		Tracks tube.Tracks
		Next   string
	}{}

	tracks, next, err := tube.GetTracksPartial(ctx, u.ID, 500, startFrom)
	if err != nil {
		panic(err)
	}
	data.Tracks = tracks
	for i, t := range data.Tracks {
		t.DL = b2DownloadURL(u, t)
		data.Tracks[i] = t
	}
	if next != nil {
		data.Next = *next["ID"].S
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}
