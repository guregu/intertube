package web

import (
	"context"
	"net/http"
	"sort"
	"strconv"

	"github.com/guregu/kami"

	"github.com/guregu/intertube/tube"
)

const tracksPerPage = 1000

func showMusicHead(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// u, _ := userFrom(ctx)

	// if !u.LastMod.IsZero() {
	// 	w.Header().Set("Cache-Control", "no-cache, max-age=0")
	// 	w.Header().Set("Last-Modified", u.LastMod.Format(http.TimeFormat))
	// }
}

func showMusic(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	lastMod := lastestMod(u.LastMod)

	start := r.URL.Query().Get("next")
	// if ims := r.Header.Get("If-Modified-Since"); ims != "" && start != "" {
	// 	since, err := time.Parse(http.TimeFormat, ims)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	trunc := lastMod.Truncate(time.Second)
	// 	if !trunc.After(since) {
	// 		w.WriteHeader(http.StatusNotModified)
	// 		return
	// 	}
	// }

	// w.Header().Set("Cache-Control", "no-cache, max-age=0")
	// if !u.LastMod.IsZero() {
	// 	w.Header().Set("Last-Modified", u.LastMod.Format(http.TimeFormat))
	// }

	kind := kami.Param(ctx, "kind")
	if kind == "" {
		kind = "all"
	}
	view := "music_" + kind // playlist template name
	inline := r.URL.Query().Get("render") == "inline"

	lib, err := getLibrary(ctx, u)
	if err != nil && err != tube.ErrNotFound {
		panic(err)
	}
	offset, _ := strconv.Atoi(start)
	filter := organize{
		size:   tracksPerPage,
		offset: offset,
	}
	tracks := lib.Tracks(filter)
	var nextID int
	if len(tracks) == filter.size {
		// last := tracks[len(tracks)-1]
		nextID = filter.offset + filter.size
		w.Header().Set("Tube-Next", strconv.Itoa(nextID))
	}
	w.Header().Set("Tube-Count", strconv.Itoa(len(tracks)))

	sortParam := r.URL.Query().Get("sort")
	reverse := false
	if len(sortParam) > 0 {
		if sortParam[0] == '-' {
			sortParam = sortParam[1:]
			reverse = true
		}
	} else {
		sortParam = "artist"
	}

	// sorted := trackSortings(tracks)
	// if reverse {
	// 	tracks = sorted[sortParam].Desc
	// } else {
	// 	tracks = sorted[sortParam].Asc
	// }

	// TODO: unfuck
	// var sortedIDs = map[string]struct {
	// 	Asc  []string
	// 	Desc []string
	// }{}
	// for k, v := range sorted {
	// 	sortedIDs[k] = struct {
	// 		Asc  []string
	// 		Desc []string
	// 	}{
	// 		Asc:  v.Asc.IDs(),
	// 		Desc: v.Desc.IDs(),
	// 	}
	// }

	data := struct {
		User     tube.User
		Tracks   []tube.Track
		Albums   [][]tube.Track
		Sort     string
		Reverse  bool
		View     string
		ViewTmpl string
		Next     string
		LastMod  int64
		// Query    string
	}{
		User:     u,
		Tracks:   tracks,
		Albums:   byAlbum(tracks),
		Sort:     sortParam,
		Reverse:  reverse,
		View:     view,
		ViewTmpl: view + ".gohtml",
		LastMod:  lastMod.UnixNano(),
		// Query:    query,
	}
	if nextID > 0 {
		data.Next = strconv.Itoa(nextID)
	}

	if inline {
		renderTemplate(ctx, w, view, data, http.StatusOK)
		return
	}

	renderTemplate(ctx, w, "music", data, http.StatusOK)
}

// TODO: group by picture ID
func byAlbum(tracks []tube.Track) [][]tube.Track {
	var albums [][]tube.Track
	var album []tube.Track
	// var prev tube.Track
	for i, t := range tracks {
		if i != 0 && (!tracks[i-1].AlbumEqual(t)) {
			albums = append(albums, album)
			album = nil
		}
		album = append(album, t)
	}
	if len(album) > 0 {
		albums = append(albums, album)
	}
outer:
	for i, a := range albums {
		if len(a) == 0 {
			continue
		}
		if a[0].Picture.ID != "" {
			continue
		}
		for ii := range a[1:] {
			if a[ii].Picture.ID != "" {
				t := albums[i][0]
				t.Picture = a[ii].Picture
				albums[i][0] = t
				break outer
			}
		}
	}
	return albums
}

func sortTracks(tracks []tube.Track) {
	sort.Slice(tracks, func(i, j int) bool {
		a := tracks[i]
		b := tracks[j]
		if !a.ArtistEqual(b) {
			if a.AlbumArtist < b.AlbumArtist {
				return true
			}
			if a.AlbumArtist > b.AlbumArtist {
				return false
			}
			if a.Artist < b.Artist {
				return true
			}
			if a.Artist > b.Artist {
				return false
			}
		}
		if !a.AlbumEqual(b) {
			if a.Album < b.Album {
				return true
			}
			if a.Album > b.Album {
				return false
			}
		}
		if a.Disc != 0 && b.Disc != 0 {
			if a.Disc < b.Disc {
				return true
			}
			if a.Disc > b.Disc {
				return false
			}
		}
		if a.Number < b.Number {
			return true
		}
		if a.Number > b.Number {
			return false
		}
		if a.Title < b.Title {
			return true
		}
		if a.Title > b.Title {
			return false
		}
		if a.Year < b.Year {
			return true
		}
		if a.Year > b.Year {
			return false
		}
		if a.Filename < b.Filename {
			return true
		}
		// if a.ID < b.ID {
		// 	return true
		// }
		if !a.Date.IsZero() && !b.Date.IsZero() {
			return a.Date.Before(b.Date)
		}
		return a.ID < b.ID
	})
}

