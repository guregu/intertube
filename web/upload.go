package web

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	// "github.com/aws/aws-lambda-go/events"
	// "github.com/aws/aws-lambda-go/lambda"

	"github.com/guregu/tag"
	"github.com/hajimehoshi/go-mp3"
	"github.com/mewkiz/flac"
	"golang.org/x/crypto/sha3"

	"github.com/guregu/intertube/storage"
	"github.com/guregu/intertube/tube"
)

func uploadForm(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	// test lol
	tracks, err := u.GetTracks(ctx)
	if err != nil {
		panic(err)
	}
	lib := NewLibrary(tracks, nil)
	type meta struct {
		LastMod int64
		Size    int
	}
	dupes := make(map[string][]meta)
	for _, t := range lib.Tracks(organize{}) {
		dupes[t.Filename] = append(dupes[t.Filename], meta{
			Size:    t.Size,
			LastMod: t.LocalMod,
		})
	}

	data := struct {
		User  tube.User
		Dupes map[string][]meta
	}{
		User:  u,
		Dupes: dupes,
	}
	if err := getTemplate(ctx, "upload").Execute(w, data); err != nil {
		panic(err)
	}
}

func handleUpload(ctx context.Context, key string, user tube.User, b2ID string) (tube.Track, error) {
	id := path.Base(key)

	fmeta, err := tube.GetFile(ctx, id)
	if err != nil {
		return tube.Track{}, err
	}
	_ = fmeta

	r, err := storage.UploadsBucket.Get(key)
	if err != nil {
		return tube.Track{}, err
	}
	defer r.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return tube.Track{}, err
	}
	raw := bytes.NewReader(buf.Bytes())

	_, format, err := tag.Identify(raw)
	if err != tag.ErrNoTagsFound && err != nil {
		return tube.Track{}, err
	}
	// fmt.Println("GOT:", format, noTags)
	if format == tag.UnknownFileType {
		switch strings.ToLower(path.Ext(fmeta.Name)) {
		case ".mp3":
			format = tag.MP3
		case ".flac":
			format = tag.FLAC
		case ".m4a":
			format = tag.M4A
		}
	}
	if format != tag.MP3 && format != tag.FLAC && format != tag.M4A {
		return tube.Track{}, fmt.Errorf("only mp3/flac/m4a supported right now (got: %v)", format)
	}
	raw.Seek(0, io.SeekStart)

	dur, err := calcDuration(raw, format)
	if err != nil && !skippableError(err) {
		return tube.Track{}, err
	}
	raw.Seek(0, io.SeekStart)

	// var tags tag.Metadata
	var tags multiMeta
	if got, err := tag.ReadID3v1Tags(raw); err == nil {
		tags = append(tags, got)
	}
	raw.Seek(0, io.SeekStart)
	if got, err := tag.ReadFrom(raw); err == nil {
		tags = append(tags, got)
	}
	raw.Seek(0, io.SeekStart)
	tags = append(tags, guessMetadata(fmeta.Name, format))
	unfuckID3(tags)
	raw.Seek(0, io.SeekStart)

	sum, err := tag.SumAll(raw)
	if err != nil {
		return tube.Track{}, err
	}

	trackInfo := tube.TrackInfo{
		Title:       tags.Title(),
		Artist:      tags.Artist(),
		Album:       tags.Album(),
		AlbumArtist: tags.AlbumArtist(),
		Composer:    tags.Composer(),
		Genre:       tags.Genre(),
		Comment:     tags.Comment(),
	}
	trackInfo.Sanitize()

	// TODO: don't need this anyway?
	// meta := copyTags(tags.Raw(), "PIC", "APIC", "PIC\u0000")
	track := tube.Track{
		UserID: fmeta.UserID,
		ID:     sum,

		Year: tags.Year(),

		Filename: strings.ToValidUTF8(fmeta.Name, replacementChar),
		Filetype: string(tags.FileType()),
		UploadID: fmeta.ID,
		Size:     buf.Len(),
		LocalMod: fmeta.LocalMod,
		Duration: dur,

		TagFormat: string(tags.Format()),
		// Metadata:  meta,
	}
	track.Number, track.Total = tags.Track()
	track.Disc, track.Discs = tags.Disc()
	track.ApplyInfo(trackInfo)

	err = copyUploadToMain(ctx, track.B2Key(), b2ID, fmeta)
	if err != nil {
		return tube.Track{}, err
	}

	if pic := tags.Picture(); pic != nil {
		track.Picture, err = savePic(pic.Data, pic.Ext, pic.Type, pic.Description)
		if err != nil {
			return tube.Track{}, err
		}
	}

	if err := track.Create(ctx); err != nil {
		return tube.Track{}, err
	}

	if err := fmeta.SetTrackID(track.ID); err != nil {
		return tube.Track{}, err
	}

	return track, nil
}

