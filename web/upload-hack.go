package web

import (
	"strings"
	"unicode/utf8"

	"github.com/guregu/tag"
)

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
