package web

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/guregu/tag"
)

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

var id3v2to3 = map[string]string{
	"TT2": "TIT2",
	"TP1": "TPE1",
	"TAL": "TALB",
	"TP2": "TPE2",
	"TCM": "TCOM",
	"TYE": "TYER",
	"TRK": "TRCK",
	"TPA": "TPOS",
	"TCO": "TCON",
	// "PIC": "APIC",
	// "":    "USLT",
	// "COM": "COMM", // panics on *tag.Comm conversion
}

// unfuckID3 fixes the given metadata if it is ID3 format but with ID2 keys
// (yes, such terrible files actually exist)
func unfuckID3(metadata tag.Metadata) {
	if metadata.Format() != tag.ID3v2_3 {
		return
	}
	raw := metadata.Raw()
	for k, v := range raw {
		if k == "PIC\u0000" {
			pic, err := tag.ReadPICFrame(v.([]byte))
			if err == nil {
				raw["APIC"] = pic
			}
			continue
		}
		if len(k) == 4 && k[3] == 0 {
			if fixed, ok := id3v2to3[k[:3]]; ok {
				raw[fixed] = v
				delete(raw, k)
			}
		}
	}
}

var (
	_ tag.Metadata = guessedMeta{}
	_ tag.Metadata = multiMeta{}
)
