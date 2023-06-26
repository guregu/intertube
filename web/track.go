package web

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guregu/dynamo"
	"github.com/guregu/kami"

	"github.com/guregu/intertube/tube"
)

func deleteTrack(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	trackID := kami.Param(ctx, "id")

	if err := tube.DeleteTrack(ctx, u.ID, trackID); err != nil {
		panic(err)
	}
	if err := u.UpdateLastMod(ctx); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "deleted "+trackID)
}

func incPlays(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	trackID := kami.Param(ctx, "id")
	secs, _ := strconv.ParseFloat(r.FormValue("duration"), 64)

	track, err := tube.GetTrack(ctx, u.ID, trackID)
	if err != nil {
		panic(err)
	}

	if track.Duration == 0 && secs > 0 {
		if err := track.SetDuration(ctx, int(secs)); err != nil && !dynamo.IsCondCheckFailed(err) {
			panic(err)
		}
	}

	if err := track.IncPlays(ctx); err != nil {
		panic(err)
	}

	if err := tube.IncTotalPlays(ctx, int(secs)); err != nil {
		panic(err)
	}

	// TODO: hmm...
	// if err := u.UpdateLastMod(ctx); err != nil {
	// panic(err)
	// }

	fmt.Fprint(w, track.Plays)
}

func setResume(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	trackID := kami.Param(ctx, "id")
	secs, _ := strconv.ParseFloat(r.FormValue("cur"), 64)  // track current position
	at, _ := strconv.ParseInt(r.FormValue("time"), 10, 64) // unix time in msec
	mod := msec2time(at)

	track, err := tube.GetTrack(ctx, u.ID, trackID)
	if err != nil {
		if err == tube.ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		panic(err)
	}

	if err := track.SetResume(ctx, secs, mod); err != nil {
		if dynamo.IsCondCheckFailed(err) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		panic(err)
	}

	if err := u.UpdateLastMod(ctx); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func editTrackForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	// trackID, _ := strconv.Atoi(kami.Param(ctx, "id"))
	id := kami.Param(ctx, "id")
	ids := strings.Split(id, ",")
	t, tracks, err := getMultiTracks(ctx, u, ids)
	if err != nil {
		panic(err)
	}
	multi := len(tracks) > 1

	// allTracks, err := tube.GetTracks(ctx, u.ID)
	// if err != nil && err != tube.ErrNotFound {
	// 	panic(err)
	// }
	// group := groupTracks(allTracks, true)

	data := struct {
		User     tube.User
		IDs      string
		Track    tube.Track
		Tracks   tube.Tracks
		Multi    bool
		LastMod  int64
		ErrorMsg string
	}{
		User:    u,
		IDs:     id,
		Track:   t,
		Tracks:  tracks,
		Multi:   multi,
		LastMod: u.LastMod.UnixNano(),
	}
	if err := getTemplate(ctx, "track-edit").Execute(w, data); err != nil {
		panic(err)
	}
}

