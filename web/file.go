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
	cfAuthURL = "https://intertube.download/auth?token=%s&dl=%s" // token, track.B2Path
	cfFileURL = "https://intertube.download/dl/%s?token=%s"      // track.B2Path, token

	maxFileSize = 500 * 1000 * 1000 // 500MB

	fileDownloadTTL      = 1 * time.Hour
	thumbnailDownloadTTL = 1 * time.Hour
	uploadTTL            = 4 * time.Hour
)

// intertube.download/auth?token=XYZ&r={home/dl}
//
//	set cookie, redir to inter.tube
//
// intertube.download/file/...
//
//	check cookie
//
// https://intertube.download/auth?token=B2_TOKEN?dl=USERID/FILENAME
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

	href, err := storage.FilesBucket.PresignGet(f.B2Key(), fileDownloadTTL)
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

	w.Header().Set("Tube-Upload-ID", zf.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

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

	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
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
			fmt.Fprintln(w, "file too big. max size is "+strconv.FormatInt(maxFileSize/1000/1000, 10)+"MB")
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
			w.WriteHeader(400)
			fmt.Fprintln(w, "would exceed upload quota")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Tube-Upload-Usage", strconv.FormatInt(u.Usage, 10))
	w.Header().Set("Tube-Upload-Quota", strconv.FormatInt(u.CalcQuota(), 10))
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(output); err != nil {
		panic(err)
	}
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
	if f.Deleted || f.UserID != u.ID {
		w.WriteHeader(http.StatusForbidden)
		fmt.Println("nope")
		return
	}

	head, err := storage.UploadsBucket.Head(f.Path())
	if err != nil {
		panic(err)
	}
	if err := f.Finish(ctx, head.Type, head.Size); err != nil {
		panic(err)
	}
	if head.Size > maxFileSize {
		storage.FilesBucket.Delete(f.Path())
		w.WriteHeader(400)
		fmt.Println("nice try. file too big.")
	}

	track, err := handleUpload(ctx, f.Path(), u, bID)
	if err != nil {
		panic(err)
	}
	if err := u.UpdateLastMod(ctx); err != nil {
		panic(err)
	}

	if err := json.NewEncoder(w).Encode(&track); err != nil {
		panic(err)
	}
}

func DeleteFile(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	id := kami.Param(ctx, "id")
	f, err := tube.GetFile(ctx, id)
	if err != nil {
		panic(err)
	}
	if f.UserID != u.ID {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, "Forbidden")
		return
	}
	if err := f.Delete(ctx); err != nil {
		panic(err)
	}
	http.Redirect(w, r, "//"+Domain+"/account/files", http.StatusSeeOther)
}

func encodeContentDisp(filename string) string {
	ext := path.Ext(filename)
	// return "attachment; filename*=UTF-8''" + url.PathEscape(filename)
	escaped := url.QueryEscape(filename)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	return "attachment; filename=\"file" + ext + "\"; filename*=UTF-8''" + escaped
}
