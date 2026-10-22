package report

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/havspect/project-times/internal/store"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func loadLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

// fixture builds a one-user, one-project, one-team Input so each test only
// supplies entries and a timezone.
func fixture(tz string, entries ...store.Entry) Input {
	return Input{
		Entries:  entries,
		Users:    []store.User{{ID: 1, Name: "Ada Lovelace", Email: "ada@example.com", Timezone: tz}},
		Projects: []store.Project{{ID: 1, TeamID: 1, Name: "Timeline"}},
		Teams:    []store.Team{{ID: 1, Name: "Platform"}},
	}
}

func mustSplit(t *testing.T, in Input, from, to time.Time) []Segment {
	t.Helper()
	segs, err := Split(in, from, to)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	return segs
}

func sumSeconds(segs []Segment) int64 {
	var n int64
	for _, s := range segs {
		n += s.Seconds
	}
	return n
}

// 1. A 12-hour entry crossing local midnight becomes two segments whose
// seconds still sum to exactly 12h, one local day apart.
func TestSplitCrossesLocalMidnight(t *testing.T) {
	tokyo := loadLoc(t, "Asia/Tokyo")
	start := time.Date(2026, 10, 19, 18, 0, 0, 0, tokyo)
	end := time.Date(2026, 10, 20, 6, 0, 0, 0, tokyo)
	in := fixture("Asia/Tokyo", store.Entry{
		ID: 1, UserID: 1, ProjectID: 1, StartAt: start, EndAt: &end, Note: "night shift",
	})

	segs := mustSplit(t, in, date(2026, 10, 19), date(2026, 10, 21))

	if len(segs) != 2 {
		t.Fatalf("got %d segments, want 2: %+v", len(segs), segs)
	}
	if got := sumSeconds(segs); got != 12*3600 {
		t.Errorf("seconds sum = %d, want %d", got, 12*3600)
	}
	if segs[0].Date != "2026-10-19" || segs[1].Date != "2026-10-20" {
		t.Errorf("dates = %q, %q; want 2026-10-19, 2026-10-20", segs[0].Date, segs[1].Date)
	}
	if segs[0].Seconds != 6*3600 || segs[1].Seconds != 6*3600 {
		t.Errorf("seconds = %d, %d; want 21600 each", segs[0].Seconds, segs[1].Seconds)
	}
}

// 2. DST spring forward in Europe/Berlin: the wall clock jumps 02:00 -> 03:00,
// so 01:30+01:00 to 03:30+02:00 is one hour of real time, not two.
func TestSplitDSTSpringForward(t *testing.T) {
	berlin := loadLoc(t, "Europe/Berlin")
	start := time.Date(2026, 3, 29, 1, 30, 0, 0, berlin)
	end := time.Date(2026, 3, 29, 3, 30, 0, 0, berlin)
	in := fixture("Europe/Berlin", store.Entry{
		ID: 1, UserID: 1, ProjectID: 1, StartAt: start, EndAt: &end,
	})

	segs := mustSplit(t, in, date(2026, 3, 29), date(2026, 3, 30))

	if len(segs) != 1 {
		t.Fatalf("got %d segments, want 1: %+v", len(segs), segs)
	}
	if segs[0].Seconds != 3600 {
		t.Errorf("seconds = %d, want 3600", segs[0].Seconds)
	}
	if segs[0].Date != "2026-03-29" {
		t.Errorf("date = %q, want 2026-03-29", segs[0].Date)
	}
}

// 3. DST fall back: 02:30 occurs twice, but the instants are one hour apart.
func TestSplitDSTFallBack(t *testing.T) {
	// Constructed in UTC to pick the two distinct instants unambiguously.
	start := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC) // 02:30+02:00 Berlin
	end := time.Date(2026, 10, 25, 1, 30, 0, 0, time.UTC)   // 02:30+01:00 Berlin
	in := fixture("Europe/Berlin", store.Entry{
		ID: 1, UserID: 1, ProjectID: 1, StartAt: start, EndAt: &end,
	})

	segs := mustSplit(t, in, date(2026, 10, 25), date(2026, 10, 26))

	if len(segs) != 1 {
		t.Fatalf("got %d segments, want 1: %+v", len(segs), segs)
	}
	if segs[0].Seconds != 3600 {
		t.Errorf("seconds = %d, want 3600", segs[0].Seconds)
	}
	if segs[0].Date != "2026-10-25" {
		t.Errorf("date = %q, want 2026-10-25", segs[0].Date)
	}
	if got := segs[0].Start.Format(time.RFC3339); got != "2026-10-25T02:30:00+02:00" {
		t.Errorf("start = %q, want 2026-10-25T02:30:00+02:00", got)
	}
	if got := segs[0].End.Format(time.RFC3339); got != "2026-10-25T02:30:00+01:00" {
		t.Errorf("end = %q, want 2026-10-25T02:30:00+01:00", got)
	}
}

