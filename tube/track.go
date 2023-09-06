package tube

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/guregu/dynamo"
)

const (
	UnknownArtist = "Unknown Artist"
	UnknownAlbum  = "Unknown Album"
	UnknownTitle  = "Untitled"
)

type Track struct {
	UserID int       `dynamo:",hash" index:"UserID-SortID-index,hash" index:"UserID-Date-index,hash"`
	ID     string    `dynamo:",range"`
	SortID string    `index:"UserID-SortID-index,range" badgerhold:"index"`
	Date   time.Time `index:"UserID-Date-index,range"`

	// proper case:
	Info TrackInfo

	// should be lowercase:
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Composer    string
	Genre       string
	Comment     string

	Number  int
	Total   int
	Disc    int
	Discs   int
	Year    int
	Picture Picture  `dynamo:",omitempty"`
	Tags    []string `dynamo:",set"` // user-defined tags

	Filename string
	Filetype string
	UploadID string
	Size     int
	Duration int // seconds

	TagFormat string
	Metadata  map[string]interface{} // IDv3 tags

	LastMod  time.Time
	LocalMod int64 // lastMod from client at upload time
	Dirty    bool

	Plays      int
	LastPlayed time.Time
	Resume     float64   // seconds
	ResumeMod  time.Time `dynamo:",omitempty"`

	Deleted bool

	// view only
	Starred time.Time `dynamo:"-"`
	DL      string    `dynamo:"-" json:",omitempty"`
}

func (Track) CreateTable(create *dynamo.CreateTable) {
	create.Stream(dynamo.NewAndOldImagesView)
}

type TrackInfo struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Composer    string
	Genre       string
	Comment     string
}

func (ti *TrackInfo) Sanitize() {
	if !utf8.ValidString(ti.Title) {
		ti.Title = strings.ToValidUTF8(ti.Title, "�")
	}
	if !utf8.ValidString(ti.Artist) {
		ti.Artist = strings.ToValidUTF8(ti.Artist, "�")
	}
	if !utf8.ValidString(ti.Album) {
		ti.Album = strings.ToValidUTF8(ti.Album, "�")
	}
	if !utf8.ValidString(ti.AlbumArtist) {
		ti.AlbumArtist = strings.ToValidUTF8(ti.AlbumArtist, "�")
	}
	if !utf8.ValidString(ti.Composer) {
		ti.Composer = strings.ToValidUTF8(ti.Composer, "�")
	}
	if !utf8.ValidString(ti.Genre) {
		ti.Genre = strings.ToValidUTF8(ti.Genre, "�")
	}
	if !utf8.ValidString(ti.Comment) {
		ti.Comment = strings.ToValidUTF8(ti.Comment, "�")
	}
}

func (t TrackInfo) AnyArtist() string {
	if t.AlbumArtist != "" {
		return t.AlbumArtist
	}
	if t.Artist != "" {
		return t.Artist
	}
	if t.Composer != "" {
		return t.Composer
	}
	return ""
}

func (t *Track) ApplyInfo(info TrackInfo) {
	t.Info = info
	t.Title = strings.ToLower(info.Title)
	t.Artist = strings.ToLower(info.Artist)
	t.Album = strings.ToLower(info.Album)
	t.AlbumArtist = strings.ToLower(info.AlbumArtist)
	t.Composer = strings.ToLower(info.Composer)
	t.Genre = strings.ToLower(info.Genre)
	t.Comment = strings.ToLower(info.Comment)
}

func (t *Track) Create(ctx context.Context) error {
	t.Date = time.Now().UTC()
	t.SortID = t.SortKey()

	tracks := dynamoTable("Tracks")
	var old Track
	err := tracks.Put(t).OldValue(&old)
	if err == ErrNotFound {
		// new file, so inc usage
		_, err := AddUsage(ctx, t.UserID, int64(t.Size), 1)
		return err
	}
	return err
}

func (t *Track) Save(ctx context.Context) error {
	t.SortID = t.SortKey()

	tracks := dynamoTable("Tracks")
	return tracks.Put(t).Run()
}

func (t *Track) Delete(ctx context.Context) error {
	size := t.Size
	if size == 0 {
		f, err := GetFile(ctx, t.UploadID)
		if err != nil {
			return err
		}
		size = int(f.Size)
	}

	tracks := dynamoTable("Tracks")
	if err := tracks.Delete("UserID", t.UserID).Range("ID", t.ID).Run(); err != nil {
		return err
	}
	users := dynamoTable(tableUsers)
	return users.Update("ID", t.UserID).
		Add("Usage", -size).
		Add("Tracks", -1).
		Run()
}

