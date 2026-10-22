package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/havspect/project-times/internal/store"
)

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }

// userIn creates a user with a timezone, without a membership.
func (f *fixture) userIn(t *testing.T, email, name, tz string) *store.User {
	t.Helper()
	u := &store.User{Email: email, Name: name, PasswordHash: fixtureHash(), Timezone: tz, Active: true}
	if err := f.st.CreateUser(u); err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u
}

type reportBody struct {
	Rows []struct {
		Bucket   string `json:"bucket"`
		UserID   uint   `json:"user_id"`
		User     string `json:"user"`
		Team     string `json:"team"`
		Project  string `json:"project"`
		Archived bool   `json:"archived"`
		Seconds  int64  `json:"seconds"`
	} `json:"rows"`
	TotalSeconds int64 `json:"total_seconds"`
}

func TestReportBucketsAndTotals(t *testing.T) {
	f := seed(t)
	f.entry(t, f.Member, f.ProjectA, time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 19, 11, 0, 0, 0, time.UTC), "monday")
	f.entry(t, f.Member, f.ProjectA, time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 20, 10, 30, 0, 0, time.UTC), "tuesday")
	f.entry(t, f.Member, f.ProjectB, time.Date(2026, 10, 20, 11, 0, 0, 0, time.UTC), time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC), "other project")

	cases := []struct {
		query string
		rows  int
		total int64
	}{
		{"?from=2026-10-19&to=2026-10-26&by=day", 3, 16200},
		{"?from=2026-10-19&to=2026-10-26&by=week", 2, 16200},
		{"?from=2026-10-19&to=2026-10-26&by=none", 2, 16200},
		{"?from=2026-10-19&to=2026-10-26", 2, 16200}, // no by means week buckets
		{"?by=day", 3, 16200},                        // no range means the caller's current ISO week
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			rec := sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/reports"+tc.query, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("reports = %d: %s", rec.Code, rec.Body)
			}
			body := decode[reportBody](t, rec)
			if len(body.Rows) != tc.rows || body.TotalSeconds != tc.total {
				t.Fatalf("got %d rows and %d seconds, want %d and %d: %+v",
					len(body.Rows), body.TotalSeconds, tc.rows, tc.total, body.Rows)
			}
		})
	}

	body := decode[reportBody](t, sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/reports?by=day", nil))
	if body.Rows[0].Bucket != "2026-10-19" || body.Rows[0].Seconds != 2*3600 {
		t.Fatalf("first day row = %+v", body.Rows[0])
	}
	if body.Rows[1].Project != "Sync" || body.Rows[2].Project != "Timeline" {
		t.Fatalf("day rows not ordered by project: %+v", body.Rows)
	}
}

func TestReportScope(t *testing.T) {
	f := seed(t)
	f.entry(t, f.Member, f.ProjectA, f.clock.now.Add(-2*time.Hour), f.clock.now.Add(-time.Hour), "team A work")
	f.entry(t, f.LeadB, f.ProjectB, f.clock.now.Add(-2*time.Hour), f.clock.now.Add(-time.Hour), "team B work")

	cases := []struct {
		name string
		user *store.User
		want int64
	}{
		{"member sees own only", f.Member, 3600},
		{"lead sees own team's", f.LeadA, 3600},
		{"other lead sees their own team", f.LeadB, 3600},
		{"outsider sees nothing", f.Outsider, 0},
		{"admin sees everything", f.Admin, 7200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := decode[reportBody](t, sendAs(t, f, tc.user, http.MethodGet, apiPrefix+"/reports?by=none", nil))
			if body.TotalSeconds != tc.want {
				t.Fatalf("total = %d, want %d: %+v", body.TotalSeconds, tc.want, body.Rows)
			}
		})
	}

	// The team filter narrows an admin's view.
	body := decode[reportBody](t, sendAs(t, f, f.Admin, http.MethodGet, apiPrefix+"/reports?by=none&team_id="+itoa(f.TeamB.ID), nil))
	if body.TotalSeconds != 3600 {
		t.Fatalf("team filter total = %d, want 3600", body.TotalSeconds)
	}
	// The user filter cannot widen a lead's scope.
	body = decode[reportBody](t, sendAs(t, f, f.LeadA, http.MethodGet, apiPrefix+"/reports?by=none&user_id="+itoa(f.LeadB.ID), nil))
	if body.TotalSeconds != 0 {
		t.Fatalf("lead filtered to another user got %d seconds, want 0", body.TotalSeconds)
	}
}

