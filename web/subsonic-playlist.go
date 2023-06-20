package web

import (
	"context"
	"encoding/xml"
	"net/http"
	"strconv"

	"github.com/guregu/intertube/tube"
)

type subsonicPlaylist struct {
	XMLName   xml.Name `xml:"playlist" json:"-"`
	ID        int      `xml:"id,attr" json:"id"`
	Name      string   `xml:"name,attr" json:"name"`
	Comment   string   `xml:"comment,attr,omitempty" json:"comment,omitempty"`
	Owner     string   `xml:"owner,attr,omitempty" json:"owner,omitempty"`
	Public    bool     `xml:"public,attr" json:"public"`
	SongCount int      `xml:"songCount,attr" json:"songCount"`
	Duration  int      `xml:"duration,attr" json:"duration"`
	Created   string   `xml:"created,attr" json:"created"`
	Changed   string   `xml:"changed,attr" json:"changed"`
	CoverArt  string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`

	Entries []subsonicSong `json:"entry,omitempty"`
}

func newSubsonicPlaylist(pl tube.Playlist, tracks []tube.Track, owner tube.User) subsonicPlaylist {
	list := subsonicPlaylist{
		ID:        pl.ID,
		Name:      pl.Name,
		Comment:   pl.Desc,
		Owner:     owner.Email,
		Public:    false,
		SongCount: len(pl.Tracks),
		Duration:  pl.Duration,
		Created:   pl.Date.Format(subsonicTimeLayout),
		Changed:   pl.LastMod.Format(subsonicTimeLayout),
	}
	for _, t := range tracks {
		if list.CoverArt == "" && t.Picture.ID != "" {
			list.CoverArt = t.TrackSSID().String()
		}
		list.Entries = append(list.Entries, newSubsonicSong(t, "entry"))
	}
	if len(tracks) == 0 && len(pl.Tracks) > 0 {
		list.CoverArt = tube.NewSSID(tube.SSIDTrack, pl.Tracks[0]).String()
	}
	return list
}

func subsonicGetPlaylists(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// <playlist id="15" name="Some random songs" comment="Just something I tossed together" owner="admin" public="false" songCount="6" duration="1391" created="2012-04-17T19:53:44" coverArt="pl-15">
	u, _ := userFrom(ctx)

	resp := struct {
		subsonicResponse
		Playlists struct {
			List []subsonicPlaylist `json:"playlist,omitempty"`
		} `xml:"playlists" json:"playlists"`
	}{
		subsonicResponse: subOK(),
	}

	pls, err := tube.GetPlaylists(ctx, u.ID)
	if err != nil {
		panic(err)
	}
	// lib, err := getLibrary(ctx, u)
	// if err != nil {
	// 	panic(err)
	// }

	for _, pl := range pls {
		// tracks := lib.TracksById(pl.Tracks)
		resp.Playlists.List = append(resp.Playlists.List, newSubsonicPlaylist(pl, nil, u))
	}

	writeSubsonic(ctx, w, r, resp)
}

func subsonicGetPlaylist(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	// id := tube.ParseSSID(r.FormValue("id"))
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		panic(err)
	}

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}
	pl, err := tube.GetPlaylist(ctx, u.ID, id)
	if err == tube.ErrNotFound {
		writeSubsonic(ctx, w, r, subErr(70, "The requested data was not found."))
		return
	} else if err != nil {
		panic(err)
	}
	tracks, err := playlistTracks(lib, pl)
	if err != nil {
		panic(err)
	}

	resp := struct {
		subsonicResponse
		Playlist subsonicPlaylist `xml:"playlist" json:"playlist"`
	}{
		subsonicResponse: subOK(),
		Playlist:         newSubsonicPlaylist(pl, tracks, u),
	}
	writeSubsonic(ctx, w, r, resp)
}

func subsonicCreatePlaylist(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	r.ParseForm()
	name := r.FormValue("name")
	// pid := tube.ParseSSID(r.FormValue("playlistId"))
	pid, _ := strconv.Atoi(r.FormValue("playlistId"))

	var ids []string
	for _, id := range r.Form["songId"] {
		ids = append(ids, tube.ParseSSID(id).ID)
	}

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}
	var tracks []tube.Track
	for _, id := range ids {
		t, ok := lib.TrackByID(id)
		if !ok {
			continue
		}
		tracks = append(tracks, t)
	}

	if pid != 0 {
		pl, err := tube.GetPlaylist(ctx, u.ID, pid)
		if err != nil {
			panic(err)
		}
		pl.With(tracks)
		if err := pl.Save(ctx); err != nil {
			panic(err)
		}
		return
	}

	pl := tube.Playlist{
		UserID: u.ID,
		Name:   name,
	}
	pl.With(tracks)
	if err := pl.Create(ctx); err != nil {
		panic(err)
	}

	resp := struct {
		subsonicResponse
		Playlist subsonicPlaylist `xml:"playlist" json:"playlist"`
	}{
		subsonicResponse: subOK(),
		Playlist:         newSubsonicPlaylist(pl, tracks, u),
	}
	writeSubsonic(ctx, w, r, resp)
}

func subsonicUpdatePlaylist(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	r.ParseForm()
	name := r.FormValue("name")
	desc := r.FormValue("comment")
	// TODO: public?

	// pid := tube.ParseSSID(r.FormValue("playlistId"))
	pid, err := strconv.Atoi(r.FormValue("playlistId"))
	if err != nil {
		panic(err)
	}

	var add []string
	for _, id := range r.Form["songIdToAdd"] {
		add = append(add, tube.ParseSSID(id).ID)
	}

	rem := make(map[int]struct{})
	for _, idx := range r.Form["songIndexToRemove"] {
		i, err := strconv.Atoi(idx)
		if err != nil {
			panic(err)
		}
		rem[i] = struct{}{}
	}

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}
	pl, err := tube.GetPlaylist(ctx, u.ID, pid)
	if err != nil {
		panic(err)
	}

	ids := make([]string, 0, len(pl.Tracks))
	for i, id := range pl.Tracks {
		if _, ok := rem[i]; ok {
			continue
		}
		ids = append(ids, id)
	}
	ids = append(ids, add...)

	tracks := lib.TracksByID(ids)
	pl.With(tracks)

	if name != "" {
		pl.Name = name
	}
	if desc != "" {
		pl.Desc = desc
	}

	if err := pl.Save(ctx); err != nil {
		panic(err)
	}

	resp := struct {
		subsonicResponse
		Playlist subsonicPlaylist `xml:"playlist" json:"playlist"`
	}{
		subsonicResponse: subOK(),
		Playlist:         newSubsonicPlaylist(pl, tracks, u),
	}
	writeSubsonic(ctx, w, r, resp)
}

func subsonicDeletePlaylist(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	pid, err := strconv.Atoi(r.FormValue("playlistId"))
	if err != nil {
		panic(err)
	}

	if err := tube.DeletePlaylist(ctx, u.ID, pid); err != nil {
		panic(err)
	}

	writeSubsonic(ctx, w, r, subOK())
}
