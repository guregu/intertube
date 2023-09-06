package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/guregu/intertube/tube"
)

func createPlaylistForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}
	var tracks []tube.Track
	query := r.FormValue("q")
	if query != "" {
		var err error
		tracks, err = lib.Query(query)
		if err != nil {
			panic(err)
		}
	}

	data := struct {
		Tracks   []tube.Track
		Query    string
		Playlist tube.Playlist
	}{
		Tracks: tracks,
		Query:  query,
	}

	page := "playlist"
	if r.FormValue("frag") == "tracks" {
		page = "playlist_tracks"
	}

	renderTemplate(ctx, w, page, data, http.StatusOK)
}

/*
{"meta":{"playlist-name":"a","sort-by":"default","sort-order":"ascending"},"form":{"all":[{"inc":true,"attr":"Artist","op":"$1 == $2","val":"\"a\"","expr":"(Artist == \"a\")"}],"expr":"(Artist == \"a\")"}}
*/
type PlaylistRequest struct {
	Meta struct {
		Name      string
		SortBy    string
		SortOrder string
	}
	Form struct {
		All  []PlaylistCond
		Expr string
	}
}

type PlaylistCond struct {
	Include bool   `json:"inc"`
	Attr    string `json:"attr"`
	Op      string `json:"op"`
	Val     string `json:"val"`
}

type PLUIMeta struct {
	Conds []PlaylistCond
	Ver   int
}

// TODO: static playlist
func createPlaylist(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	const playlistVersion = 1

	u, _ := userFrom(ctx)
	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}

	var plr PlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&plr); err != nil {
		panic(err)
	}
	expr := plr.Form.Expr

	pl := tube.Playlist{
		UserID: u.ID,
		Name:   plr.Meta.Name,

		Dynamic: true,
		Query:   expr,
	}

	// test out query
	tracks, err := lib.Query(expr)
	if err != nil {
		panic(err)
	}
	pl.With(tracks)

	enc, err := json.Marshal(PLUIMeta{
		Conds: plr.Form.All,
		Ver:   playlistVersion,
	})
	if err != nil {
		panic(err)
	}
	pl.UIMeta = enc

	if err := pl.Create(ctx); err != nil {
		panic(err)
	}
	w.Header().Set("Location", fmt.Sprintf("/playlist/%d", pl.ID))
	w.WriteHeader(http.StatusCreated)
}

func playlistTracks(lib *Library, pl tube.Playlist) ([]tube.Track, error) {
	var tracks []tube.Track
	var err error
	if pl.Dynamic {
		tracks, err = lib.Query(pl.Query)
	} else {
		tracks = lib.TracksByID(pl.Tracks)
	}
	// TODO: SORT
	return tracks, err
}