/*
type sortedTracks struct {
	Asc  tube.Tracks
	Desc tube.Tracks
}

func trackSortings(tracks tube.Tracks) map[string]sortedTracks {
	sorted := make(map[string]sortedTracks)

	commonCond := []interface{}{
		func(a, b tube.Track) int { return a.Disc - b.Disc },
		func(a, b tube.Track) int { return a.Number - b.Number },
		func(a, b tube.Track) int { return strings.Compare(a.Title, b.Title) },
		func(a, b tube.Track) int { return strings.Compare(a.Filename, b.Filename) },
		func(a, b tube.Track) int { return strings.Compare(a.ID, b.ID) },
	}
	{
		asc := make([]tube.Track, len(tracks))
		copy(asc, tracks)
		desc := make([]tube.Track, len(tracks))
		copy(desc, tracks)
		order.By(append([]interface{}{
			func(a, b tube.Track) int { return strings.Compare(a.AnyArtist(), b.AnyArtist()) },
			func(a, b tube.Track) int { return strings.Compare(a.Info.Album, b.Info.Album) },
		}, commonCond...)...).Sort(asc)
		order.By(append([]interface{}{
			func(a, b tube.Track) int { return invert(strings.Compare(a.AnyArtist(), b.AnyArtist())) },
			func(a, b tube.Track) int { return strings.Compare(a.Info.Album, b.Info.Album) },
		}, commonCond...)...).Sort(desc)
		sorted["artist"] = sortedTracks{
			Asc:  asc,
			Desc: desc,
		}
	}
	{
		asc := make([]tube.Track, len(tracks))
		copy(asc, tracks)
		desc := make([]tube.Track, len(tracks))
		copy(desc, tracks)
		order.By(append([]interface{}{
			func(a, b tube.Track) int { return a.LastPlayed.Compare(b.LastPlayed) },
			func(a, b tube.Track) int { return strings.Compare(a.AnyArtist(), b.AnyArtist()) },
			func(a, b tube.Track) int { return strings.Compare(a.Info.Album, b.Info.Album) },
		}, commonCond...)...).Sort(asc)
		order.By(append([]interface{}{
			func(a, b tube.Track) int { return invert(a.LastPlayed.Compare(b.LastPlayed)) },
			func(a, b tube.Track) int { return strings.Compare(a.AnyArtist(), b.AnyArtist()) },
			func(a, b tube.Track) int { return strings.Compare(a.Info.Album, b.Info.Album) },
		}, commonCond...)...).Sort(desc)
		sorted["played"] = sortedTracks{
			Asc:  asc,
			Desc: desc,
		}
	}
	{
		asc := make([]tube.Track, len(tracks))
		copy(asc, tracks)
		desc := make([]tube.Track, len(tracks))
		copy(desc, tracks)
		order.By(
			func(a, b tube.Track) int { return strings.Compare(a.Title, b.Title) },
			func(a, b tube.Track) int { return strings.Compare(a.AnyArtist(), b.AnyArtist()) },
			func(a, b tube.Track) int { return strings.Compare(a.Filename, b.Filename) },
			func(a, b tube.Track) int { return strings.Compare(a.Info.Album, b.Info.Album) },
			func(a, b tube.Track) int { return strings.Compare(a.ID, b.ID) },
		).Sort(asc)
		order.By(
			func(a, b tube.Track) int { return invert(strings.Compare(a.Title, b.Title)) },
			func(a, b tube.Track) int { return strings.Compare(a.AnyArtist(), b.AnyArtist()) },
			func(a, b tube.Track) int { return strings.Compare(a.Filename, b.Filename) },
			func(a, b tube.Track) int { return strings.Compare(a.Info.Album, b.Info.Album) },
			func(a, b tube.Track) int { return strings.Compare(a.ID, b.ID) },
		).Sort(desc)
		sorted["title"] = sortedTracks{
			Asc:  asc,
			Desc: desc,
		}
	}
	{
		asc := make([]tube.Track, len(tracks))
		copy(asc, tracks)
		desc := make([]tube.Track, len(tracks))
		copy(desc, tracks)
		order.By(
			func(a, b tube.Track) int { return a.Date.Compare(b.Date) },
			func(a, b tube.Track) int { return strings.Compare(a.ID, b.ID) },
		).Sort(asc)
		order.By(
			func(a, b tube.Track) int { return invert(a.Date.Compare(b.Date)) },
			func(a, b tube.Track) int { return strings.Compare(a.ID, b.ID) },
		).Sort(desc)
		sorted["added"] = sortedTracks{
			Asc:  asc,
			Desc: desc,
		}
	}
	{
		asc := make([]tube.Track, len(tracks))
		copy(asc, tracks)
		desc := make([]tube.Track, len(tracks))
		copy(desc, tracks)
		order.By(append([]interface{}{
			func(a, b tube.Track) int { return strings.Compare(a.Info.Album, b.Info.Album) },
			func(a, b tube.Track) int { return strings.Compare(a.AnyArtist(), b.AnyArtist()) },
		}, commonCond...)...).Sort(asc)
		order.By(append([]interface{}{
			func(a, b tube.Track) int { return invert(strings.Compare(a.Info.Album, b.Info.Album)) },
			func(a, b tube.Track) int { return strings.Compare(a.AnyArtist(), b.AnyArtist()) },
		}, commonCond...)...).Sort(desc)
		sorted["album"] = sortedTracks{
			Asc:  asc,
			Desc: desc,
		}
	}

	return sorted
}
*/

func invert(n int) int {
	return -n
}