// 4. ISO week year edge: 2027-01-01 falls in ISO week 2026-W53, whose week
// year differs from its calendar year.
func TestSplitISOWeekYearEdge(t *testing.T) {
	start := time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC)
	end := time.Date(2027, 1, 1, 10, 0, 0, 0, time.UTC)
	in := fixture("UTC", store.Entry{
		ID: 1, UserID: 1, ProjectID: 1, StartAt: start, EndAt: &end,
	})

	segs := mustSplit(t, in, date(2026, 12, 28), date(2027, 1, 4))

	if len(segs) != 1 {
		t.Fatalf("got %d segments, want 1: %+v", len(segs), segs)
	}
	if segs[0].Date != "2027-01-01" {
		t.Errorf("date = %q, want 2027-01-01", segs[0].Date)
	}
	if segs[0].Week != "2026-W53" {
		t.Errorf("week = %q, want 2026-W53", segs[0].Week)
	}

	rows, total := Aggregate(segs, "week")
	if len(rows) != 1 || rows[0].Bucket != "2026-W53" {
		t.Errorf("week buckets = %+v, want one row in 2026-W53", rows)
	}
	if total != 3600 {
		t.Errorf("total = %d, want 3600", total)
	}
}

// 5. Range clipping: only the in-range local day counts, and a day equal to
// `to` is excluded.
func TestSplitClipsToRange(t *testing.T) {
	// Evening before `from`, spilling two hours into the first in-range day.
	before := time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)
	beforeEnd := time.Date(2026, 10, 19, 2, 0, 0, 0, time.UTC)
	// Starts exactly on `to`, must be excluded.
	onTo := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	onToEnd := time.Date(2026, 10, 21, 1, 0, 0, 0, time.UTC)
	in := fixture("UTC",
		store.Entry{ID: 1, UserID: 1, ProjectID: 1, StartAt: before, EndAt: &beforeEnd},
		store.Entry{ID: 2, UserID: 1, ProjectID: 1, StartAt: onTo, EndAt: &onToEnd},
	)

	segs := mustSplit(t, in, date(2026, 10, 19), date(2026, 10, 21))

	if len(segs) != 1 {
		t.Fatalf("got %d segments, want 1: %+v", len(segs), segs)
	}
	if segs[0].Date != "2026-10-19" {
		t.Errorf("date = %q, want 2026-10-19", segs[0].Date)
	}
	if segs[0].Seconds != 2*3600 {
		t.Errorf("seconds = %d, want %d", segs[0].Seconds, 2*3600)
	}
}

// 6. The same segments aggregate the same total under day, week and none,
// while the row counts differ.
func TestAggregateBuckets(t *testing.T) {
	// Two days in the same ISO week (2026-10-19 is a Monday).
	d1s := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	d1e := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	d2s := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	d2e := time.Date(2026, 10, 20, 11, 0, 0, 0, time.UTC)
	in := fixture("UTC",
		store.Entry{ID: 1, UserID: 1, ProjectID: 1, StartAt: d1s, EndAt: &d1e},
		store.Entry{ID: 2, UserID: 1, ProjectID: 1, StartAt: d2s, EndAt: &d2e},
	)

	segs := mustSplit(t, in, date(2026, 10, 19), date(2026, 10, 21))

	day, dayTotal := Aggregate(segs, "day")
	week, weekTotal := Aggregate(segs, "week")
	none, noneTotal := Aggregate(segs, "none")

	if dayTotal != weekTotal || weekTotal != noneTotal {
		t.Errorf("totals differ: day=%d week=%d none=%d", dayTotal, weekTotal, noneTotal)
	}
	if dayTotal != 3*3600 {
		t.Errorf("total = %d, want %d", dayTotal, 3*3600)
	}
	if len(day) != 2 {
		t.Errorf("day rows = %d, want 2", len(day))
	}
	if len(week) != 1 || len(none) != 1 {
		t.Errorf("week rows = %d, none rows = %d; want 1 each", len(week), len(none))
	}
	if day[0].Bucket != "2026-10-19" || day[1].Bucket != "2026-10-20" {
		t.Errorf("day buckets = %q, %q; want 2026-10-19, 2026-10-20", day[0].Bucket, day[1].Bucket)
	}
	if week[0].Bucket != "2026-W43" {
		t.Errorf("week bucket = %q, want 2026-W43", week[0].Bucket)
	}
	if none[0].Bucket != "" {
		t.Errorf("none bucket = %q, want empty", none[0].Bucket)
	}
}