func TestReportCSV(t *testing.T) {
	f := seed(t)
	f.entry(t, f.Member, f.ProjectA,
		time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 19, 11, 30, 0, 0, time.UTC),
		`drag, resize, "snap"`)

	rec := sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/reports.csv?from=2026-10-19&to=2026-10-26", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("csv = %d: %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content type = %q, want text/csv", ct)
	}
	rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	wantHeader := []string{"date", "iso_week", "user", "email", "team", "project", "start", "end", "seconds", "note"}
	if len(rows) != 2 {
		t.Fatalf("csv has %d rows, want header + 1: %q", len(rows), rec.Body)
	}
	for i := range wantHeader {
		if rows[0][i] != wantHeader[i] {
			t.Fatalf("header = %v, want %v", rows[0], wantHeader)
		}
	}
	want := []string{
		"2026-10-19", "2026-W43", "Linus Torvalds", "linus@example.com", "Platform", "Timeline",
		"2026-10-19T09:00:00Z", "2026-10-19T11:30:00Z", "9000", `drag, resize, "snap"`,
	}
	for i := range want {
		if rows[1][i] != want[i] {
			t.Fatalf("row[%d] = %q, want %q (row %v)", i, rows[1][i], want[i], rows[1])
		}
	}
}

func TestReportRangeValidation(t *testing.T) {
	f := seed(t)
	for name, query := range map[string]string{
		"bad from":       "?from=19-10-2026&to=2026-10-26",
		"from alone":     "?from=2026-10-19",
		"to not after":   "?from=2026-10-26&to=2026-10-26",
		"longer than yr": "?from=2025-01-01&to=2026-10-26",
		"unknown bucket": "?by=month",
	} {
		t.Run(name, func(t *testing.T) {
			wantProblem(t, sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/reports"+query, nil),
				http.StatusUnprocessableEntity, "")
		})
	}
}

func TestReportSplitsAtTheSubjectsLocalMidnight(t *testing.T) {
	f := seed(t)
	u := f.userIn(t, "ada.berlin@example.com", "Ada Lovelace", "Europe/Berlin")
	// 21:00Z is 23:00 in Berlin; 01:00Z the next day is 03:00 there.
	f.entry(t, u, f.ProjectA,
		time.Date(2026, 10, 20, 21, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 21, 1, 0, 0, 0, time.UTC), "night shift")

	body := decode[reportBody](t, sendAs(t, f, u, http.MethodGet, apiPrefix+"/reports?from=2026-10-20&to=2026-10-22&by=day", nil))
	if len(body.Rows) != 2 {
		t.Fatalf("rows = %+v, want one per local day", body.Rows)
	}
	if body.Rows[0].Bucket != "2026-10-20" || body.Rows[0].Seconds != 3600 {
		t.Fatalf("first segment = %+v, want 1h on 2026-10-20", body.Rows[0])
	}
	if body.Rows[1].Bucket != "2026-10-21" || body.Rows[1].Seconds != 3*3600 {
		t.Fatalf("second segment = %+v, want 3h on 2026-10-21", body.Rows[1])
	}
	if body.TotalSeconds != 4*3600 {
		t.Fatalf("total = %d, want 4h", body.TotalSeconds)
	}
}

func TestReportDefaultWeekUsesTheCallersLocalDates(t *testing.T) {
	f := seed(t)
	u := f.userIn(t, "kiri@example.com", "Kiri", "Pacific/Kiritimati") // UTC+14
	loc, err := time.LoadLocation("Pacific/Kiritimati")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	// The clock is Thursday 09:00Z, Thursday 23:00 local, so the default week is
	// 2026-10-19..2026-10-26 as local dates. Monday 00:00 local is Sunday in UTC,
	// which must not widen the range to Sunday the 18th.
	sunday := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	f.entry(t, u, f.ProjectA, sunday, sunday.Add(time.Hour), "sunday")
	monday := time.Date(2026, 10, 19, 12, 0, 0, 0, loc)
	f.entry(t, u, f.ProjectA, monday, monday.Add(time.Hour), "monday")

	body := decode[reportBody](t, sendAs(t, f, u, http.MethodGet, apiPrefix+"/reports?by=none", nil))
	if body.TotalSeconds != 3600 || len(body.Rows) != 1 {
		t.Fatalf("default week total = %d, want only Monday's hour: %+v", body.TotalSeconds, body.Rows)
	}
}

func TestReportCountsArchivedProjects(t *testing.T) {
	f := seed(t)
	archived := f.clock.now
	f.ProjectA.ArchivedAt = &archived
	if err := f.st.UpdateProject(f.ProjectA); err != nil {
		t.Fatalf("archive: %v", err)
	}
	f.entry(t, f.Member, f.ProjectA, f.clock.now.Add(-2*time.Hour), f.clock.now.Add(-time.Hour), "old work")

	body := decode[reportBody](t, sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/reports?by=none", nil))
	if body.TotalSeconds != 3600 || len(body.Rows) != 1 || !body.Rows[0].Archived {
		t.Fatalf("archived project hours = %+v, want 1h marked archived", body.Rows)
	}
}

func TestReportCountsTheRunningEntryUpToNow(t *testing.T) {
	f := seed(t)
	f.running(t, f.Member, f.ProjectA, f.clock.now.Add(-90*time.Minute), "still going")

	body := decode[reportBody](t, sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/reports?by=none", nil))
	if body.TotalSeconds != 90*60 {
		t.Fatalf("total = %d, want 90 minutes", body.TotalSeconds)
	}
}
