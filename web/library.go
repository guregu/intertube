package web

import (
	"context"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	// "github.com/davecgh/go-spew/spew"

	"github.com/antonmedv/expr"
	"github.com/posener/order"
	"golang.org/x/sync/errgroup"

	"github.com/guregu/intertube/tube"
)

type Library struct {
	tracks  []tube.Track
	stars   map[tube.SSID]tube.Star
	artists map[string]*artistInfo
	albums  map[string]*albumInfo
	genres  map[string]*genreInfo
	idmap   map[string]tube.Track // TODO: pointer?
}

type genreInfo struct {
	name   string
	tracks []tube.Track
	albums map[string]struct{}
}

type artistInfo struct {
	id      string
	ssid    tube.SSID
	name    string
	tracks  []tube.Track
	albums  map[string]struct{} // AlbumSSID
	starred time.Time
}

func (a *artistInfo) matches(ssid tube.SSID) bool {
	for _, t := range a.tracks {
		if t.MatchesSSID(ssid) {
			return true
		}
	}
	return false
}

type albumInfo struct {
	id       string
	ssid     tube.SSID
	name     string
	artist   string
	artists  map[string]struct{}
	genres   map[string]int
	tracks   []tube.Track
	date     time.Time
	plays    int
	played   time.Time
	lowYear  int
	highYear int
	picture  string
	starred  time.Time
}

func (a *albumInfo) add(t tube.Track) {
	a.tracks = append(a.tracks, t)
	a.plays += t.Plays

	if a.id == "" {
		a.ssid = t.AlbumSSID()
		a.id = a.ssid.String()
	}
	if a.name == "" {
		a.name = t.Info.Album
	}
	if a.artist == "" {
		a.artist = t.Info.AnyArtist()
	}

	if t.Info.Artist != "" {
		a.artists[t.Info.Artist] = struct{}{}
	}
	if t.Info.AlbumArtist != "" {
		a.artists[t.Info.AlbumArtist] = struct{}{}
	}
	if t.Genre != "" {
		a.genres[t.Genre] = a.genres[t.Genre] + 1
	}

	if t.Year != 0 {
		if a.lowYear == 0 || t.Year < a.lowYear {
			a.lowYear = t.Year
		}
		if a.highYear == 0 || t.Year > a.highYear {
			a.highYear = t.Year
		}
	}

	if t.Date.After(a.date) {
		a.date = t.Date
	}
	if t.LastPlayed.After(a.played) {
		a.played = t.LastPlayed
	}

	if a.picture == "" {
		a.picture = t.ID // TODO: hmm
	}
}

func (a *albumInfo) mainGenre() string {
	var most string
	for g, n := range a.genres {
		if n > a.genres[most] {
			most = g
		}
	}
	return most
}

func (a *albumInfo) cmpgenre(b *albumInfo) int {
	ag, bg := a.mainGenre(), b.mainGenre()
	if ag == "" && bg != "" {
		return 1
	}
	if ag != "" && bg == "" {
		return -1
	}
	return strings.Compare(ag, bg)
}

func (a *albumInfo) matches(ssid tube.SSID) bool {
	for _, t := range a.tracks {
		if t.MatchesSSID(ssid) {
			return true
		}
	}
	return false
}

func NewLibrary(tracks []tube.Track, stars map[tube.SSID]tube.Star) *Library {
	sortTracks(tracks)
	lib := &Library{
		tracks:  tracks,
		stars:   stars,
		artists: make(map[string]*artistInfo),
		albums:  make(map[string]*albumInfo),
		genres:  make(map[string]*genreInfo),
		idmap:   make(map[string]tube.Track),
	}

	for i, t := range tracks {
		// track
		t.Starred = lib.starred(t.TrackSSID())
		tracks[i] = t
		lib.idmap[t.ID] = t

		// artist
		artist, ok := lib.artists[t.ArtistSSID().String()]
		if !ok {
			artist = &artistInfo{
				// TODO: reconcile if different between tracks
				id:      t.ArtistSSID().String(),
				ssid:    t.ArtistSSID(),
				name:    t.Info.AnyArtist(),
				albums:  make(map[string]struct{}),
				starred: lib.starred(t.ArtistSSID()),
			}
			lib.artists[t.ArtistSSID().String()] = artist
		}
		artist.tracks = append(artist.tracks, t)
		artist.albums[t.AlbumSSID().String()] = struct{}{}

		// album
		album, ok := lib.albums[t.AlbumSSID().String()]
		if !ok {
			album = &albumInfo{
				artists: make(map[string]struct{}),
				genres:  make(map[string]int),
				starred: lib.starred(t.AlbumSSID()),
			}
			lib.albums[t.AlbumSSID().String()] = album
		}
		album.add(t)

		// genre
		genre := t.Genre
		stats, ok := lib.genres[t.Genre]
		if !ok {
			stats = &genreInfo{
				name:   genre,
				albums: make(map[string]struct{}),
			}
			lib.genres[genre] = stats
		}
		stats.tracks = append(stats.tracks, t)
		stats.albums[t.AlbumSSID().String()] = struct{}{}
	}
	return lib
}

