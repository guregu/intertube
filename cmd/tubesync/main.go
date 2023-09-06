package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"golang.org/x/net/publicsuffix"
	"golang.org/x/term"

	"github.com/guregu/intertube/tube"
)

const (
	version = "0.1.0"
)

var client *http.Client
var rootPath string

var (
	parallel = flag.Int("parallel", 10, "number of simultaneous downloads")
	host     = flag.String("host", "https://inter.tube", "host URL, change for custom deployments")
	workDir  = flag.String("path", "", "path to music library directory, leave blank (default) for current directory")
	help     = flag.Bool("help", false, "show this help message")
)

var ErrSkip = fmt.Errorf("skipped")

func main() {
	flag.Parse()
	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *workDir == "" {
		exe, err := os.Executable()
		maybeDie(err)
		rootPath = filepath.Dir(exe)
	} else {
		rootPath = *workDir
	}

	fmt.Println("welcome to tubesync version", version)
	fmt.Println("   for", *host)
	fmt.Println("working directory:", rootPath)
	var email, pass string
	fmt.Print("email (blank to quit): ")
	fmt.Scanln(&email)
	if email == "" {
		fmt.Println("email is blank, bye")
		os.Exit(1)
	}
	// fmt.Println()
	fmt.Print("password (hidden): ")
	pwraw, err := term.ReadPassword(int(syscall.Stdin))
	maybeDie(err)
	pass = string(pwraw)
	fmt.Println("\nlogging in as", email, "...")

	err = login(email, pass)
	maybeDie(err)
	fmt.Println("login successful")

	fmt.Print("getting track metadata (might take a while)")
	tracks, err := getTracks()
	fmt.Println()
	maybeDie(err)
	fmt.Println("got", len(tracks), "tracks")
	fmt.Println("syncing...")

	total := len(tracks)
	progress := new(int64)
	errct := new(int64)

	dlchan := make(chan tube.Track, *parallel)
	var wg sync.WaitGroup
	for n := 0; n < *parallel; n++ {
		wg.Add(1)
		go func() {
			for track := range dlchan {
				var msg string
				name := displayName(track)
				if err := download(track); err != nil {
					if err == ErrSkip {
						msg = fmt.Sprint("skipped: ", name, " (already downloaded)")
					} else {
						msg = fmt.Sprint("download error: ", track.ID, name, " ", err)
						atomic.AddInt64(errct, 1)
					}
				} else {
					msg = "downloaded: " + name
				}
				prog := atomic.AddInt64(progress, 1)
				fmt.Printf("[%d/%d] %s\n", prog, total, msg)
			}
			wg.Done()
		}()
	}

	for _, track := range tracks {
		dlchan <- track
	}
	close(dlchan)
	wg.Wait()

	fmt.Println("done~")
	fmt.Printf("got %d error(s)\n", atomic.LoadInt64(errct))
	fmt.Println("\nPress ENTER to exit")
	fmt.Scanln()

	os.Exit(0)
}

func login(email, pass string) error {
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return err
	}
	client = &http.Client{
		Jar: jar,
	}

	req := struct {
		Email    string
		Password string
	}{
		Email:    email,
		Password: pass,
	}
	resp := struct {
		Session string
	}{}

	return post("/api/v0/login", req, &resp)
}

func getTracks() (tube.Tracks, error) {
	var tracks tube.Tracks
	var resp struct {
		Tracks tube.Tracks
		Next   string
	}
	var err error
	for {
		err = get("/api/v0/tracks?start="+resp.Next, &resp)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, resp.Tracks...)
		if resp.Next == "" {
			break
		}
		fmt.Print(".")
	}
	return tracks, nil
}

func download(track tube.Track) error {
	href := track.FileURL()
	artist := track.Info.AlbumArtist
	if artist == "" {
		if track.Info.Artist != "" {
			artist = track.Info.Artist
		} else {
			artist = "Unknown Artist"
		}
	}
	album := track.Info.Album
	if album == "" {
		album = "Unknown Album"
	}
	title := track.Info.Title
	if title == "" {
		if track.Filename != "" {
			title = track.Filename
		} else {
			title = "Untitled"
		}
	}
	var num string
	if track.Disc != 0 && track.Number != 0 {
		num = fmt.Sprintf("%d-%02d ", track.Disc, track.Number)
	} else if track.Number != 0 {
		num = fmt.Sprintf("%02d ", track.Number)
	}
	filename := scrub(num + title + track.Ext())

	dir := filepath.Join(rootPath, scrub(artist), scrub(album))
	os.MkdirAll(dir, os.ModePerm)
	fpath := filepath.Join(dir, filename)

	if ok, err := shouldSkip(fpath, track.ID); ok && err == nil {
		return ErrSkip
	} else if err != nil {
		return err
	}

	dlURL := track.DL
	if dlURL == "" {
		dlURL = *host + href
	}
	resp, err := client.Get(dlURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func post(path string, in interface{}, out interface{}) error {
	href := *host + path

	var inRdr io.Reader
	if in != nil {
		inraw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		inRdr = bytes.NewReader(inraw)
	}

	resp, err := client.Post(href, "application/json", inRdr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// fmt.Println("got:", string(body))

	if resp.StatusCode == 500 {
		return fmt.Errorf("%s", strings.TrimPrefix(string(body), "Panic! "))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return err
		}
	}
	return nil
}

func get(path string, out interface{}) error {
	href := *host + path

	resp, err := client.Get(href)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// fmt.Println("got:", string(body))

	if resp.StatusCode == 500 {
		return fmt.Errorf("%s", strings.TrimPrefix(string(body), "Panic! "))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return err
		}
	}
	return nil
}

func shouldSkip(path, hash string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	sum, err := sha1Sum(f)
	if err != nil {
		return false, err
	}
	if sum == hash {
		return true, nil
	}
	return false, nil
}

func sha1Sum(r io.ReadSeeker) (string, error) {
	h := sha1.New()
	_, err := io.Copy(h, r)
	if err != nil {
		return "", nil
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func scrub(name string) string {
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

func displayName(track tube.Track) string {
	return fmt.Sprintf("%s - %s - %s", track.Info.Artist, track.Info.Album, track.Info.Title)
}

func maybeDie(err error) {
	if err == nil {
		return
	}
	fmt.Println("error:", err.Error())
	os.Exit(1)
}
