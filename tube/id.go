package tube

import (
	"fmt"
	"strconv"
	"strings"
)

var MusicFolder = SSID{Kind: SSIDFolder, ID: "1"}

type SSID struct {
	Kind SSIDKind
	ID   string
}

type SSIDKind rune

const (
	SSIDArtist SSIDKind = 'A'
	SSIDAlbum  SSIDKind = 'a'
	SSIDTrack  SSIDKind = 't'
	// SSIDPlaylist SSIDKind = 'P'
	SSIDFolder  SSIDKind = 'F'
	SSIDInvalid SSIDKind = -1
)

func (s SSID) MarshalText() ([]byte, error) {
	if s.Kind == SSIDInvalid {
		return nil, fmt.Errorf("invalid ssid: %s", s.String())
	}
	return []byte(s.String()), nil
}

func (s *SSID) UnmarshalText(text []byte) error {
	*s = ParseSSID(string(text))
	return nil
}

func (s SSID) String() string {
	if s.Kind == SSIDFolder {
		return s.ID
	}
	return s.Kind.String() + "-" + s.ID
}

func (s SSID) IsZero() bool {
	return s == SSID{}
}

func (k SSIDKind) String() string {
	if k == SSIDInvalid {
		return "~INVALID~"
	}
	return string(k)
}

func NewSSID(kind SSIDKind, id string) SSID {
	return SSID{
		Kind: kind,
		ID:   id,
	}
}

func ParseSSID(id string) SSID {
	if len(id) == 0 {
		return SSID{}
	}
	id = strings.Replace(id, "!", "-", 1)
	if !strings.ContainsRune(id, '-') {
		// special case: folders are integers...
		if _, err := strconv.Atoi(id); err == nil {
			return SSID{Kind: SSIDFolder, ID: id}
		}
	}
	if len(id) < 3 || id[1] != '-' {
		return SSID{Kind: SSIDInvalid, ID: ""}
	}
	rest := id[2:]
	switch SSIDKind(id[0]) {
	case SSIDArtist:
		return SSID{Kind: SSIDArtist, ID: rest}
	case SSIDAlbum:
		return SSID{Kind: SSIDAlbum, ID: rest}
	case SSIDTrack:
		return SSID{Kind: SSIDTrack, ID: rest}
		// case SSIDPlaylist:
		// 	return SSID{Kind: SSIDPlaylist, ID: rest}
	}
	return SSID{Kind: SSIDInvalid, ID: id}
}
