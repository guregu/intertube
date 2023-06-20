package web

import (
	"context"
	"encoding/xml"
	"net/http"
	"strings"

	"github.com/guregu/intertube/tube"
)

type subsonicArtist struct {
	XMLName xml.Name `xml:"artist" json:"-"`
	// <artist id="5449" name="A-Ha" coverArt="ar-5449" albumCount="4"/>
	/*
		<xs:attribute name="id" type="xs:string" use="required"/>
		<xs:attribute name="name" type="xs:string" use="required"/>
		<xs:attribute name="coverArt" type="xs:string" use="optional"/>
		<xs:attribute name="artistImageUrl" type="xs:string" use="optional"/>
		<!--  Added in 1.16.1  -->
		<xs:attribute name="albumCount" type="xs:int" use="required"/>
		<xs:attribute name="starred" type="xs:dateTime" use="optional"/>
	*/
	ID       tube.SSID `xml:"id,attr" json:"id"`
	Name     string    `xml:"name,attr" json:"name"`
	CoverArt string    `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	// TODO: artistImageUrl
	AlbumCount int    `xml:"albumCount,attr" json:"albumCount"`
	Starred    string `xml:"starred,attr,omitempty" json:"starred,omitempty"`

	Albums []subsonicAlbum `xml:",omitempty" json:"album,omitempty"`
}

func newSubsonicArtist(a *artistInfo) subsonicArtist {
	artist := subsonicArtist{
		ID:         a.ssid,
		Name:       a.name,
		AlbumCount: len(a.albums),
	}
	for _, t := range a.tracks {
		if t.Picture.ID != "" {
			// TODO: artist pic?
			artist.CoverArt = t.TrackSSID().String()
			break
		}
	}
	return artist
}

func subsonicGetArtists(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	tracks, err := u.GetTracks(ctx)
	if err != nil {
		panic(err)
	}

	grp := groupTracks(tracks, true)

	type artistsResp struct {
		subsonicResponse
		Indexes struct {
			IgnoredArticles string          `xml:"ignoredArticles,attr" json:"ignoredArticles"`
			List            []subsonicIndex `json:"index,omitempty"`
		} `xml:"artists" json:"artists"`
	}

	resp := artistsResp{
		subsonicResponse: subOK(),
	}
	resp.Indexes.IgnoredArticles = subsonicIgnoreArticles
	resp.Indexes.List = newSubsonicIndexes(grp)

	writeSubsonic(ctx, w, r, resp)
}

func subsonicGetIndexes(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	tracks, err := u.GetTracks(ctx)
	if err != nil {
		panic(err)
	}

	grp := groupTracks(tracks, true)

	type artistsResp struct {
		subsonicResponse
		// TODO: ignoredArticles=""
		Indexes struct {
			IgnoredArticles string          `xml:"ignoredArticles,attr" json:"ignoredArticles"`
			List            []subsonicIndex `json:"index,omitempty"`
		} `xml:"indexes" json:"indexes"`
	}

	resp := artistsResp{
		subsonicResponse: subOK(),
	}
	resp.Indexes.IgnoredArticles = subsonicIgnoreArticles
	resp.Indexes.List = newSubsonicIndexes(grp)

	writeSubsonic(ctx, w, r, resp)
}

func subsonicGetArtist(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	rawid := r.FormValue("id")
	id := tube.ParseSSID(rawid).ID

	tracks, err := u.GetTracks(ctx)
	if err != nil {
		panic(err)
	}
	sortTracks(tracks)

	// TODO: make efficient
	allAlbums := byAlbum(tracks)

	var albums []subsonicAlbum
	for _, a := range allAlbums {
		if a[0].AnyArtist() == id /*|| a[0].Info.AlbumArtist == id || a[0].Info.Composer == id */ {
			x := newSubsonicAlbum(a, false)
			albums = append(albums, x)
		}
	}

	artist := subsonicArtist{
		ID:   albums[0].ArtistID,
		Name: albums[0].Artist,
		// Name: first.,
		// TODO: CoverArt: first.
		AlbumCount: len(albums),
		Albums:     albums,
	}

	resp := struct {
		subsonicResponse
		Artist subsonicArtist `xml:"artist" json:"artist"`
	}{
		subsonicResponse: subOK(),
		Artist:           artist,
	}

	writeSubsonic(ctx, w, r, resp)
}

func subsonicGetArtistInfo(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	type info struct {
		Biography      string `xml:"biography,omitempty" json:"biography,omitempty"`
		MusicBrainzId  string `xml:"musicBrainzId,omitempty" json:"musicBrainzId,omitempty"`
		LastFmUrl      string `xml:"lastFmUrl,omitempty" json:"lastFmUrl,omitempty"`
		SmallImageUrl  string `xml:"smallImageUrl,omitempty" json:"smallImageUrl,omitempty"`
		MediumImageUrl string `xml:"mediumImageUrl,omitempty" json:"mediumImageUrl,omitempty"`
		LargeImageUrl  string `xml:"largeImageUrl,omitempty" json:"largeImageUrl,omitempty"`
		// <similarArtist id="22" name="...."/>
	}
	resp := struct {
		subsonicResponse
		Info  *info `xml:"artistInfo,omitempty" json:"artistInfo,omitempty"`
		Info2 *info `xml:"artistInfo2,omitempty" json:"artistInfo2,omitempty"`
	}{
		subsonicResponse: subOK(),
	}

	ai := &info{}
	if strings.Contains(r.URL.Path, "Info2") {
		resp.Info2 = ai
	} else {
		resp.Info = ai
	}
	// ai.Biography = "TODO: not implemented yet, sorry~"

	writeSubsonic(ctx, w, r, resp)
}