func (lib *Library) Genres() []*genreInfo {
	genres := make([]*genreInfo, 0, len(lib.genres))
	for _, g := range lib.genres {
		if g.name == "" {
			continue
		}
		genres = append(genres, g)
	}
	sort.Slice(genres, func(i, j int) bool {
		return genres[i].name < genres[j].name
	})
	return genres
}

func (lib *Library) Artists(org organize) []*artistInfo {
	artists := make([]*artistInfo, 0, len(lib.artists))
	for _, a := range lib.artists {
		// TODO: year etc?

		// filter by folder
		if !org.ssid.IsZero() && !a.matches(org.ssid) {
			continue
		}

		// filter by starred
		if org.starred && a.starred.IsZero() {
			continue
		}

		if org.query != "" {
			// TODO: fancier?
			q := strings.ToLower(org.query)
			if !strings.Contains(strings.ToLower(a.name), q) {
				continue
			}
		}

		artists = append(artists, a)
	}

	// TODO: sorting?
	sort.Slice(artists, func(i, j int) bool {
		return artists[i].name < artists[j].name
	})

	return artists
}

func (lib *Library) Albums(org organize) []*albumInfo {
	albums := make([]*albumInfo, 0, len(lib.albums))
	for _, a := range lib.albums {
		// filter by year
		if org.fromYear != 0 || org.toYear != 0 {
			if org.fromYear < org.toYear {
				if org.fromYear != 0 && a.lowYear < org.fromYear {
					continue
				}
				if org.toYear != 0 && a.highYear > org.toYear {
					continue
				}
			} else {
				if org.fromYear != 0 && a.highYear > org.fromYear {
					continue
				}
				if org.toYear != 0 && a.lowYear < org.toYear {
					continue
				}
			}
		}

		// filter by genre
		if org.genre != "" {
			if _, ok := a.genres[org.genre]; !ok {
				continue
			}
		}

		// filter by folder
		if !org.ssid.IsZero() && !a.matches(org.ssid) {
			continue
		}

		// filter by starred
		if org.starred && a.starred.IsZero() {
			continue
		}

		if org.query != "" {
			// TODO: fancier?
			q := strings.ToLower(org.query)
			if !strings.Contains(strings.ToLower(a.name), q) {
				continue
			}
		}

		albums = append(albums, a)
	}

	// commonCond := []interface{}{
	// 	func(a, b tube.Track) int { return a.Disc - b.Disc },
	// 	func(a, b tube.Track) int { return a.Number - b.Number },
	// 	func(a, b tube.Track) int { return strings.Compare(a.Title, b.Title) },
	// 	func(a, b tube.Track) int { return strings.Compare(a.Filename, b.Filename) },
	// 	func(a, b tube.Track) int { return strings.Compare(a.ID, b.ID) },
	// }
	// _ = commonCond

	switch org.by {
	case "random":
		rand.Shuffle(len(albums), func(i, j int) {
			albums[i], albums[j] = albums[j], albums[i]
		})
	case "newest":
		order.By(
			func(a, b *albumInfo) int { return invert(a.date.Compare(b.date)) },
		).Sort(albums)
	case "highest":
		// TODO: ???
	case "frequent":
		order.By(
			func(a, b *albumInfo) int { return invert(a.plays - b.plays) },
			func(a, b *albumInfo) int { return invert(a.date.Compare(b.date)) },
			func(a, b *albumInfo) int { return strings.Compare(a.name, b.name) },
		).Sort(albums)
	case "recent":
		order.By(
			func(a, b *albumInfo) int { return invert(a.played.Compare(b.played)) },
			func(a, b *albumInfo) int { return strings.Compare(a.name, b.name) },
		).Sort(albums)
	case "alphabeticalByName":
		order.By(
			func(a, b *albumInfo) int { return strings.Compare(a.name, b.name) },
			func(a, b *albumInfo) int { return strings.Compare(a.artist, b.artist) },
			func(a, b *albumInfo) int { return strings.Compare(a.id, b.id) },
		).Sort(albums)
	case "byYear":
		order.By(
			func(a, b *albumInfo) int {
				if org.fromYear > org.toYear {
					return a.highYear - b.highYear
				}
				return a.lowYear - b.lowYear
			},
			func(a, b *albumInfo) int { return strings.Compare(a.name, b.name) },
			func(a, b *albumInfo) int { return strings.Compare(a.artist, b.artist) },
			func(a, b *albumInfo) int { return strings.Compare(a.id, b.id) },
		).Sort(albums)
	case "byGenre":
		order.By(
			func(a, b *albumInfo) int { return a.cmpgenre(b) },
			func(a, b *albumInfo) int { return strings.Compare(a.artist, b.artist) },
			func(a, b *albumInfo) int { return strings.Compare(a.id, b.id) },
		).Sort(albums)
	case "alphabeticalByArtist", "":
		fallthrough
	default:
		order.By(
			func(a, b *albumInfo) int { return strings.Compare(a.artist, b.artist) },
			func(a, b *albumInfo) int { return strings.Compare(a.name, b.name) },
			func(a, b *albumInfo) int { return strings.Compare(a.id, b.id) },
		).Sort(albums)
	}

	if org.offset > len(albums) {
		return []*albumInfo{}
	}
	albums = albums[org.offset:]

	if org.size != 0 && org.size < len(albums) {
		albums = albums[:org.size]
	}

	return albums
}

