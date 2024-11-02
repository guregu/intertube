package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strings"

	// "github.com/aws/aws-lambda-go/events"
	// "github.com/aws/aws-lambda-go/lambda"

	"github.com/guregu/tag"
	"github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
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
	renderTemplate(ctx, w, "upload", data, http.StatusOK)
}

func handleUpload(ctx context.Context, key string, user tube.User, b2ID string) (tube.Track, error) {
	id := path.Base(key)

	fmeta, err := tube.GetFile(ctx, id)
	if err != nil {
		return tube.Track{}, err
	}

	if fmeta.TrackID != "" {
		log.Println("already exists?", fmeta.TrackID)
		track, err := tube.GetTrack(ctx, user.ID, fmeta.TrackID)
		if err == nil {
			return track, nil
		}
		log.Println("error getting pre-existing track:", err)
	}

	log.Println("get file ...")

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
		case ".ogg":
			format = tag.OGG
		}
	}
	if format != tag.MP3 && format != tag.FLAC && format != tag.M4A && format != tag.OGG {
		return tube.Track{}, fmt.Errorf("only mp3/flac/m4a supported right now (got: %v)", format)
	}
	raw.Seek(0, io.SeekStart)

	log.Println("calcDuration ...")

	dur, err := calcDuration(raw, format)
	if err != nil && !skippableError(err) {
		return tube.Track{}, err
	}
	raw.Seek(0, io.SeekStart)

	// var tags tag.Metadata
	var tags multiMeta
	if format == tag.OGG {
		if got, err := tag.ReadOGGTags(raw); err == nil {
			tags = append(tags, got)
		}
		raw.Seek(0, io.SeekStart)
	}
	if got, err := tag.ReadID3v2Tags(raw); err == nil {
		tags = append(tags, got)
	}
	// spew.Dump(tags)
	raw.Seek(0, io.SeekStart)
	if got, err := tag.ReadFrom(raw); err == nil {
		tags = append(tags, got)
	}
	raw.Seek(0, io.SeekStart)
	tags = append(tags, guessMetadata(fmeta.Name, format))
	unfuckID3(tags)
	raw.Seek(0, io.SeekStart)

	log.Println("tag.SumAll ...")

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

	log.Println("copyUploadToFiles ...")
	err = copyUploadToFiles(ctx, track.StorageKey(), b2ID, fmeta)
	if err != nil {
		return tube.Track{}, err
	}

	if pic := tags.Picture(); pic != nil {
		log.Println("savePic ...")
		track.Picture, err = savePic(pic.Data, pic.Ext, pic.Type, pic.Description)
		if err != nil {
			return tube.Track{}, err
		}
	}

	log.Println("track.Create ...")

	if err := track.Create(ctx); err != nil {
		return tube.Track{}, err
	}

	log.Println("SetTrackID ...")

	if err := fmeta.SetTrackID(track.ID); err != nil {
		return tube.Track{}, err
	}

	return track, nil
}

var replacementChar = "�"

func savePic(data []byte, ext string, mimetype string, desc string) (tube.Picture, error) {
	id, err := sha3Sum(data)
	if err != nil {
		return tube.Picture{}, err
	}
	pic := tube.Picture{
		ID:   id,
		Ext:  ext,
		Type: mimetype,
		Desc: desc,
	}
	err = storage.FilesBucket.Put(mimetype, pic.StorageKey(), bytes.NewReader(data))
	return pic, err
}

// for images
func sha3Sum(b []byte) (string, error) {
	sum := sha3.Sum224(b)
	str := base64.RawURLEncoding.EncodeToString(sum[:])
	return str, nil
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
		defer stream.Close()
		sec := stream.Info.NSamples / uint64(stream.Info.SampleRate)
		return int(sec), nil
	case tag.M4A:
		// TODO: need to find a go library with a proper license that parses these
		return 0, nil
		// secs, err := mp4util.Duration(r)
		// if err != nil {
		// return 0, err
		// }
		// return secs, nil
	case tag.OGG:
		length, format, err := oggvorbis.GetLength(r)
		if err != nil {
			// TODO: verify
			log.Println("OGG ERROR:", err)
			return 0, nil
		}
		sec := length / int64(format.SampleRate)
		return int(sec), nil
	}
	return 0, fmt.Errorf("unknown type: %v", ftype)
}

func skippableError(err error) bool {
	if err == nil {
		return true
	}
	str := err.Error()
	// mp3 package chokes on certain files, so let it fail
	return strings.Contains(str, "mp3:")
}