func (t *Track) IncPlays(ctx context.Context) error {
	tracks := dynamoTable("Tracks")
	return tracks.Update("UserID", t.UserID).Range("ID", t.ID).
		Add("Plays", 1).
		Set("LastPlayed", time.Now().UTC()).
		Value(t)
}

func (t *Track) SetResume(ctx context.Context, secs float64, modTime time.Time) error {
	tracks := dynamoTable("Tracks")
	return tracks.Update("UserID", t.UserID).Range("ID", t.ID).
		Set("Resume", secs).
		Set("ResumeMod", modTime).
		If("attribute_not_exists('ResumeMod') OR 'ResumeMod' <= ?", modTime).
		Value(t)
}

func (t *Track) SetDuration(ctx context.Context, secs int) error {
	tracks := dynamoTable("Tracks")
	return tracks.Update("UserID", t.UserID).Range("ID", t.ID).
		Set("Duration", secs).
		If("attribute_not_exists('Duration') OR 'Duration' <= ?", secs).
		Value(t)
}

func (t *Track) RefreshSortID(ctx context.Context) error {
	tracks := dynamoTable("Tracks")
	return tracks.Update("UserID", t.UserID).Range("ID", t.ID).
		Set("SortID", t.SortKey()).
		Value(t)
}

func (t Track) StorageKey() string {
	return fmt.Sprintf("u/tracks/%d/%s%s", t.UserID, t.ID, path.Ext(t.Filename))
}

func (t Track) SortKey() string {
	artist := strings.ReplaceAll(t.AnyArtist(), " ", "-")
	if artist == "" {
		artist = "~{artist}"
	}
	if len(artist) > 100 {
		artist = artist[:100]
	}
	album := strings.ReplaceAll(t.Album, " ", "-")
	if album == "" {
		album = "~{album}"
	}
	if len(album) > 100 {
		album = album[:100]
	}
	title := strings.ReplaceAll(t.Title, " ", "-")
	if title == "" {
		title = strings.ReplaceAll(t.Filename, " ", "-")
	}
	if title == "" {
		title = "~{title}"
	}
	if len(title) > 100 {
		title = title[:100]
	}
	return fmt.Sprintf("%s %s %04d %06d %s %s", artist, album, t.Disc, t.Number, title, t.ID)
}

func (t Track) FileURL() string {
	return fmt.Sprintf("/dl/tracks/%s%s", t.ID, path.Ext(t.Filename))
}

// VirtualPath returns an ideal path as synchronized to a filesystem
func (t Track) VirtualPath() string {
	scrub := func(name string) string {
		name = strings.ReplaceAll(name, "/", "-")
		name = strings.ReplaceAll(name, "\\", "-")
		name = strings.ReplaceAll(name, "...", "")
		name = strings.ReplaceAll(name, "..", "")
		const cutset = `<>:"|?*`
		for _, chr := range cutset {
			name = strings.ReplaceAll(name, string(chr), "")
		}
		return name
	}
	artist := t.Info.AlbumArtist
	if artist == "" {
		if t.Info.Artist != "" {
			artist = t.Info.Artist
		} else {
			artist = "Unknown Artist"
		}
	}
	album := t.Info.Album
	if album == "" {
		album = "Unknown Album"
	}
	title := t.Info.Title
	if title == "" {
		if t.Filename != "" {
			title = t.Filename
		} else {
			title = "Untitled"
		}
	}
	var num string
	if t.Disc != 0 && t.Number != 0 {
		num = fmt.Sprintf("%d-%02d ", t.Disc, t.Number)
	} else if t.Number != 0 {
		num = fmt.Sprintf("%02d ", t.Number)
	}
	filename := scrub(num + title + t.Ext())
	return path.Join(artist, album, filename)
}

func (t Track) LastModOrDate() time.Time {
	if !t.LastMod.IsZero() {
		return t.LastMod
	}
	return t.Date
}

func (t Track) MIMEType() string {
	switch t.Filetype {
	case "MP3":
		return "audio/mp3"
	case "FLAC":
		return "audio/flac"
	case "M4A":
		return "audio/mp4"
	}
	// return "binary/octet-stream"
	return ""
}

func (t Track) Ext() string {
	switch t.Filetype {
	case "MP3":
		return ".mp3"
	case "FLAC":
		return ".flac"
	case "M4A":
		return ".m4a"
	}
	return ""
}

