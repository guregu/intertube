package web

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/guregu/intertube/tube"
)

type subsonicFolder struct {
	XMLName  xml.Name `json:"-"`
	ID       string   `xml:"id,attr" json:"id"`
	Parent   string   `xml:"parent,attr" json:"parent"`
	Title    string   `xml:"title,attr" json:"title"`
	Artist   string   `xml:"artist,attr" json:"artist"`
	IsDir    bool     `xml:"isDir,attr" json:"isDir"`
	CoverArt string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
}

func newSubsonicFolder(album *albumInfo) subsonicFolder {
	dir := subsonicFolder{
		XMLName: xml.Name{Local: "album"},
		ID:      album.ssid.String(),
		Parent:  album.tracks[0].ArtistSSID().String(),
		Title:   album.name,
		Artist:  album.artist,
		IsDir:   true,
	}
	if album.picture != "" {
		// TODO:
		dir.CoverArt = tube.NewSSID(tube.SSIDTrack, album.picture).String()
	}
	return dir
}

type subsonicIndex struct {
	XMLName xml.Name         `xml:"index" json:"-"`
	Name    string           `xml:"name,attr" json:"name"`
	Artists []subsonicArtist `json:"artist,omitempty"`
}

func newSubsonicIndexes(grp groupedTracks) []subsonicIndex {
	idxMap := make(map[rune]*subsonicIndex)

	for _, a := range grp.artists {
		albs := grp.albumsByEitherArtist[a]
		if len(albs) == 0 || len(grp.trackMap[a][albs[0]]) == 0 {
			continue
		}
		first := grp.trackMap[a][albs[0]][0]

		r, _ := utf8.DecodeRuneInString(strings.ToUpper(a))
		idx, ok := idxMap[r]
		if !ok {
			idx = &subsonicIndex{
				Name: string(r),
			}
			idxMap[r] = idx
		}

		ai := subsonicArtist{
			ID:   first.ArtistSSID(),
			Name: a,
			// Name: first.,
			CoverArt:   first.TrackSSID().String(),
			AlbumCount: len(albs),
		}
		if ai.AlbumCount == 0 {
			// TODO fix
			continue
		}
		idx.Artists = append(idx.Artists, ai)
	}

	var indexes []subsonicIndex
	for _, idx := range idxMap {
		indexes = append(indexes, *idx)
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].Name < indexes[j].Name
	})
	return indexes
}

func subsonicGetMusicFolders(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// TODO: this
	type subsonicMusicFolder struct {
		XMLName xml.Name `xml:"musicFolder" json:"-"`
		ID      int      `xml:"id,attr" json:"id"`
		Name    string   `xml:"name,attr" json:"name"`
	}
	type subsonicMusicFolders struct {
		subsonicResponse
		Folders struct {
			Content []subsonicMusicFolder `json:"musicFolder,omitempty"`
		} `xml:"musicFolders" json:"musicFolders"`
	}

	folders := []subsonicMusicFolder{
		{
			ID:   1,
			Name: "Music",
		},
	}
	resp := subsonicMusicFolders{
		subsonicResponse: subOK(),
	}
	resp.Folders.Content = folders

	writeSubsonic(ctx, w, r, resp)
}

func subsonicGetGenres(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	grp, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}

	type subsonicGenre struct {
		XMLName    xml.Name `xml:"genre" json:"-"`
		SongCount  int      `xml:"songCount,attr" json:"songCount"`
		AlbumCount int      `xml:"albumCount,attr" json:"albumCount"`
		Genre      string   `xml:",chardata" json:"value"`
	}

	resp := struct {
		subsonicResponse
		Genres struct {
			List []subsonicGenre `json:"genre,omitempty"`
		} `xml:"genres" json:"genres"`
	}{
		subsonicResponse: subOK(),
	}

	for _, g := range grp.Genres() {
		resp.Genres.List = append(resp.Genres.List, subsonicGenre{
			Genre:      g.name,
			SongCount:  len(g.tracks),
			AlbumCount: len(g.albums),
		})
	}

	writeSubsonic(ctx, w, r, resp)
	return
}

func subsonicSearch2(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	q := r.FormValue("query")
	mfid := r.FormValue("musicFolderId")
	artistSize, _ := strconv.Atoi(r.FormValue("artistCount"))
	artistOffset, _ := strconv.Atoi(r.FormValue("artistOffset"))
	albumSize, _ := strconv.Atoi(r.FormValue("albumCount"))
	albumOffset, _ := strconv.Atoi(r.FormValue("albumOffset"))
	songSize, _ := strconv.Atoi(r.FormValue("songCount"))
	songOffset, _ := strconv.Atoi(r.FormValue("songOffset"))

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}

	resp := struct {
		subsonicResponse
		Results struct {
			Artists []subsonicArtist `json:"artist,omitempty"`
			Albums  []subsonicFolder `json:"album,omitempty"`
			Songs   []subsonicSong   `json:"song,omitempty"`
		} `xml:"searchResult2" json:"searchResult2"`
	}{
		subsonicResponse: subOK(),
	}
	_ = mfid

	artists := lib.Artists(organize{
		query:  q,
		size:   artistSize,
		offset: artistOffset,
		// ssid: tube.NewSSID(tube.SSIDFolder, mfid),
	})
	for _, a := range artists {
		resp.Results.Artists = append(resp.Results.Artists, newSubsonicArtist(a))
	}
	albums := lib.Albums(organize{
		query:  q,
		size:   albumSize,
		offset: albumOffset,
	})
	for _, a := range albums {
		resp.Results.Albums = append(resp.Results.Albums, newSubsonicFolder(a))
	}
	songs := lib.Tracks(organize{
		query:  q,
		size:   songSize,
		offset: songOffset,
	})
	for _, t := range songs {
		resp.Results.Songs = append(resp.Results.Songs, newSubsonicSong(t, "song"))
	}

	writeSubsonic(ctx, w, r, resp)
}

