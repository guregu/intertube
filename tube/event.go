package tube

import (
	"bytes"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

type Event struct {
	UserID int
	Time   Timegarb
	Kind   EventKind
}

type EventKind string

const (
	EventPaid EventKind = "paid"
)

type Timegarb struct {
	time.Time
	Garb string
}

func NewTimegarb(t time.Time) Timegarb {
	garb := strconv.FormatUint(rand.Uint64(), 36)
	return Timegarb{
		Time: t.UTC(),
		Garb: garb,
	}
}

func (t Timegarb) MarshalText() ([]byte, error) {
	text, err := t.Time.MarshalText()
	if err != nil {
		return nil, err
	}
	text = append(text, []byte(" "+t.Garb)...)
	return text, nil
}

func (t *Timegarb) UnmarshalText(text []byte) error {
	idx := bytes.IndexRune(text, ' ')
	if idx < 0 || idx == len(text) {
		return fmt.Errorf("invalid timegarb: %s", string(text))
	}
	if err := t.Time.UnmarshalText(text[:idx]); err != nil {
		return err
	}
	t.Garb = string(text[idx+1:])
	return nil
}
