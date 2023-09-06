package web

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	// "github.com/davecgh/go-spew/spew"
	"github.com/guregu/kami"

	"github.com/guregu/intertube/tube"
)

const (
	subsonicAPIPrefix      = "/rest/"
	subsonicAPIVersion     = "1.12.0" // TODO
	subsonicXMLNS          = "http://subsonic.org/restapi"
	subsonicTimeLayout     = "2006-01-02T15:04:05"
	subsonicIgnoreArticles = "The El La Los Las Le Les"
	subsonicMaxSize        = 500 // TODO: ignore?
)

func init() {
	add := func(path string, h any) {
		href := subsonicAPIPrefix + path + ".view"
		kami.Get(href, h)
		kami.Post(href, h)
		// some bad clients use capitalized URLs (clementine)
		href = subsonicAPIPrefix + strings.ToUpper(string(rune(path[0]))) + path[1:] + ".view"
		kami.Get(href, h)
		kami.Post(href, h)
		// i guess this is a thing too
		kami.Get(subsonicAPIPrefix+path, h)
		kami.Post(subsonicAPIPrefix+path, h)
	}

	// kami.Use(subsonicAPIPrefix, subsonic)
	add("ping", subsonicPing)
	add("getLicense", subsonicGetLicense)
	add("getUser", subsonicGetUser)
	add("getMusicFolders", subsonicGetMusicFolders)
	add("getMusicDirectory", subsonicGetMusicDirectory)
	add("getAlbumList", subsonicGetAlbumList1)
	add("getAlbumList2", subsonicGetAlbumList2)
	add("getAlbum", subsonicGetAlbum)
	add("getArtists", subsonicGetArtists)
	add("getIndexes", subsonicGetIndexes)
	add("getGenres", subsonicGetGenres)
	add("getArtist", subsonicGetArtist)
	add("getRandomSongs", subsonicGetRandomSongs)
	add("getSongsByGenre", subsonicGetSongsByGenre)
	add("getStarred", subsonicGetStarred)
	add("getStarred2", subsonicGetStarred)
	add("search2", subsonicSearch2)
	add("search3", subsonicSearch3)
	add("getSong", subsonicGetSong)
	add("getCoverArt", subsonicGetCoverArt)
	add("stream", subsonicStream)
	add("download", subsonicStream)
	add("scrobble", subsonicScrobble)
	add("star", subsonicStar)
	add("unstar", subsonicUnstar)
	add("getPlaylists", subsonicGetPlaylists)
	add("getPlaylist", subsonicGetPlaylist)
	add("createPlaylist", subsonicCreatePlaylist)
	add("updatePlaylist", subsonicUpdatePlaylist)
	add("deletePlaylist", subsonicDeletePlaylist)
	// TODO: unstub
	add("savePlayQueue", subsonicSavePlayQueue)
	add("getPlayQueue", subsonicGetPlayQueue)
	add("getArtistInfo", subsonicGetArtistInfo)
	add("getArtistInfo2", subsonicGetArtistInfo)
	add("getLyrics", subsonicGetLyrics)
	add("getNowPlaying", subsonicGetNowPlaying)

	// TODO:
	// getChatMessages, addChatMessage
	// setRating
	// bookmarks

	// to stub:
	// getInternetRadioStations
	// getTopSongs ?
	// podcast stuff
}

type subsonicResponse struct {
	XMLName xml.Name `xml:"http://subsonic.org/restapi subsonic-response" json:"-"`
	Status  string   `xml:"status,attr" json:"status"`
	Version string   `xml:"version,attr" json:"version"`
}
type subsonicError struct {
	subsonicResponse
	Error struct {
		Code int    `xml:"code,attr" json:"code"`
		Msg  string `xml:"message,attr" json:"message"`
	} `xml:"error" json:"error"`
}

func subOK() subsonicResponse {
	return subsonicResponse{
		Status:  "ok",
		Version: subsonicAPIVersion,
	}
}

func subErr(code int, msg string) subsonicError {
	resp := subsonicError{
		subsonicResponse: subsonicResponse{
			Status:  "failed",
			Version: subsonicAPIVersion,
		},
	}
	resp.Error.Code = code
	resp.Error.Msg = msg
	return resp
}

