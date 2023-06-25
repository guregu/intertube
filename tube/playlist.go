package tube

import (
	"context"
	"fmt"
	"time"

	"github.com/guregu/dynamo"
)

type Playlist struct {
	UserID int `dynamo:",hash"`
	ID     int `dynamo:",range"`

	Date time.Time
	Name string
	Desc string

	Tracks   []string
	Duration int // seconds

	Dynamic bool
	Query   string
	Sort    []string
	UIMeta  []byte

	LastMod time.Time
}

// type PlaylistEntry struct {
// 	Ref     string
// 	TrackID string
// }

func (p *Playlist) Create(ctx context.Context) error {
	if p.ID != 0 {
		return fmt.Errorf("already exists: %d", p.ID)
	}

	id, err := NextID(ctx, "Playlists")
	if err != nil {
		return err
	}
	p.ID = id
	p.Date = time.Now().UTC()
	p.LastMod = p.Date

	table := dynamoTable("Playlists")
	return table.Put(p).If("attribute_not_exists('ID')").Run()
}

func (p *Playlist) Save(ctx context.Context) error {
	p.LastMod = time.Now().UTC()
	table := dynamoTable("Playlists")
	return table.Put(p).Run()
}

func (p *Playlist) With(tracks []Track) {
	p.Tracks = make([]string, 0, len(tracks))
	p.Duration = 0
	for _, t := range tracks {
		p.Duration += t.Duration
		p.Tracks = append(p.Tracks, t.ID)
	}
}

// func (p Playlist) SSID() SSID {
// 	return SSID{Kind: SSIDPlaylist, ID: fmt.Sprintf("%d.%d", p.UserID, p.ID)}
// }

// func parsePlaylistID(id string) (userID, pid int, err error) {
// 	println(id)
// 	split := strings.Split(id, ".")
// 	if len(split) != 2 {
// 		return 0, 0, fmt.Errorf("invalid playlist id")
// 	}
// 	userID, err = strconv.Atoi(split[0])
// 	if err != nil {
// 		return
// 	}
// 	pid, err = strconv.Atoi(split[1])
// 	return
// }

func GetPlaylist(ctx context.Context, userID int, id int) (Playlist, error) {
	var p Playlist
	table := dynamoTable("Playlists")
	err := table.Get("UserID", userID).Range("ID", dynamo.Equal, id).One(&p)
	return p, err
}

// func GetPlaylistBySSID(ctx context.Context, userID int, ssid SSID) (Playlist, error) {
// 	_, pid, err := parsePlaylistID(ssid.ID)
// 	if err != nil {
// 		return Playlist{}, err
// 	}
// 	return GetPlaylist(ctx, userID, pid)
// }

func GetPlaylists(ctx context.Context, userID int) ([]Playlist, error) {
	var pp []Playlist
	table := dynamoTable("Playlists")
	err := table.Get("UserID", userID).All(&pp)
	if err == ErrNotFound {
		err = nil
	}
	return pp, err
}

func DeletePlaylist(ctx context.Context, userID int, id int) error {
	table := dynamoTable("Playlists")
	return table.Delete("UserID", userID).Range("ID", id).Run()
}

// func DeletePlaylistBySSID(ctx context.Context, userID int, ssid SSID) error {
// 	_, pid, err := parsePlaylistID(ssid.ID)
// 	if err != nil {
// 		return err
// 	}
// 	return DeletePlaylist(ctx, userID, pid)
// }