var replacementChar = "�"

func savePic(data []byte, ext string, mimetype string, desc string) (tube.Picture, error) {
	id, err := sumBytes(data)
	if err != nil {
		return tube.Picture{}, err
	}
	pic := tube.Picture{
		ID:   id,
		Ext:  ext,
		Type: mimetype,
		Desc: desc,
	}
	err = storage.FilesBucket.Put(mimetype, pic.S3Key(), bytes.NewReader(data))
	return pic, err
}

// for files (TODO: use sha3?)
func sha1Sum(r io.ReadSeeker) (string, error) {
	h := sha1.New()
	_, err := io.Copy(h, r)
	if err != nil {
		return "", nil
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// for images
func sumBytes(b []byte) (string, error) {
	r := bytes.NewReader(b)
	hash := sha3.New224()
	_, err := io.Copy(hash, r)
	sum := base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
	return sum, err
}

func copyTags(tags map[string]interface{}, exclude ...string) map[string]interface{} {
	m := make(map[string]interface{}, len(tags))
next:
	for k, v := range tags {
		for _, ex := range exclude {
			if k == ex {
				continue next
			}
		}
		if vstr, ok := v.(string); ok {
			if len(vstr) == 0 {
				continue next
			}
			raw := []byte(vstr)
			// sometimes this sh*t isn't utf8 :c
			if !utf8.Valid(raw) {
				v = raw
			}
		}

		// wtf: some tags used fucked up non-utf8 encoding
		k = strings.TrimRight(k, "\u0000")
		if !utf8.ValidString(k) {
			k = strings.ToValidUTF8(k, replacementChar)
		}
		m[k] = v
	}
	return m
}

func mimetypeOfTrack(ftype tag.FileType) string {
	switch ftype {
	case tag.MP3:
		return "audio/mp3"
	case tag.FLAC:
		return "audio/flac"
	case tag.M4A:
		return "audio/mp4"
	}
	return "binary/octet-stream"
}

// secs
func calcDuration(r io.ReadSeeker, ftype tag.FileType) (int, error) {
	switch ftype {
	case tag.MP3:
		dec, err := mp3.NewDecoder(r)
		if err != nil {
			if strings.Contains(err.Error(), "free bitrate") {
				return 0, nil
			}
			return 0, err
		}
		sr := dec.SampleRate()
		length := dec.Length()
		if sr == 0 {
			return 0, nil
		}
		return (int(length) / sr) / 4, nil
	case tag.FLAC:
		stream, err := flac.Parse(r)
		if err != nil {
			return 0, err
		}
		sec := stream.Info.NSamples / uint64(stream.Info.SampleRate)
		return int(sec), nil
		// case tag.M4A:
		// 	secs, err := mp4util.Duration(r)
		// 	if err != nil {
		// 		return 0, err
		// 	}
		// 	return secs, nil
	}
	return 0, fmt.Errorf("unknown type: %v", ftype)
}

func skippableError(err error) bool {
	if err == nil {
		return true
	}
	str := err.Error()
	if strings.Contains(str, "mp3:") {
		return true
	}
	return false
}

type guessedMeta struct {
	ftype       tag.FileType
	title       string
	album       string
	artist      string
	albumArtist string
	track       int
	disc        int
}

func guessMetadata(name string, ftype tag.FileType) tag.Metadata {
	name = strings.TrimSuffix(name, path.Ext(name))
	if !strings.ContainsRune(name, ' ') {
		name = strings.ReplaceAll(name, "_", " ")
	}
	meta := guessedMeta{
		ftype: ftype,
	}
	parts := strings.Split(name, "-")
	var nums []int
	var strs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if n, err := strconv.Atoi(p); err == nil {
			nums = append(nums, n)
			continue
		}
		strs = append(strs, p)
	}
	fmt.Printf("n: %#v\n", nums)
	fmt.Printf("s: %#v\n", strs)

	if len(strs) == 0 {
		return guessedMeta{title: name, ftype: ftype}
	}

	// title + maybe track number
	// TODO: rewrite lol
	last := strs[len(strs)-1]
	if lsplit := strings.Split(last, " "); len(lsplit) >= 2 {
		maybeTrack := strings.TrimSuffix(lsplit[0], ".")
		if strings.ContainsRune(maybeTrack, '-') {
			nsplit := strings.Split(maybeTrack, "-")
			d, err1 := strconv.Atoi(nsplit[0])
			n, err2 := strconv.Atoi(nsplit[1])
			fmt.Println(d, err1, n, err2)
			if err1 == nil && err2 == nil {
				meta.disc = d
				meta.track = n
				meta.title = strings.Join(lsplit[1:], " ")
			} else {
				meta.title = last
			}
		} else if n, err := strconv.Atoi(maybeTrack); err == nil {
			meta.track = n
			meta.title = strings.Join(lsplit[1:], " ")
		} else {
			meta.title = last
		}
	} else {
		meta.title = strs[len(strs)-1]
	}

	if len(nums) > 0 {
		lastnum := nums[len(nums)-1]
		if meta.track != 0 {
			meta.disc = lastnum
		} else {
			meta.track = lastnum
		}
		// if meta.title == "" && len(nums) == 2 {
		// 	meta.title = strconv.Itoa(lastnum)
		// 	meta.track = nums[0]
		// }
	}

	switch len(strs) {
	case 1:
	case 2:
		meta.artist = strs[0]
	case 3:
		meta.artist = strs[0]
		meta.album = strs[1]
	case 4:
		meta.albumArtist = strs[0]
		meta.album = strs[1]
		meta.artist = strs[2]
	default:
		// give up
		meta.title = name
	}

	return meta
}

func (m guessedMeta) Format() tag.Format          { return tag.UnknownFormat }
func (m guessedMeta) FileType() tag.FileType      { return m.ftype }
func (m guessedMeta) Title() string               { return m.title }
func (m guessedMeta) Album() string               { return m.album }
func (m guessedMeta) Artist() string              { return m.artist }
func (m guessedMeta) Track() (int, int)           { return m.track, 0 }
func (m guessedMeta) Disc() (int, int)            { return m.disc, 0 }
func (m guessedMeta) AlbumArtist() string         { return "" }
func (m guessedMeta) Composer() string            { return "" }
func (m guessedMeta) Year() int                   { return 0 }
func (m guessedMeta) Genre() string               { return "" }
func (m guessedMeta) Picture() *tag.Picture       { return nil }
func (m guessedMeta) Lyrics() string              { return "" }
func (m guessedMeta) Comment() string             { return "" }
func (m guessedMeta) Raw() map[string]interface{} { return map[string]interface{}{} }

type multiMeta []tag.Metadata

func (m multiMeta) Format() tag.Format {
	for _, child := range m {
		if f := child.Format(); f != "" {
			return f
		}
	}
	return tag.UnknownFormat
}

func (m multiMeta) FileType() tag.FileType {
	for _, child := range m {
		if f := child.FileType(); f != "" && f != tag.UnknownFileType {
			return f
		}
	}
	return tag.UnknownFileType
}

func (m multiMeta) Title() string {
	return m.try(func(meta tag.Metadata) string { return meta.Title() })
}

func (m multiMeta) Album() string {
	return m.try(func(meta tag.Metadata) string { return meta.Album() })
}

func (m multiMeta) Artist() string {
	return m.try(func(meta tag.Metadata) string { return meta.Artist() })
}

func (m multiMeta) AlbumArtist() string {
	return m.try(func(meta tag.Metadata) string { return meta.AlbumArtist() })
}

func (m multiMeta) Composer() string {
	return m.try(func(meta tag.Metadata) string { return meta.Composer() })
}

func (m multiMeta) Genre() string {
	return m.try(func(meta tag.Metadata) string { return meta.Genre() })
}

func (m multiMeta) Lyrics() string {
	return m.try(func(meta tag.Metadata) string { return meta.Lyrics() })
}
func (m multiMeta) Comment() string {
	return m.try(func(meta tag.Metadata) string { return meta.Comment() })
}

func (m multiMeta) Track() (int, int) {
	for _, child := range m {
		a, b := child.Track()
		if a != 0 || b != 0 {
			return a, b
		}
	}
	return 0, 0
}

func (m multiMeta) Disc() (int, int) {
	for _, child := range m {
		a, b := child.Disc()
		if a != 0 || b != 0 {
			return a, b
		}
	}
	return 0, 0
}

func (m multiMeta) Year() int {
	for _, child := range m {
		x := child.Year()
		if x != 0 {
			return x
		}
	}
	return 0
}
func (m multiMeta) Picture() *tag.Picture {
	for _, child := range m {
		x := child.Picture()
		if x != nil {
			return x
		}
	}
	return nil
}

func (m multiMeta) Raw() map[string]interface{} {
	tags := map[string]interface{}{}
	for _, child := range m {
		if len(child.Raw()) > len(tags) {
			tags = child.Raw()
		}
	}
	return tags
}

func (m multiMeta) try(get func(tag.Metadata) string) string {
	var invalid string
	for _, child := range m {
		if str := get(child); str != "" {
			if !utf8.ValidString(str) {
				invalid = str
				continue
			}
			return str
		}
	}
	if invalid != "" {
		if valid := strings.ToValidUTF8(invalid, ""); valid != "" {
			return valid
		}
	}
	return ""
}

var (
	_ tag.Metadata = guessedMeta{}
	_ tag.Metadata = multiMeta{}
)