type subsonicUserFolder struct {
	XMLName xml.Name `xml:"folder" json:"-"`
	ID      int      `xml:",chardata" json:"value"`
}

type subsonicUser struct {
	XMLName           xml.Name `xml:"user" json:"-"`
	Username          string   `xml:"username,attr" json:"username"`
	Email             string   `xml:"email,attr" json:"email"`
	ScrobblingEnabled bool     `xml:"scrobblingEnabled,attr" json:"scrobblingEnabled"`
	AdminRole         bool     `xml:"adminRole,attr" json:"adminRole"`
	SettingsRole      bool     `xml:"settingsRole,attr" json:"settingsRole"`
	DownloadRole      bool     `xml:"downloadRole,attr" json:"downloadRole"`
	UploadRole        bool     `xml:"uploadRole,attr" json:"uploadRole"`
	PlaylistRole      bool     `xml:"playlistRole,attr" json:"playlistRole"`
	CoverArtRole      bool     `xml:"coverArtRole,attr" json:"coverArtRole"`
	CommentRole       bool     `xml:"commentRole,attr" json:"commentRole"`
	PodcastRole       bool     `xml:"podcastRole,attr" json:"podcastRole"`
	StreamRole        bool     `xml:"streamRole,attr" json:"streamRole"`
	JukeboxRole       bool     `xml:"jukeboxRole,attr" json:"jukeboxRole"`
	ShareRole         bool     `xml:"shareRole,attr" json:"shareRole"`

	Folders []subsonicUserFolder `json:"folder,omitempty"`
}

func subsonicPanicHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	ex := kami.Exception(ctx)
	var msg string
	var code int
	if err, ok := ex.(error); ok {
		msg = err.Error()
		switch err {
		case tube.ErrNotFound:
			code = 70 // The requested data was not found.
		case strconv.ErrSyntax:
			code = 10 // Required parameter is missing.
		}
	} else {
		msg = fmt.Sprint(ex)
	}

	resp := subErr(code, msg)
	writeSubsonic(ctx, w, r, resp)
}

func subsonicPing(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	resp := subOK()
	writeSubsonic(ctx, w, r, resp)
}

func subsonicGetUser(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)

	name := r.FormValue("username")
	// hmm
	if name != "" && name != u.Email {
		writeSubsonic(ctx, w, r, subErr(50, "User is not authorized for the given operation."))
		return
	}

	type userResponse struct {
		subsonicResponse
		User subsonicUser `json:"user"`
	}

	resp := userResponse{
		subsonicResponse: subOK(),

		User: subsonicUser{
			Username:          u.Email,
			Email:             u.Email,
			ScrobblingEnabled: true,
			AdminRole:         false,
			SettingsRole:      false,
			DownloadRole:      true,
			UploadRole:        true,
			PlaylistRole:      true,
			CoverArtRole:      true,
			CommentRole:       false,
			PodcastRole:       false,
			StreamRole:        true,
			JukeboxRole:       false,
			ShareRole:         false,

			Folders: []subsonicUserFolder{{ID: 1}}, // TODO
		},
	}

	writeSubsonic(ctx, w, r, resp)
}

func subsonicAuth(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	if DebugMode {
		out, _ := httputil.DumpRequest(r, true)
		fmt.Println(string(out))
	}

	u := r.FormValue("u")
	u = strings.ReplaceAll(u, " ", "+")
	p := r.FormValue("p")
	f := r.FormValue("f")

	if f != "" {
		ctx = withFormat(ctx, f)
	}

	// special case: ping without auth
	if r.URL.Path == "/rest/ping.view" || r.URL.Path == "/rest/ping" {
		return ctx
	}
	// return valid license if u/p missing
	// some clients need this... (dsub?)
	if u == "" && p == "" && r.URL.Path == "/rest/getLicense.view" {
		return ctx
	}

	if u == "" || p == "" {
		writeSubsonic(ctx, w, r, subErr(10, "missing u/p"))
		return nil
	}

	// TODO: use subsonic token or something
	user, err := tube.GetUserByEmail(ctx, u)
	if err != nil {
		writeSubsonic(ctx, w, r, subErr(40, "Wrong username or password"))
		return nil
	}

	// http://your-server/rest/ping.view?u=joe&p=sesame&v=1.12.0&c=myapp
	// http://your-server/rest/ping.view?u=joe&p=enc:736573616d65&v=1.12.0&c=myapp
	if strings.HasPrefix(p, "enc:") {
		enc := strings.TrimPrefix(p, "enc:")
		pw, err := hex.DecodeString(enc)
		if err != nil {
			panic(err)
		}
		p = string(pw)
	}

	if !user.ValidPassword(p) {
		writeSubsonic(ctx, w, r, subErr(40, "Wrong username or password"))
		return nil
	}

	ctx = withUser(ctx, user)
	return ctx
}

