package web

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"

	"github.com/guregu/intertube/tube"
)

type subsonicAlbum struct {
	XMLName   xml.Name  `xml:"album" json:"-"`
	ID        tube.SSID `xml:"id,attr" json:"id"`
	Name      string    `xml:"name,attr" json:"name"`
	Artist    string    `xml:"artist,attr" json:"artist"`
	ArtistID  tube.SSID `xml:"artistId,attr" json:"artistId"`
	CoverArt  string    `xml:"coverArt,omitempty,attr" json:"coverArt,omitempty"`
	SongCount int       `xml:"songCount,attr" json:"songCount"`
	Duration  int       `xml:"duration,attr" json:"duration"`
	Created   string    `xml:"created,attr" json:"created"`
	Year      int       `xml:"year,attr,omitempty" json:"year,omitempty"`
	Starred   string    `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	// <xs:attribute name="playCount" type="xs:long" use="optional"/>
	// <xs:attribute name="genre" type="xs:string" use="optional"/>

	Songs []subsonicSong `xml:",omitempty" json:"song,omitempty"`
}

func newSubsonicAlbum(tt []tube.Track, includeTracks bool) subsonicAlbum {
	first := tt[0]
	album := subsonicAlbum{
		ID:        first.AlbumSSID(),
		Name:      first.Info.Album,
		Artist:    first.Info.Artist, // TODO
		ArtistID:  first.ArtistSSID(),
		SongCount: len(tt),
		Created:   first.Date.Format(subsonicTimeLayout),
		Year:      first.Year,
	}
	if first.Picture.ID != "" {
		album.CoverArt = first.TrackSSID().String()
	}
	var dur int
	for _, t := range tt {
		dur += t.Duration
		if includeTracks {
			song := newSubsonicSong(t, "song")
			if t.Picture.ID == first.Picture.ID {
				song.CoverArt = first.TrackSSID().String()
			}
			album.Songs = append(album.Songs, song)
		}
	}
	album.Duration = dur
	return album
}

func subsonicGetAlbumList2(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	filter := subsonicFilter(r)

	type albumlist2 struct {
		subsonicResponse
		List struct {
			Albums []subsonicAlbum `json:"album,omitempty"`
		} `xml:"albumList2" json:"albumList2"`
	}

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}
	split := lib.Albums(filter)

	albums := make([]subsonicAlbum, 0, len(split))
	for _, a := range split {
		if len(a.tracks) == 0 {
			continue
		}
		a := newSubsonicAlbum(a.tracks, false)
		albums = append(albums, a)
	}

	resp := albumlist2{
		subsonicResponse: subOK(),
	}
	resp.List.Albums = albums

	writeSubsonic(ctx, w, r, resp)
}

func subsonicGetAlbumList1(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	filter := subsonicFilter(r)

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}
	allAlbums := lib.Albums(filter)

	fmt.Printf("FILTER: %#v\n", filter)

	resp := struct {
		subsonicResponse
		AlbumList struct {
			Albums []subsonicFolder `json:"album,omitempty"`
		} `xml:"albumList" json:"albumList"`
	}{
		subsonicResponse: subOK(),
	}

	for _, a := range allAlbums {
		dir := newSubsonicFolder(a)
		resp.AlbumList.Albums = append(resp.AlbumList.Albums, dir)
	}

	writeSubsonic(ctx, w, r, resp)
	return
}

func subsonicGetAlbum(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	rawid := r.FormValue("id")
	ssid := tube.ParseSSID(rawid)

	lib, err := getLibrary(ctx, u)
	if err != nil {
		panic(err)
	}
	album, ok := lib.albums[ssid.String()]
	if !ok {
		writeSubsonic(ctx, w, r, subErr(70, "The requested data was not found."))
		return
	}

	type subsonicAlbumResp struct {
		subsonicResponse
		Album subsonicAlbum `xml:"album" json:"album"`
	}
	resp := subsonicAlbumResp{
		subsonicResponse: subOK(),
		Album:            newSubsonicAlbum(album.tracks, true),
	}
	writeSubsonic(ctx, w, r, resp)
}

func subsonicFilter(r *http.Request) organize {
	sortby := r.FormValue("type")
	size, _ := strconv.Atoi(r.FormValue("size"))
	if size == 0 {
		size = subsonicMaxSize
	}
	offset, _ := strconv.Atoi(r.FormValue("offset"))
	fromYear, _ := strconv.Atoi(r.FormValue("fromYear"))
	toYear, _ := strconv.Atoi(r.FormValue("toYear"))
	genre := r.FormValue("genre")
	mfid := r.FormValue("musicFolderId")
	if mfid == "1" {
		// "Music" folder
		// TODO: hmm
		mfid = ""
	}
	ssid := tube.ParseSSID(mfid)
	filter := organize{
		by:       sortby,
		size:     size,
		offset:   offset,
		fromYear: fromYear,
		toYear:   toYear,
		genre:    genre,
		ssid:     ssid,
	}
	return filter
}