func subsonicSearch3(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	q := r.FormValue("query")
	mfid := r.FormValue("musicFolderId")
	artistSize, _ := strconv.Atoi(r.FormValue("artistCount"))
	artistOffset, _ := strconv.Atoi(r.FormValue("artistOffset"))
	albumSize, _ := strconv.Atoi(r.FormValue("albumCount"))
	albumOffset, _ := strconv.Atoi(r.FormValue("albumOffset"))
	songSize, _ := strconv.Atoi(r.FormValue("songCount"))
	songOffset, _ := strconv.Atoi(r.FormValue("songOffset"))

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}

	resp := struct {
		subsonicResponse
		Results struct {
			Artists []subsonicArtist `json:"artist,omitempty"`
			Albums  []subsonicAlbum  `json:"album,omitempty"`
			Songs   []subsonicSong   `json:"song,omitempty"`
		} `xml:"searchResult3" json:"searchResult3"`
	}{
		subsonicResponse: subOK(),
	}
	_ = mfid

	artists := lib.Artists(organize{
		query:  q,
		size:   artistSize,
		offset: artistOffset,
		// ssid: tube.NewSSID(tube.SSIDFolder, mfid),
	})
	for _, a := range artists {
		resp.Results.Artists = append(resp.Results.Artists, newSubsonicArtist(a))
	}
	albums := lib.Albums(organize{
		query:  q,
		size:   albumSize,
		offset: albumOffset,
	})
	for _, a := range albums {
		resp.Results.Albums = append(resp.Results.Albums, newSubsonicAlbum(a.tracks, false))
	}
	songs := lib.Tracks(organize{
		query:  q,
		size:   songSize,
		offset: songOffset,
	})
	for _, t := range songs {
		resp.Results.Songs = append(resp.Results.Songs, newSubsonicSong(t, "song"))
	}

	writeSubsonic(ctx, w, r, resp)
}

func subsonicGetSongsByGenre(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	size, _ := strconv.Atoi(r.FormValue("size"))
	if size == 0 {
		size = 10
	}
	if size > 500 {
		size = 500
	}
	offset, _ := strconv.Atoi(r.FormValue("offset"))
	genre := r.FormValue("genre")
	match := tube.ParseSSID(r.FormValue("musicFolderId"))

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}
	got := lib.Tracks(organize{
		by:     "",
		genre:  genre,
		ssid:   match,
		offset: offset,
		size:   size,
	})

	var result []subsonicSong
	for _, t := range got {
		result = append(result, newSubsonicSong(t, "song"))
	}

	type tracksResponse struct {
		subsonicResponse
		Songs struct {
			List []subsonicSong `json:"song,omitempty"`
		} `xml:"songsByGenre" json:"songsByGenre"`
	}

	resp := tracksResponse{
		subsonicResponse: subOK(),
	}
	resp.Songs.List = result

	writeSubsonic(ctx, w, r, resp)
}

// TODO: this is broken in json mode (doesn't turn into arrays)
func subsonicGetStarred(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	match := tube.ParseSSID(r.FormValue("musicFolderId"))

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}

	resp := struct {
		subsonicResponse
		Starred struct {
			XMLName xml.Name
			List    []interface{}
		}
	}{
		subsonicResponse: subOK(),
	}

	if strings.Contains(r.URL.Path, "Starred2") {
		resp.Starred.XMLName = xml.Name{Local: "starred2"}
	} else {
		resp.Starred.XMLName = xml.Name{Local: "starred"}
	}

	artists := lib.Artists(organize{
		ssid:    match,
		starred: true,
	})
	for _, a := range artists {
		artist := newSubsonicArtist(a)
		artist.Starred = a.starred.Format(subsonicTimeLayout)
		resp.Starred.List = append(resp.Starred.List, artist)
	}

	albums := lib.Albums(organize{
		ssid:    match,
		starred: true,
	})
	for _, a := range albums {
		album := newSubsonicAlbum(a.tracks, false)
		album.Starred = a.starred.Format(subsonicTimeLayout)
		resp.Starred.List = append(resp.Starred.List, album)
	}

	tracks := lib.Tracks(organize{
		ssid:    match,
		starred: true,
	})
	fmt.Println("STARS", tracks)
	fmt.Println("LIB", lib.stars)
	for _, t := range tracks {
		resp.Starred.List = append(resp.Starred.List, newSubsonicSong(t, "song"))
	}

	if formatFrom(ctx) == "json" {
		w.Header().Set("Content-Type", "application/json")

		type starList struct {
			Artists []subsonicArtist `json:"artist,omitempty"`
			Albums  []subsonicAlbum  `json:"album,omitempty"`
			Tracks  []subsonicSong   `json:"song,omitempty"`
		}

		jresp := struct {
			subsonicResponse
			Starred  *starList `json:"starred,omitempty"`
			Starred2 *starList `json:"starred2,omitempty"`
		}{
			subsonicResponse: subOK(),
		}

		list := &starList{}
		if resp.Starred.XMLName.Local == "starred2" {
			jresp.Starred2 = list
		} else {
			jresp.Starred = list
		}

		for _, entry := range resp.Starred.List {
			switch x := entry.(type) {
			case subsonicArtist:
				list.Artists = append(list.Artists, x)
			case subsonicAlbum:
				list.Albums = append(list.Albums, x)
			case subsonicSong:
				list.Tracks = append(list.Tracks, x)
			}
		}

		writeSubsonic(ctx, w, r, jresp)
		return
	}

	writeSubsonic(ctx, w, r, resp)
}
