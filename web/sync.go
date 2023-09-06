package web

import (
	"context"
	"net/http"
	"sort"

	"github.com/guregu/intertube/tube"
)

func syncForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	// test lol
	tracks, err := u.GetTracks(ctx)
	if err != nil {
		panic(err)
	}
	lib := NewLibrary(tracks, nil)
	type meta struct {
		ID      string
		Size    int
		LastMod int64
		Path    string
		URL     string
	}
	metadata := make([]meta, 0, len(lib.tracks))
	index := make(map[string]meta, len(lib.tracks))
	for _, t := range lib.Tracks(organize{}) {
		m := meta{
			ID:      t.ID,
			Size:    t.Size,
			LastMod: t.LocalMod,
			Path:    t.VirtualPath(),
			URL:     presignTrackDL(u, t),
		}
		metadata = append(metadata, m)
		index[m.ID] = m
	}
	sort.Slice(metadata, func(i, j int) bool {
		return metadata[i].Path < metadata[j].Path
	})

	data := struct {
		User     tube.User
		Metadata []meta
		Index    map[string]meta
	}{
		User:     u,
		Metadata: metadata,
		Index:    index,
	}
	renderTemplate(ctx, w, "sync", data, http.StatusOK)
}
