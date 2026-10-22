package entry

import (
	"errors"
	"testing"
	"time"
)

// now is fixed; tests never sleep and never call time.Now.
var now = time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		start, end time.Time
		want       error
	}{
		{"valid 2h entry", now.Add(-3 * time.Hour), now.Add(-1 * time.Hour), nil},
		{"exactly 24h allowed", now.Add(-25 * time.Hour), now.Add(-1 * time.Hour), nil},
		{"24h+1s rejected", now.Add(-25 * time.Hour), now.Add(-1*time.Hour + time.Second), ErrTooLong},
		{"end == start rejected", now.Add(-time.Hour), now.Add(-time.Hour), ErrInvalidRange},
		{"end < start rejected", now.Add(-time.Hour), now.Add(-2 * time.Hour), ErrInvalidRange},
		{"start 4m59s in future allowed", now.Add(4*time.Minute + 59*time.Second), now.Add(5 * time.Minute), nil},
		{"start 5m01s in future rejected", now.Add(5*time.Minute + time.Second), now.Add(5*time.Minute + 2*time.Second), ErrFuture},
		{"end in the future rejected", now.Add(-time.Hour), now.Add(6 * time.Minute), ErrFuture},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.start, tt.end, now); !errors.Is(err, tt.want) {
				t.Errorf("Validate(%v, %v) = %v, want %v", tt.start, tt.end, err, tt.want)
			}
		})
	}
}

func TestValidateStop(t *testing.T) {
	tests := []struct {
		name      string
		start, at time.Time
		want      error
	}{
		{"valid 2h entry", now.Add(-2 * time.Hour), now, nil},
		{"at exactly 24h after start allowed", now.Add(-24 * time.Hour), now, nil},
		{"24h+1s rejected", now.Add(-24*time.Hour - time.Second), now, ErrTooLong},
		{"at == start rejected", now.Add(-time.Hour), now.Add(-time.Hour), ErrInvalidRange},
		{"at < start rejected", now.Add(-time.Hour), now.Add(-2 * time.Hour), ErrInvalidRange},
		{"start 4m59s in future allowed", now.Add(4*time.Minute + 59*time.Second), now.Add(5 * time.Minute), nil},
		{"start 5m01s in future rejected", now.Add(5*time.Minute + time.Second), now.Add(5*time.Minute + 2*time.Second), ErrFuture},
		{"stop instant in the future rejected", now.Add(-time.Hour), now.Add(6 * time.Minute), ErrFuture},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateStop(tt.start, tt.at, now); !errors.Is(err, tt.want) {
				t.Errorf("ValidateStop(%v, %v) = %v, want %v", tt.start, tt.at, err, tt.want)
			}
		})
	}
}
