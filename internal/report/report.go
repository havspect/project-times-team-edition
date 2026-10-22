// Package report turns stored entries into subject-local day segments,
// aggregated totals and the CSV detail export.
//
// It opens no database and reads no clock: the caller loads and scopes the
// rows and passes every instant, including "now", so tests never sleep.
package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/havspect/project-times/internal/store"
)

const dateLayout = "2006-01-02"

// Input is everything Split needs, already loaded and scoped by the caller.
type Input struct {
	Entries  []store.Entry
	Users    []store.User
	Projects []store.Project
	Teams    []store.Team
	// Now clips a still-running entry (EndAt == nil) to Now-start. Zero means
	// running entries contribute nothing.
	Now time.Time
}

// Segment is one entry clipped to one subject-local day.
type Segment struct {
	Date      string // subject-local "2006-01-02"
	Week      string // ISO week of that local date, e.g. "2026-W43"
	UserID    uint
	User      string
	Email     string
	TeamID    uint
	Team      string
	ProjectID uint
	Project   string
	Archived  bool
	Start     time.Time // instants, already located in the subject's timezone
	End       time.Time
	Seconds   int64
	Note      string
}

// Row is one (bucket, user, project) total.
type Row struct {
	Bucket    string // local date, ISO week, or "" for by=none
	UserID    uint
	User      string
	Email     string
	TeamID    uint
	Team      string
	ProjectID uint
	Project   string
	Archived  bool
	Seconds   int64
}

// Split clips and splits the entries into one segment per subject-local day,
// keeping only days in [from, to). from and to are UTC-midnight instants whose
// date components bound the range: from inclusive, to exclusive.
func Split(in Input, from, to time.Time) ([]Segment, error) {
	fromDate := from.Format(dateLayout)
	toDate := to.Format(dateLayout)

	users := make(map[uint]store.User, len(in.Users))
	for _, u := range in.Users {
		users[u.ID] = u
	}
	projects := make(map[uint]store.Project, len(in.Projects))
	for _, p := range in.Projects {
		projects[p.ID] = p
	}
	teams := make(map[uint]store.Team, len(in.Teams))
	for _, t := range in.Teams {
		teams[t.ID] = t
	}
	locations := map[string]*time.Location{}

	segments := []Segment{}
	for _, e := range in.Entries {
		if e.DeletedAt != nil {
			continue // caller excludes deleted; defensive here too
		}
		user, ok := users[e.UserID]
		if !ok {
			return nil, fmt.Errorf("report: entry %d references unknown user %d", e.ID, e.UserID)
		}
		project, ok := projects[e.ProjectID]
		if !ok {
			return nil, fmt.Errorf("report: entry %d references unknown project %d", e.ID, e.ProjectID)
		}
		team, ok := teams[project.TeamID]
		if !ok {
			return nil, fmt.Errorf("report: entry %d project %d references unknown team %d", e.ID, project.ID, project.TeamID)
		}
		loc, err := location(locations, user.Timezone)
		if err != nil {
			return nil, fmt.Errorf("report: entry %d user %d: %w", e.ID, user.ID, err)
		}

		end := e.StartAt
		switch {
		case e.EndAt != nil:
			end = *e.EndAt
		case !in.Now.IsZero():
			end = in.Now
		default:
			continue // running entry with no "now": contributes nothing
		}
		if !end.After(e.StartAt) {
			continue
		}

		for cur := e.StartAt; cur.Before(end); {
			local := cur.In(loc)
			y, m, d := local.Date()
			// Next local midnight; time.Date normalises zones that skip it.
			next := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
			if !next.After(cur) {
				next = cur.Add(24 * time.Hour) // guard against a skipped whole day
			}
			segEnd := end
			if next.Before(segEnd) {
				segEnd = next
			}
			date := local.Format(dateLayout)
			if date >= fromDate && date < toDate {
				year, week := local.ISOWeek()
				segments = append(segments, Segment{
					Date:      date,
					Week:      fmt.Sprintf("%d-W%02d", year, week),
					UserID:    user.ID,
					User:      user.Name,
					Email:     user.Email,
					TeamID:    team.ID,
					Team:      team.Name,
					ProjectID: project.ID,
					Project:   project.Name,
					Archived:  project.ArchivedAt != nil,
					Start:     cur.In(loc),
					End:       segEnd.In(loc),
					Seconds:   int64(segEnd.Sub(cur) / time.Second),
					Note:      e.Note,
				})
			}
			cur = next
		}
	}
	return segments, nil
}

// Aggregate totals segments by "day", "week" or "none". It returns rows sorted
// by bucket, then user name, then project name, and the grand total in seconds.
func Aggregate(segments []Segment, by string) ([]Row, int64) {
	type key struct {
		bucket    string
		userID    uint
		projectID uint
	}
	index := map[key]*Row{}
	var total int64
	for _, s := range segments {
		var bucket string
		switch by {
		case "day":
			bucket = s.Date
		case "week":
			bucket = s.Week
		}
		k := key{bucket, s.UserID, s.ProjectID}
		r, ok := index[k]
		if !ok {
			r = &Row{
				Bucket:    bucket,
				UserID:    s.UserID,
				User:      s.User,
				Email:     s.Email,
				TeamID:    s.TeamID,
				Team:      s.Team,
				ProjectID: s.ProjectID,
				Project:   s.Project,
				Archived:  s.Archived,
			}
			index[k] = r
		}
		r.Seconds += s.Seconds
		total += s.Seconds
	}

	rows := make([]Row, 0, len(index))
	for _, r := range index {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Bucket != rows[j].Bucket {
			return rows[i].Bucket < rows[j].Bucket
		}
		if rows[i].User != rows[j].User {
			return rows[i].User < rows[j].User
		}
		if rows[i].Project != rows[j].Project {
			return rows[i].Project < rows[j].Project
		}
		return rows[i].ProjectID < rows[j].ProjectID
	})
	return rows, total
}

// WriteCSV writes the detail export: one row per segment, with the header
// date,iso_week,user,email,team,project,start,end,seconds,note
func WriteCSV(w io.Writer, segments []Segment) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"date", "iso_week", "user", "email", "team", "project",
		"start", "end", "seconds", "note",
	}); err != nil {
		return err
	}
	for _, s := range segments {
		if err := cw.Write([]string{
			s.Date, s.Week, s.User, s.Email, s.Team, s.Project,
			s.Start.Format(time.RFC3339), s.End.Format(time.RFC3339),
			strconv.FormatInt(s.Seconds, 10), s.Note,
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// location loads and caches the subject's IANA timezone; empty means UTC.
func location(cache map[string]*time.Location, tz string) (*time.Location, error) {
	if tz == "" {
		tz = "UTC"
	}
	if loc, ok := cache[tz]; ok {
		return loc, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("unknown timezone %q", tz)
	}
	cache[tz] = loc
	return loc, nil
}
