package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/guregu/kami"

	"github.com/guregu/intertube/storage"
	"github.com/guregu/intertube/tube"
)

const (
	maxFileSize = 1024 * 1024 * 1024 // 1GB

	fileDownloadTTL      = 1 * time.Hour
	thumbnailDownloadTTL = 1 * time.Hour
	uploadTTL            = 4 * time.Hour
)

func downloadTrack(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	id := kami.Param(ctx, "id")
	if ext := path.Ext(id); ext != "" {
		id = id[:len(id)-len(ext)]
	}

	f, err := tube.GetTrack(ctx, u.ID, id)
	if err == tube.ErrNotFound {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		panic(err)
	}

	href, err := storage.FilesBucket.PresignGet(f.StorageKey(), fileDownloadTTL)
	if err != nil {
		panic(err)
	}
	http.Redirect(w, r, href, http.StatusTemporaryRedirect)
}

func uploadStart(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	name := r.FormValue("name")
	filetype := r.FormValue("type")
	size, err := strconv.ParseInt(r.FormValue("size"), 10, 64)
	if err != nil {
		panic(err)
	}
	if size == 0 {
		panic("missing file size")
	}
	var localMod int64
	if msec, err := strconv.ParseInt(r.FormValue("lastmod"), 10, 64); err == nil {
		localMod = msec
	}

	w.Header().Set("Tube-Upload-Usage", strconv.FormatInt(u.Usage, 10))
	w.Header().Set("Tube-Upload-Quota", strconv.FormatInt(u.CalcQuota(), 10))
	if size > maxFileSize {
		w.WriteHeader(400)
		fmt.Fprintln(w, "file too big. max size is "+strconv.FormatInt(maxFileSize/1000/1000, 10)+"MB")
		return
	}
	if (u.CalcQuota() != 0) && (u.Usage+size > u.CalcQuota()) {
		w.WriteHeader(400)
		fmt.Fprintln(w, "upload quota exceeded")
		return
	}

	zf := tube.NewFile(u.ID, name, size)
	zf.Type = filetype // TODO
	zf.LocalMod = localMod
	if err := zf.Create(ctx); err != nil {
		panic(err)
	}

	if storage.UploadsBucket.Exists(zf.Path()) {
		panic("already exists?!")
	}

	disp := encodeContentDisp(name)
	url, err := storage.UploadsBucket.PresignPut(zf.Path(), size, disp, uploadTTL)
	if err != nil {
		panic(err)
	}

	var data = struct {
		ID    string
		CD    string
		URL   string
		Token string
	}{
		ID:  zf.ID,
		CD:  disp,
		URL: url,
	}

	w.Header().Set("Tube-Upload-ID", zf.ID)
	renderJSON(w, data, http.StatusOK)
}

