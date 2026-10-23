package api

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// TestParseRange pins the date rules the reader owns: the default week follows
// the caller's local calendar, and a bad range is a 422 before any query runs.
func TestParseRange(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		tz         string
		now        time.Time
		from, to   string
		wantFrom   string
		wantTo     string
		wantStatus int
	}{
		{name: "explicit dates", tz: "UTC", now: now, from: "2026-10-19", to: "2026-10-26", wantFrom: "2026-10-19", wantTo: "2026-10-26"},
		{name: "default week", tz: "UTC", now: now, wantFrom: "2026-10-19", wantTo: "2026-10-26"},
		{
			// Sunday 23:00Z is Monday 13:00 in UTC+14, so the local week just turned.
			name: "default week follows the local calendar", tz: "Pacific/Kiritimati",
			now:      time.Date(2026, 10, 25, 23, 0, 0, 0, time.UTC),
			wantFrom: "2026-10-26", wantTo: "2026-11-02",
		},
		{name: "from alone", tz: "UTC", now: now, from: "2026-10-19", wantStatus: http.StatusUnprocessableEntity},
		{name: "to not after from", tz: "UTC", now: now, from: "2026-10-26", to: "2026-10-26", wantStatus: http.StatusUnprocessableEntity},
		{name: "longer than a year", tz: "UTC", now: now, from: "2025-01-01", to: "2026-01-03", wantStatus: http.StatusUnprocessableEntity},
		{name: "bad date", tz: "UTC", now: now, from: "19-10-2026", to: "2026-10-26", wantStatus: http.StatusUnprocessableEntity},
		{name: "stored timezone is unknown", tz: "Mars/Olympus", now: now, wantStatus: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rng, err := parseRange(tc.tz, tc.now, tc.from, tc.to)
			if tc.wantStatus != 0 {
				var p *huma.ErrorModel
				if !errors.As(err, &p) || p.Status != tc.wantStatus {
					t.Fatalf("err = %v, want status %d", err, tc.wantStatus)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRange: %v", err)
			}
			if got := rng.From.Format("2006-01-02"); got != tc.wantFrom {
				t.Errorf("from = %s, want %s", got, tc.wantFrom)
			}
			if got := rng.To.Format("2006-01-02"); got != tc.wantTo {
				t.Errorf("to = %s, want %s", got, tc.wantTo)
			}
		})
	}
}