type subsonicLicense struct {
	subsonicResponse
	// <license valid="true" email="foo@bar.com" licenseExpires="2019-09-03T14:46:43"/>
	License struct {
		Valid   bool   `xml:"valid,attr" json:"valid"`
		Expires string `xml:"licenseExpires,attr" json:"licenseExpires"`
		Email   string `xml:"email,attr,omitempty" json:"email,omitempty"`
	} `xml:"license" json:"license"`
}

func subsonicGetLicense(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(ctx)
	expire := time.Now().UTC().Add(24 * time.Hour * 30)
	resp := subsonicLicense{
		subsonicResponse: subOK(),
	}
	resp.License.Valid = expire.After(time.Now())
	resp.License.Expires = expire.Format(subsonicTimeLayout)
	resp.License.Email = u.Email
	writeSubsonic(ctx, w, r, resp)
}

func subsonicSavePlayQueue(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// TODO: unstub
	writeSubsonic(ctx, w, r, subOK())
}

func subsonicGetPlayQueue(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// TODO: unstub
	writeSubsonic(ctx, w, r, subOK())
}

func subsonicGetNowPlaying(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// TODO: unstub
	writeSubsonic(ctx, w, r, subOK())
}

func writeSubsonic(ctx context.Context, w http.ResponseWriter, r *http.Request, resp any) {
	f := formatFrom(ctx)
	switch f {
	case "json":
		w.Header().Set("Content-Type", "application/json")

		wrap := struct {
			Resp any `json:"subsonic-response"`
		}{
			Resp: resp,
		}

		if DebugMode {
			fmt.Println("\n>>>>>>>>>>>>>>> "+r.URL.EscapedPath(), " ? ", r.URL.Query().Encode())
			raw, err := json.Marshal(wrap)
			fmt.Println("=================\n"+string(raw)+"\n~~~~~~~err", err, "~~~~~~~")
		}

		renderJSON(w, wrap, http.StatusOK)
	case "jsonp":
		w.Header().Set("Content-Type", "application/javascript")
		cb := r.FormValue("callback")
		if cb == "" {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"subsonic-response":{"status":"failed","version":"1.12.0","error":{"code":10,"message":"missing callback param"}}}`)
			return
		}

		wrap := struct {
			Resp any `json:"subsonic-response"`
		}{
			Resp: resp,
		}

		fmt.Fprintf(w, "%s(", cb)
		js, err := json.Marshal(wrap)
		if err != nil {
			panic(err)
		}
		fmt.Fprint(w, string(js))
		fmt.Fprint(w, ");")
	case "xml":
		if DebugMode {
			raw, err := xml.MarshalIndent(resp, "  ", "	")
			fmt.Println("\n>>>>>>>>>>>>>>> "+r.URL.EscapedPath(), " ? ", r.URL.Query().Encode())
			fmt.Println("=================\n"+string(raw)+"\n~~~~~~~~ err:", err, "~~~~")
		}

		renderXML(w, resp, http.StatusOK)

		if err := xml.NewEncoder(w).Encode(resp); err != nil {
			panic(err)
		}
	default:
		panic("unknown format: " + f)
	}
}

func isSubsonicReq(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, subsonicAPIPrefix)
}

func sec2msec(sec float64) int {
	return int(sec * 1000)
}

type formatkey struct{}

func withFormat(ctx context.Context, f string) context.Context {
	return context.WithValue(ctx, formatkey{}, f)
}

func formatFrom(ctx context.Context) string {
	f, _ := ctx.Value(formatkey{}).(string)
	if f == "" {
		return "xml"
	}
	return f
}