func (lib *Library) Tracks(org organize) []tube.Track {
	tracks := []tube.Track{}
	for _, t := range lib.tracks {
		// filter by genre
		if org.genre != "" && t.Genre != org.genre {
			continue
		}
		// filter by folder
		if !org.ssid.IsZero() && !t.MatchesSSID(org.ssid) {
			continue
		}
		// filter by starred
		if org.starred && t.Starred.IsZero() {
			continue
		}

		if org.query != "" {
			// TODO: fancier?
			q := strings.ToLower(org.query)
			if !strings.Contains(strings.ToLower(t.Title), q) {
				continue
			}
		}

		tracks = append(tracks, t)
	}
	if org.offset > len(tracks) {
		return []tube.Track{}
	}
	tracks = tracks[org.offset:]

	if org.size != 0 && org.size < len(tracks) {
		tracks = tracks[:org.size]
	}
	return tracks
}

func (lib *Library) TrackByID(id string) (tube.Track, bool) {
	t, ok := lib.idmap[id]
	return t, ok
}

func (lib *Library) TracksByID(ids []string) []tube.Track {
	tracks := make([]tube.Track, 0, len(ids))
	for _, id := range ids {
		t, ok := lib.TrackByID(id)
		if !ok {
			continue
		}
		tracks = append(tracks, t)
	}
	return tracks
}

func (lib *Library) Query(q string) ([]tube.Track, error) {
	prog, err := compile(q)
	if err != nil {
		return nil, err
	}
	var tracks []tube.Track
	for _, t := range lib.tracks {
		env := NewExprEnv(t)
		out, err := expr.Run(prog, env)
		if err != nil {
			return nil, err
		}
		if include, ok := out.(bool); include && ok {
			tracks = append(tracks, t)
		}
	}
	return tracks, nil
}

func (lib *Library) starred(ssid tube.SSID) time.Time {
	star := lib.stars[ssid]
	return star.Date
}

type organize struct {
	by       string
	size     int
	offset   int
	fromYear int
	toYear   int
	genre    string
	ssid     tube.SSID
	query    string
	starred  bool
}

func getLibrary(ctx context.Context, u tube.User) (*Library, error) {
	wg, ctx := errgroup.WithContext(ctx)
	var tracks []tube.Track
	var stars map[tube.SSID]tube.Star
	wg.Go(func() error {
		var err error
		tracks, err = u.GetTracks(ctx)
		return err
	})
	wg.Go(func() error {
		var err error
		stars, err = tube.GetStars(ctx, u.ID)
		return err
	})
	if err := wg.Wait(); err != nil {
		return nil, err
	}
	lib := NewLibrary(tracks, stars)
	return lib, nil
}

func resetCache(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	if err := tube.RecreateDump(ctx, u.ID, time.Now().UTC()); err != nil {
		panic(err)
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