// 7. CSV: header exact, offsets in the subject's timezone, and a note with a
// comma and a double quote round-trips through encoding/csv.
func TestWriteCSV(t *testing.T) {
	berlin := loadLoc(t, "Europe/Berlin")
	start := time.Date(2026, 10, 19, 9, 0, 0, 0, berlin)
	end := time.Date(2026, 10, 19, 11, 30, 0, 0, berlin)
	note := `drag, resize, "snap"`
	in := fixture("Europe/Berlin", store.Entry{
		ID: 1, UserID: 1, ProjectID: 1, StartAt: start, EndAt: &end, Note: note,
	})
	segs := mustSplit(t, in, date(2026, 10, 19), date(2026, 10, 20))

	var buf bytes.Buffer
	if err := WriteCSV(&buf, segs); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("re-reading CSV: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2:\n%s", len(records), buf.String())
	}
	wantHeader := []string{"date", "iso_week", "user", "email", "team", "project", "start", "end", "seconds", "note"}
	if !equalStrings(records[0], wantHeader) {
		t.Errorf("header = %v, want %v", records[0], wantHeader)
	}
	wantRow := []string{
		"2026-10-19", "2026-W43", "Ada Lovelace", "ada@example.com", "Platform", "Timeline",
		"2026-10-19T09:00:00+02:00", "2026-10-19T11:30:00+02:00", "9000", note,
	}
	if !equalStrings(records[1], wantRow) {
		t.Errorf("row = %v, want %v", records[1], wantRow)
	}
}

// 8. A running entry contributes nothing when Now is zero, and is clipped to
// Now when it is set.
func TestSplitRunningEntry(t *testing.T) {
	start := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	entry := store.Entry{ID: 1, UserID: 1, ProjectID: 1, StartAt: start, EndAt: nil}

	// Zero Now: nothing.
	in := fixture("UTC", entry)
	if segs := mustSplit(t, in, date(2026, 10, 19), date(2026, 10, 20)); len(segs) != 0 {
		t.Errorf("zero Now gave %d segments, want 0", len(segs))
	}

	// Now set: clip to Now - start.
	in.Now = start.Add(30 * time.Minute)
	segs := mustSplit(t, in, date(2026, 10, 19), date(2026, 10, 20))
	if len(segs) != 1 {
		t.Fatalf("got %d segments, want 1", len(segs))
	}
	if segs[0].Seconds != 1800 {
		t.Errorf("seconds = %d, want 1800", segs[0].Seconds)
	}
}

// Missing reference data is a descriptive error, not a silent drop.
func TestSplitMissingReferences(t *testing.T) {
	start := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	in := fixture("UTC", store.Entry{ID: 7, UserID: 99, ProjectID: 1, StartAt: start, EndAt: &end})
	_, err := Split(in, date(2026, 10, 19), date(2026, 10, 20))
	if err == nil || !strings.Contains(err.Error(), "unknown user 99") {
		t.Errorf("err = %v, want unknown user 99", err)
	}
}

func TestAggregateEmptyIsNonNil(t *testing.T) {
	rows, total := Aggregate(nil, "day")
	if rows == nil {
		t.Error("rows = nil, want non-nil empty slice")
	}
	if len(rows) != 0 || total != 0 {
		t.Errorf("rows = %v, total = %d; want empty and 0", rows, total)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