func (t Track) ArtistEqual(other Track) bool {
	if t.Artist != "" && t.Artist == other.Artist {
		return true
	}
	if t.AlbumArtist != "" && t.AlbumArtist == other.AlbumArtist {
		return true
	}
	if t.Composer != "" && t.Composer == other.Composer {
		return true
	}
	return false
}

func (t Track) AlbumEqual(other Track) bool {
	if !t.ArtistEqual(other) {
		return false
	}
	return t.Album == other.Album
}

func (t Track) AnyArtist() string {
	if t.AlbumArtist != "" {
		return t.AlbumArtist
	}
	if t.Artist != "" {
		return t.Artist
	}
	if t.Composer != "" {
		return t.Composer
	}
	return ""
}

// func (t Track) AlbumCode() string {
// return base64.RawURLEncoding.EncodeToString([]byte(t.AnyArtist() + " " + t.Album))
// }

func (t Track) AlbumCode() string {
	artist := t.AnyArtist()
	b := make([]byte, 0, len(artist)+1+len(t.Album))
	b = append(b, []byte(artist)...)
	b = append(b, '\x1F')
	b = append(b, []byte(t.Album)...)
	sum := sha256.Sum256(b)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (t Track) ArtistSSID() SSID {
	// TODO: fix
	return NewSSID(SSIDArtist, t.AnyArtist())
}

func (t Track) AlbumSSID() SSID {
	// TODO: fix
	return NewSSID(SSIDAlbum, t.AlbumCode())
}

func (t Track) TrackSSID() SSID {
	return NewSSID(SSIDTrack, t.ID)
}

func (t Track) MatchesSSID(ssid SSID) bool {
	// TODO fix
	// ssid := ParseSSID(id)
	switch ssid.Kind {
	default:
		fallthrough
	case SSIDTrack, SSIDInvalid:
		return ssid.ID == t.ID
	case SSIDAlbum:
		return t.AlbumSSID() == ssid
	case SSIDArtist:
		return t.ArtistSSID() == ssid
	case SSIDFolder:
		return ssid == MusicFolder // TODO: other folders?
	}
}

// TODO: improve
func (t Track) Bitrate() int {
	if t.Duration == 0 {
		return 320
	}
	//           const bitrate = Math.floor((file.size * 0.008) / duration);
	guess := float64(t.Size) * 0.008 / float64(t.Duration)
	return int(guess)
}

func (t Track) Env() map[string]interface{} {
	/*
			type TrackInfo struct {
			Title       string
			Artist      string
			Album       string
			AlbumArtist string
			Composer    string
			Genre       string
			Comment     string
		}
	*/
	/*
			Number  int
		Total   int
		Disc    int
		Discs   int
		Year    int
		Picture Picture  `dynamo:",omitempty"`
		Tags    []string `dynamo:",set"` // user-defined tags

		Filename string
		Filetype string
		UploadID string
		Size     int
		Duration int // seconds

		TagFormat string
		Metadata  map[string]interface{} // IDv3 tags

		LastMod  time.Time
		LocalMod int64 // lastMod from client at upload time
		Dirty    bool

		Plays      int
		LastPlayed time.Time
		Resume     float64   // seconds
		ResumeMod  time.Time `dynamo:",omitempty"`
	*/
	return map[string]interface{}{
		"id":         t.ID,
		"ssid":       t.TrackSSID(),
		"artistssid": t.ArtistSSID(),

		"number": t.Number,
		"total":  t.Total,
		"disc":   t.Disc,
		"discs":  t.Discs,
		"year":   t.Year,

		"filename": t.Filename,
		"filetype": t.Filetype,
		"size":     t.Size,
		"duration": t.Duration,

		"plays":    t.Plays,
		"lastplay": t.LastPlayed,
		"resume":   t.Resume,
		// "resumemod": t.ResumeMod,

		"title":       t.Info.Title,
		"artist":      t.Info.Artist,
		"album":       t.Info.Album,
		"albumartist": t.Info.AlbumArtist,
		"composer":    t.Info.Composer,
		"genre":       t.Info.Genre,
		"comment":     t.Info.Comment,
	}
}

type Picture struct {
	ID   string
	Type string
	Ext  string
	Desc string
}

func (p Picture) StorageKey() string {
	return fmt.Sprintf("pic/%s.%s", p.ID, p.Ext)
}

func (u User) GetTracks(ctx context.Context) (Tracks, error) {
	if useDump {
		if d, err := u.GetDump(); err == nil {
			log.Println("using dump for", u.ID, "@", d.Time, d.Key())
			return d.Tracks, nil
		}
	}
	tracks, _, err := GetTracksPartialSorted(ctx, u.ID, 0, nil)
	return tracks, err
}

func GetTracks(ctx context.Context, userID int) (Tracks, error) {
	table := dynamoTable("Tracks")
	var tracks Tracks
	err := table.Get("UserID", userID).Consistent(true).All(&tracks)
	sort.Sort(tracks)
	return tracks, err
}

func GetTracksInfo(ctx context.Context, userID int) (Tracks, error) {
	table := dynamoTable("Tracks")
	var tracks []Track
	err := table.Get("UserID", userID).Project("ID", "UserID", "Info").All(&tracks)
	return tracks, err
}

func GetTracksPartial(ctx context.Context, userID int, limit int64, startFrom dynamo.PagingKey) (Tracks, dynamo.PagingKey, error) {
	table := dynamoTable("Tracks")
	var tracks Tracks
	q := table.Get("UserID", userID)
	if limit > 0 {
		q.SearchLimit(limit)
	}
	if startFrom != nil {
		q.StartFrom(startFrom)
	}
	next, err := q.AllWithLastEvaluatedKey(&tracks)
	return tracks, next, err
}

func GetTracksPartialSorted(ctx context.Context, userID int, limit int64, startFrom dynamo.PagingKey) (Tracks, dynamo.PagingKey, error) {
	table := dynamoTable("Tracks")
	var tracks Tracks
	q := table.Get("UserID", userID)
	q.Index("UserID-SortID-index")
	if limit > 0 {
		q.SearchLimit(limit)
	}
	if startFrom != nil {
		q.StartFrom(startFrom)
	}
	next, err := q.AllWithLastEvaluatedKey(&tracks)
	return tracks, next, err
}

func GetTracksBatch(ctx context.Context, userID int, trackIDs []string) (Tracks, error) {
	table := dynamoTable("Tracks")
	batch := table.Batch("UserID", "ID").Get()
	for _, id := range trackIDs {
		batch.And(dynamo.Keys{userID, id})
	}
	var tracks Tracks
	err := batch.All(&tracks)
	return tracks, err
}

func GetTrack(ctx context.Context, userID int, trackID string) (Track, error) {
	table := dynamoTable("Tracks")
	var track Track
	err := table.Get("UserID", userID).Range("ID", dynamo.Equal, trackID).One(&track)
	return track, err
}

func GetALLTracks(ctx context.Context) dynamo.Iter {
	table := dynamoTable("Tracks")
	return table.Scan().Iter()
}

func CountTracks(ctx context.Context, userID int) (int64, error) {
	table := dynamoTable("Tracks")
	ct, err := table.Get("UserID", userID).Count()
	return ct, err
}

func CountTracks2(ctx context.Context, userID int) (int64, error) {
	table := dynamoTable("Tracks")
	ct, err := table.Get("UserID", userID).Index("UserID-SortID-index").Count()
	return ct, err
}

func MassUpdateTracks(ctx context.Context, userID int, trackIDs []string, mutator func(*dynamo.Update)) error {
	// TODO: rewrite
	table := dynamoTable("Tracks")

	var wg sync.WaitGroup
	for _, id := range trackIDs {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			u := table.Update("UserID", userID).Range("ID", id)
			mutator(u)
			u.If("attribute_exists('ID')")
			var t Track
			if err := u.Value(&t); err != nil {
				fmt.Println("mass err:", err)
				return
			}
			if err := t.RefreshSortID(ctx); err != nil {
				fmt.Println("mass sid err:", err)
				return
			}
		}()
	}
	wg.Wait()
	return nil
}

func DeleteTrack(ctx context.Context, userID int, trackID string) error {
	track, err := GetTrack(ctx, userID, trackID)
	if err != nil {
		return err
	}
	return track.Delete(ctx)
}

func IncTotalPlays(ctx context.Context, secs int) error {
	_, err := NextID(ctx, "TotalPlays")
	if err != nil {
		return err
	}
	if secs <= 0 {
		return nil
	}
	table := dynamoTable("Counters")
	return table.Update("ID", "TotalTime").
		Add("Count", secs).
		Run()
}

type Tracks []Track

func (tt Tracks) IDs() []string {
	ids := make([]string, 0, len(tt))
	for _, t := range tt {
		ids = append(ids, t.ID)
	}
	return ids
}

func (tt Tracks) Less(i, j int) bool {
	return tt[i].SortID < tt[j].SortID
}

func (tt Tracks) Swap(i, j int) {
	tt[i], tt[j] = tt[j], tt[i]
}

func (tt Tracks) Len() int {
	return len(tt)
}

var _ sort.Interface = Tracks{}
