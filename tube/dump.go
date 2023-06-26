package tube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/karlseguin/ccache/v2"

	"github.com/guregu/dynamo"
	"github.com/guregu/intertube/storage"
)

const (
	dumpVer     = 1
	dumpPathFmt = "dump/v%d/%d.db"
)

var dumpCache = ccache.New(ccache.Configure())

var errStaleDump = fmt.Errorf("stale dump")

type Dump struct {
	UserID int
	Time   time.Time
	Tracks []Track
}

func (d Dump) Key() string {
	return fmt.Sprintf(dumpPathFmt, dumpVer, d.UserID)
}

func (d Dump) save(ctx context.Context) error {
	// TODO: should probably create new file instead of overwriting
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(d); err != nil {
		return err
	}
	r := bytes.NewReader(buf.Bytes())
	return storage.CacheBucket.Put("application/json", d.Key(), r)
}

func (d Dump) encache() {
	log.Println("dump encache", d.Key())
	dumpCache.Set(d.Key(), d, 5*time.Minute)
}

func (d Dump) stale(usertime time.Time) bool {
	return d.Time.Truncate(time.Millisecond).Before(usertime.Truncate(time.Millisecond))
}

func (d *Dump) sort() {
	sort.Slice(d.Tracks, func(i, j int) bool {
		return d.Tracks[i].SortID < d.Tracks[j].SortID
	})
}

func (d *Dump) Splice(tracks []Track) {
	update := make(map[string]Track, len(tracks))
	for _, t := range tracks {
		if t.LastMod.After(update[t.ID].LastMod) {
			update[t.ID] = t
		}
	}
	// update existing
	for i, t := range d.Tracks {
		up, ok := update[t.ID]
		if !ok {
			continue
		}
		d.Tracks[i] = up
		delete(update, t.ID)
		log.Println("spliced", d.UserID, up)
	}
	// new tracks
	for _, t := range update {
		d.Tracks = append(d.Tracks, t)
		d.sort()
		log.Println("added", d.UserID, t)
	}
}

func (d *Dump) Remove(trackID string) {
	tracks := d.Tracks[:0]
	for _, t := range d.Tracks {
		if t.ID != trackID {
			tracks = append(tracks, t)
			log.Println("removed", d.UserID, trackID)
		}
	}
	d.Tracks = tracks
}

func RefreshDump(ctx context.Context, userID int, at time.Time, updates []Track, deletes []string) error {
	u, err := GetUser(ctx, userID)
	if err != nil {
		return err
	}

	d, err := u.GetDump()
	if err != nil {
		log.Println("RefreshDump GetDump error:", err)
		return RecreateDump(ctx, userID, at)
	}

	d.Splice(updates)
	for _, del := range deletes {
		d.Remove(del)
	}
	d.Time = at
	return u.SaveDump(ctx, d)
}

func RecreateDump(ctx context.Context, userID int, at time.Time) error {
	u, err := GetUser(ctx, userID)
	if err != nil {
		return err
	}

	// TODO: need to check if it's an actual unexpected error or just a new dump...
	tracks, err := GetTracks(ctx, userID)
	// tracks, _, err := GetTracksPartialSorted(ctx, userID, 0, nil)
	if err != nil && err != ErrNotFound {
		return err
	}
	d := Dump{
		UserID: u.ID,
		Time:   at.UTC(),
		Tracks: tracks,
	}

	return u.SaveDump(ctx, d)
}

func (u User) SaveDump(ctx context.Context, d Dump) error {
	// only save if we 'win' the race
	err := u.UpdateLastDump(ctx, d.Time)
	if dynamo.IsCondCheckFailed(err) {
		log.Println("dump is stale:", err)
		// stale
		return nil
	}
	if err != nil {
		return err
	}
	// kewl
	log.Println("saving new dump...", d.UserID, d.Time, len(d.Tracks))
	return d.save(ctx)
}

func (u User) GetDump() (Dump, error) {
	key := fmt.Sprintf(dumpPathFmt, dumpVer, u.ID)

	// try cache
	if item := dumpCache.Get(key); item != nil {
		d := item.Value().(Dump)
		if d.stale(u.LastDump) {
			log.Println("stale dumppp")
			dumpCache.Delete(key)
		} else {
			log.Println("got dump from cache", u.ID)
			return item.Value().(Dump), nil
		}
	}

	// try s3
	d, err := loadDump(key)
	if err != nil {
		return Dump{}, err
	}
	if d.stale(u.LastDump) {
		return Dump{}, errStaleDump
	}
	return d, nil
}

// func GetDump(userID int) (Dump, error) {
// 	key := fmt.Sprintf(dumpPathFmt, dumpVer, userID)
// 	if item := dumpCache.Get(key); item != nil {
// 		log.Println("got dump from cache", userID)
// 		return item.Value().(Dump), nil
// 	}
// 	return loadDump(key)
// }

func loadDump(key string) (Dump, error) {
	r, err := storage.CacheBucket.Get(key)
	if err != nil {
		return Dump{}, err
	}
	defer r.Close()
	var d Dump
	if err := json.NewDecoder(r).Decode(&d); err != nil {
		return d, err
	}

	// TODO: compare timestamp?
	// TODO: TODO
	// d.encache()

	return d, nil
}
