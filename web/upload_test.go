package web

import (
	"reflect"
	"testing"

	"os"
	// "io"
)

var testcases = []struct {
	name   string
	expect guessedMeta
}{
	{"blah.mp3", guessedMeta{title: "blah"}},
	{"Artist - Album - 01 Track.mp3", guessedMeta{artist: "Artist", album: "Album", track: 1, title: "Track"}},
	{"2-03 songtitle.mp3", guessedMeta{disc: 2, track: 3, title: "songtitle"}},
	{"1-abc.mp3", guessedMeta{track: 1, title: "abc"}},
	{"01-abc.mp3", guessedMeta{track: 1, title: "abc"}},
	{"Bulldada - What a Bunch of Bulldada - 09 22nd Century Yahoo Answers Man.mp3", guessedMeta{artist: "Bulldada", album: "What a Bunch of Bulldada", track: 9, title: "22nd Century Yahoo Answers Man"}},
	{"YTMND Soundtrack - Volume 6 - 14 - DVDA -  America Fuck Yeah.mp3", guessedMeta{albumArtist: "YTMND Soundtrack", artist: "DVDA", album: "Volume 6", track: 14, title: "America Fuck Yeah"}},
}

func TestMetadataGuess(t *testing.T) {
	for _, testcase := range testcases {
		meta := guessMetadata(testcase.name, "MP3")
		expect := testcase.expect
		expect.ftype = "MP3"
		if !reflect.DeepEqual(meta, expect) {
			t.Error(meta, "!=", expect)
		}
	}
}

func TestDuration(t *testing.T) {
	t.Skip("local only for now")

	f, err := os.Open("C:\\code\\intertube\\test.flac")
	if err != nil {
		t.Fatal(err)
	}
	dur, err := calcDuration(f, "FLAC")
	t.Error(dur, err)
}
