package web

import (
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/guregu/intertube/tube"
)

func compile(code string) (*vm.Program, error) {
	options := []expr.Option{
		expr.Env(ExprEnv{}),

		// Operators override for date comprising.
		expr.Operator("==", "Equal"),
		expr.Operator("<", "Before"),
		expr.Operator("<=", "BeforeOrEqual"),
		expr.Operator(">", "After"),
		expr.Operator(">=", "AfterOrEqual"),

		// Time and duration manipulation.
		expr.Operator("+", "Add"),
		expr.Operator("-", "Sub"),

		// Operators override for duration comprising.
		expr.Operator("==", "EqualDuration"),
		expr.Operator("<", "BeforeDuration"),
		expr.Operator("<=", "BeforeOrEqualDuration"),
		expr.Operator(">", "AfterDuration"),
		expr.Operator(">=", "AfterOrEqualDuration"),
	}

	return expr.Compile(code, options...)
}

type ExprEnv struct {
	datetime

	ID         string
	SSID       tube.SSID
	ArtistSSID tube.SSID

	Number int
	Total  int
	Disc   int
	Discs  int
	Year   int

	Filename string
	Filetype string
	Size     int
	Duration int

	Plays    int
	LastPlay time.Time
	Resume   float64

	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Composer    string
	Genre       string
	Comment     string
}

func NewExprEnv(t tube.Track) ExprEnv {
	return ExprEnv{
		ID:          t.ID,
		SSID:        t.TrackSSID(),
		ArtistSSID:  t.ArtistSSID(),
		Number:      t.Number,
		Total:       t.Total,
		Disc:        t.Disc,
		Discs:       t.Discs,
		Year:        t.Year,
		Filename:    t.Filename,
		Filetype:    t.Filetype,
		Size:        t.Size,
		Duration:    t.Duration,
		Plays:       t.Plays,
		LastPlay:    t.LastPlayed,
		Resume:      t.Resume,
		Title:       t.Info.Title,
		Artist:      t.Info.Artist,
		Album:       t.Info.Album,
		AlbumArtist: t.Info.AlbumArtist,
		Composer:    t.Info.Composer,
		Genre:       t.Info.Genre,
		Comment:     t.Info.Comment,
	}
}

// Taken from https://github.com/antonmedv/expr/blob/master/docs/examples/dates_test.go
// MIT license
// https://github.com/antonmedv/expr/blob/master/LICENSE

type datetime struct{}

func (datetime) Date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func (datetime) Duration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (datetime) Days(n int) time.Duration {
	return time.Hour * 24 * time.Duration(n)
}

func (datetime) Now() time.Time                                { return time.Now() }
func (datetime) Equal(a, b time.Time) bool                     { return a.Equal(b) }
func (datetime) Before(a, b time.Time) bool                    { return a.Before(b) }
func (datetime) BeforeOrEqual(a, b time.Time) bool             { return a.Before(b) || a.Equal(b) }
func (datetime) After(a, b time.Time) bool                     { return a.After(b) }
func (datetime) AfterOrEqual(a, b time.Time) bool              { return a.After(b) || a.Equal(b) }
func (datetime) Add(a time.Time, b time.Duration) time.Time    { return a.Add(b) }
func (datetime) Sub(a, b time.Time) time.Duration              { return a.Sub(b) }
func (datetime) EqualDuration(a, b time.Duration) bool         { return a == b }
func (datetime) BeforeDuration(a, b time.Duration) bool        { return a < b }
func (datetime) BeforeOrEqualDuration(a, b time.Duration) bool { return a <= b }
func (datetime) AfterDuration(a, b time.Duration) bool         { return a > b }
func (datetime) AfterOrEqualDuration(a, b time.Duration) bool  { return a >= b }
