// Package entry owns the entry and timer rules and the write-side lifecycle
// that applies them; "now" is passed in, so tests never sleep.
package entry

import (
	"errors"
	"time"
)

var (
	ErrInvalidRange = errors.New("end must be after start")
	ErrTooLong      = errors.New("entry is longer than 24 hours")
	ErrFuture       = errors.New("entry is too far in the future")
)

const MaxLength = 24 * time.Hour
const FutureGrace = 5 * time.Minute

// Validate checks a finished entry's start and end against now.
func Validate(start, end, now time.Time) error {
	if !end.After(start) {
		return ErrInvalidRange
	}
	if end.Sub(start) > MaxLength {
		return ErrTooLong
	}
	if start.After(now.Add(FutureGrace)) || end.After(now.Add(FutureGrace)) {
		return ErrFuture
	}
	return nil
}
