package web

import (
	"testing"

	"os"
)

func TestDuration(t *testing.T) {
	t.Skip("local only for now")

	f, err := os.Open("C:\\code\\intertube\\test.flac")
	if err != nil {
		t.Fatal(err)
	}
	dur, err := calcDuration(f, "FLAC")
	t.Error(dur, err)
}