func uploadStart2(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	var input []struct {
		Name     string
		Type     string // mimetype
		Size     int64
		LocalMod int64 `json:"lastmod"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		panic(err)
	}

	type meta struct {
		ID  string
		CD  string
		URL string
	}
	output := make([]meta, 0, len(input))

	var totalsize int64
	for _, f := range input {
		if f.Size == 0 {
			panic("missing file size")
		}
		if f.Size > maxFileSize {
			w.WriteHeader(400)
			fmt.Fprintln(w, "file too big. max size is "+strconv.FormatInt(maxFileSize/1024/1024, 10)+"MB")
			return
		}
		totalsize += f.Size

		zf := tube.NewFile(u.ID, f.Name, f.Size)
		zf.Type = f.Type
		zf.LocalMod = f.LocalMod
		if err := zf.Create(ctx); err != nil {
			panic(err)
		}

		if storage.UploadsBucket.Exists(zf.Path()) {
			panic("already exists?! " + zf.ID)
		}

		disp := encodeContentDisp(f.Name)
		url, err := storage.UploadsBucket.PresignPut(zf.Path(), f.Size, disp, uploadTTL)
		if err != nil {
			panic(err)
		}

		output = append(output, meta{
			ID:  zf.ID,
			CD:  disp,
			URL: url,
		})
	}

	if quota := u.CalcQuota(); quota != 0 {
		if u.Usage+totalsize > quota {
			renderText(w, "file would exceed upload quota", http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Tube-Upload-Usage", strconv.FormatInt(u.Usage, 10))
	w.Header().Set("Tube-Upload-Quota", strconv.FormatInt(u.CalcQuota(), 10))
	renderJSON(w, output, http.StatusOK)
}

func ProcessUpload(ctx context.Context, f *tube.File, u tube.User, uploadPath string) (tube.Track, error) {
	if f.Deleted || f.UserID != u.ID {
		return tube.Track{}, fmt.Errorf("forbidden")
	}

	if err := f.SetStarted(ctx, time.Now().UTC()); err != nil {
		return tube.Track{}, err
	}

	head, err := storage.UploadsBucket.Head(f.Path())
	if err != nil {
		return tube.Track{}, fmt.Errorf("file not found in storage")
	}
	if err := f.Finish(ctx, head.Type, head.Size); err != nil {
		return tube.Track{}, err
	}
	if head.Size > maxFileSize {
		storage.FilesBucket.Delete(f.Path())
		return tube.Track{}, fmt.Errorf("file too big")
	}

	track, err := handleUpload(ctx, f.Path(), u, uploadPath)
	if err != nil {
		return track, err
	}
	if err := u.UpdateLastMod(ctx); err != nil {
		return track, err
	}
	return track, nil
}

func uploadFinish(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, ok := userFrom(ctx)
	if !ok {
		panic("no account")
	}
	bID := r.URL.Query().Get("bid")
	if bID == "" {
		panic("no bid")
	}

	id := kami.Param(ctx, "id")
	f, err := tube.GetFile(ctx, id)
	if err != nil {
		panic(err)
	}

	if f.Ready && f.TrackID != "" {
		track, err := tube.GetTrack(ctx, u.ID, f.TrackID)
		if err != nil {
			panic(err)
		}
		if err := json.NewEncoder(w).Encode(&track); err != nil {
			panic(err)
		}
		return
	}

	if !storage.UsingQueue() {
		track, err := ProcessUpload(ctx, &f, u, bID)
		if err != nil {
			panic(err)
		}
		if err := json.NewEncoder(w).Encode(&track); err != nil {
			panic(err)
		}
		return
	}

	if f.Queued.IsZero() {
		err = storage.EnqueueFile(storage.FileEvent{
			FileID: f.ID,
			UserID: u.ID,
			Path:   bID,
		})
		if err != nil {
			panic(err)
		}
		if err := f.SetQueued(ctx, time.Now().UTC()); err != nil {
			panic(err)
		}
	}

	w.Header().Set("Tube-Upload-Status", f.Status())
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(f); err != nil {
		panic(err)
	}
}

func encodeContentDisp(filename string) string {
	ext := path.Ext(filename)
	// return "attachment; filename*=UTF-8''" + url.PathEscape(filename)
	escaped := url.QueryEscape(filename)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	return "attachment; filename=\"file" + ext + "\"; filename*=UTF-8''" + escaped
}

func copyUploadToFiles(ctx context.Context, dstPath string, fileID string, f tube.File) error {
	disp := "attachment; filename*=UTF-8''" + escapeFilename(f.Name)
	return storage.FilesBucket.CopyFromBucket(dstPath, storage.UploadsBucket, f.Path(), f.Type, disp)
}

func presignTrackDL(_ tube.User, track tube.Track) string {
	href, err := storage.FilesBucket.PresignGet(track.StorageKey(), fileDownloadTTL*2)
	if err != nil {
		panic(err)
	}
	return href
}

func escapeFilename(name string) string {
	const illegal = `<>@,;:\"/+[]?={} 	`
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(illegal, r) {
			return '-'
		}
		return r
	}, name)
	name = url.PathEscape(name)
	if len(name) == 0 {
		return "file"
	}
	return name
}