func editTrack(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	id := kami.Param(ctx, "id")
	ids := strings.Split(id, ",")
	t, tracks, err := getMultiTracks(ctx, u, ids)
	if err != nil {
		panic(err)
	}
	multi := len(tracks) > 1

	// allTracks, err := tube.GetTracks(ctx, u.ID)
	// if err != nil && err != tube.ErrNotFound {
	// 	panic(err)
	// }
	// group := groupTracks(allTracks, true)

	renderError := func(err error) {
		var data = struct {
			User     tube.User
			IDs      string
			Track    tube.Track
			Tracks   tube.Tracks
			Multi    bool
			LastMod  int64
			ErrorMsg string
		}{
			User:     u,
			IDs:      id,
			Track:    t,
			Tracks:   tracks,
			Multi:    multi,
			LastMod:  u.LastMod.UnixNano(),
			ErrorMsg: err.Error(),
		}
		if err := getTemplate(ctx, "track-edit").Execute(w, data); err != nil {
			panic(err)
		}
	}
	_ = renderError

	newPic := t.Picture
	picdel := false
	if r.FormValue("picdel") == "on" {
		newPic = tube.Picture{}
		picdel = true
	} else {
		f, fh, err := r.FormFile("pic")
		if err != http.ErrMissingFile && err != nil {
			renderError(err)
			return
		}
		if err == nil {
			data, err := ioutil.ReadAll(f)
			if err != nil {
				renderError(err)
				return
			}
			ext := path.Ext(fh.Filename)
			if len(ext) > 0 && ext[0] == '.' {
				ext = ext[1:]
			}
			pic, err := savePic(data, ext, fh.Header.Get("Content-Type"), t.Picture.Desc)
			if err != nil {
				renderError(err)
				return
			}
			newPic = pic
		}
	}

	if !multi {
		info := t.Info
		info.Title = r.FormValue("title")
		info.Artist = r.FormValue("artist")
		info.Album = r.FormValue("album")
		info.AlbumArtist = r.FormValue("albumartist")
		info.Composer = r.FormValue("composer")
		info.Comment = r.FormValue("comment")
		t.ApplyInfo(info)
		t.Year, _ = strconv.Atoi(r.FormValue("year"))
		t.Number, _ = strconv.Atoi(r.FormValue("number"))
		t.Total, _ = strconv.Atoi(r.FormValue("total"))
		t.Disc, _ = strconv.Atoi(r.FormValue("disc"))
		t.Discs, _ = strconv.Atoi(r.FormValue("discs"))
		t.Picture = newPic
		t.Tags = strings.Split(strings.ToLower(r.FormValue("tags")), " ")
		t.Dirty = true
		t.LastMod = time.Now().UTC()
		if err := t.Save(ctx); err != nil {
			renderError(err)
			return
		}
		if err := u.UpdateLastMod(ctx); err != nil {
			renderError(err)
			return
		}

		http.Redirect(w, r, "/track/"+t.ID+"/edit", http.StatusSeeOther)
		return
	}

	info := tube.TrackInfo{
		Artist:      r.FormValue("artist"),
		Album:       r.FormValue("album"),
		AlbumArtist: r.FormValue("albumartist"),
		Composer:    r.FormValue("composer"),
		Comment:     r.FormValue("comment"),
	}
	// TODO: total
	year, _ := strconv.Atoi(r.FormValue("year"))
	disc, _ := strconv.Atoi(r.FormValue("disc"))
	discs, _ := strconv.Atoi(r.FormValue("discs"))

	update := func(u *dynamo.Update) {
		if info.Artist != "" {
			u.Set("Artist", strings.ToLower(info.Artist))
			u.Set("Info.Artist", info.Artist)
		}
		if info.Album != "" {
			u.Set("Album", strings.ToLower(info.Album))
			u.Set("Info.Album", info.Album)
		}
		if info.AlbumArtist != "" {
			u.Set("AlbumArtist", strings.ToLower(info.AlbumArtist))
			u.Set("Info.AlbumArtist", info.AlbumArtist)
		}
		if info.Composer != "" {
			u.Set("Composer", strings.ToLower(info.Composer))
			u.Set("Info.Composer", info.Composer)
		}
		if info.Comment != "" {
			u.Set("Comment", strings.ToLower(info.Comment))
			u.Set("Info.'Comment'", info.Comment)
		}
		if year != 0 {
			u.Set("Year", year)
		}
		if disc != 0 {
			u.Set("Disc", disc)
		}
		if discs != 0 {
			u.Set("Discs", discs)
		}
		if newPic != (tube.Picture{}) || picdel {
			u.Set("Picture", newPic)
		}
	}

	if err := tube.MassUpdateTracks(ctx, u.ID, ids, update); err != nil {
		renderError(err)
		return
	}
	if err := u.UpdateLastMod(ctx); err != nil {
		renderError(err)
		return
	}
	http.Redirect(w, r, "/track/"+id+"/edit", http.StatusSeeOther)
}

func getMultiTracks(ctx context.Context, u tube.User, ids []string) (t tube.Track, tracks tube.Tracks, err error) {
	multi := len(ids) > 1
	if !multi {
		t, err = tube.GetTrack(ctx, u.ID, ids[0])
		if err != nil {
			panic(err)
		}
		tracks = tube.Tracks{t}
	} else {
		tracks, err = tube.GetTracksBatch(ctx, u.ID, ids)
		if err != nil {
			panic(err)
		}
		t = tracks[0]
		for _, tx := range tracks {
			if tx.Artist != t.Artist {
				t.Artist = ""
				t.Info.Artist = ""
			}
			if tx.Album != t.Album {
				t.Album = ""
				t.Info.Album = ""
			}
			if tx.AlbumArtist != t.AlbumArtist {
				t.AlbumArtist = ""
				t.Info.AlbumArtist = ""
			}
			if tx.Composer != t.Composer {
				t.Composer = ""
				t.Info.Composer = ""
			}
			if tx.Picture != t.Picture {
				t.Picture = tube.Picture{}
			}
			if tx.Year != t.Year {
				t.Year = 0
			}
			if tx.Disc != t.Disc {
				t.Disc = 0
			}
			if tx.Discs != t.Discs {
				t.Discs = 0
			}
			if tx.Comment != t.Comment {
				t.Comment = ""
				t.Info.Comment = ""
			}
		}
	}
	return
}

// func groupByArtistAlbum(tracks []tube.Track) (artists []string, albums map[string][]string, m map[string]map[string][]tube.Track) {
// 	for _, t := range tracks {
// 		artist := t.AlbumArtist
// 		if artist == "" {
// 			artist = t.Artist
// 		}
// 		tx, seenArtist := m[artist][t.Album]
// 		album, ok := albums[artist]
// 		if ok {
// 			albums[artist] = append(albums[artist])
// 		}
// 	}
// }

type groupedTracks struct {
	artists              []string
	albums               []string
	albumArtists         []string
	eitherArtist         []string
	albumsByArtist       map[string][]string
	albumsByAlbumArtist  map[string][]string
	albumsByEitherArtist map[string][]string
	trackMap             map[string]map[string][]tube.Track
}

// TODO: this sucks ass
func groupTracks(tracks []tube.Track, mergeAlbumArtist bool) groupedTracks {
	group := &groupedTracks{
		albumsByArtist:       make(map[string][]string),
		albumsByAlbumArtist:  make(map[string][]string),
		albumsByEitherArtist: make(map[string][]string),
		trackMap:             make(map[string]map[string][]tube.Track),
	}
	albumsSeen := make(map[string]struct{})

	for _, t := range tracks {
		artist := t.Info.AlbumArtist
		if artist == "" {
			artist = t.Info.Artist
		}

		if _, ok := albumsSeen[t.Info.Album]; !ok {
			group.albums = append(group.albums, t.Info.Album)
			group.albumsByEitherArtist[artist] = append(group.albumsByEitherArtist[artist], t.Info.Album)
			albumsSeen[t.Info.Album] = struct{}{}
		}

		if t.Info.Artist != "" {
			albs, ok := group.albumsByArtist[t.Info.Artist]
			if !ok {
				group.artists = append(group.artists, t.Info.Artist)
			}
			albs = append(albs, t.Info.Album)
			group.albumsByArtist[t.Info.Artist] = albs
		}

		if t.Info.AlbumArtist != "" {
			albs, ok := group.albumsByAlbumArtist[t.Info.AlbumArtist]
			if !ok {
				group.albumArtists = append(group.albumArtists, t.Info.AlbumArtist)
			}
			albs = append(albs, t.Info.Album)
			group.albumsByAlbumArtist[t.Info.AlbumArtist] = albs
		}

		if _, ok := group.trackMap[artist]; !ok {
			group.trackMap[artist] = make(map[string][]tube.Track)
			group.eitherArtist = append(group.eitherArtist, artist)
		}
		group.trackMap[artist][t.Info.Album] = append(group.trackMap[artist][t.Info.Album], t)
	}
	if mergeAlbumArtist {
		for aa := range group.albumsByAlbumArtist {
			if _, ok := group.albumsByArtist[aa]; !ok {
				group.artists = append(group.artists, aa)
			}
		}
		group.albumArtists = group.artists
		// for a := range albumsByArtist {
		// 	if _, ok := albumsByAlbumArtist[a]; !ok {
		// 		albumArtists = append(albumArtists, a)
		// 	}
		// }
	}

	sort.Strings(group.artists)
	sort.Strings(group.albums)
	sort.Strings(group.albumArtists)
	sort.Strings(group.eitherArtist)
	for _, albums := range group.albumsByArtist {
		sort.Strings(albums)
	}
	for _, albums := range group.albumsByAlbumArtist {
		sort.Strings(albums)
	}
	for _, albums := range group.albumsByEitherArtist {
		sort.Strings(albums)
	}
	return *group
}

// js Date.getTime() -> go time.Time
func msec2time(msecs int64) time.Time {
	t := time.Unix(msecs/1000, (msecs%1000)*1000000)
	return t
}
