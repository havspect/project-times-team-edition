package cli

import (
	"fmt"
	"time"

	"github.com/havspect/tstamps/internal/client"
)

// Dates the CLI accepts on its own; the API takes plain YYYY-MM-DD.
const dateLayout = "2006-01-02"

// location is the user's profile zone, the fallback for an entry whose stored
// zone cannot be loaded.
func location(me *client.Me) *time.Location {
	loc, err := time.LoadLocation(me.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// local is the zone wall-clock input is read in: the machine, not the profile,
// so a user who travels types the time they are living in.
func (a *App) local() *time.Location {
	if a.Local != nil {
		return a.Local
	}
	return time.Local
}

// zone is the IANA name sent when creating an entry.
func (a *App) zone(me *client.Me) string {
	if a.Zone != "" {
		return a.Zone
	}
	return client.MachineTimezone(me.Timezone)
}

// entryLocation is the zone an entry is shown in, falling back to loc when the
// stored name is empty or unknown.
func entryLocation(e client.Entry, loc *time.Location) *time.Location {
	if e.Timezone == "" {
		return loc
	}
	if zone, err := time.LoadLocation(e.Timezone); err == nil {
		return zone
	}
	return loc
}

// parseDate reads today, yesterday or YYYY-MM-DD as local midnight.
func parseDate(value string, now time.Time, loc *time.Location) (time.Time, error) {
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	switch value {
	case "", "today":
		return today, nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	}
	day, err := time.ParseInLocation(dateLayout, value, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q; use today, yesterday or YYYY-MM-DD", value)
	}
	return day, nil
}

// parseInstant reads an RFC 3339 instant, a date and time, or a clock time on
// date. A bare clock time means today unless the caller passes --date.
func parseInstant(value string, date time.Time, loc *time.Location) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", value, loc); err == nil {
		return t, nil
	}
	clock, err := time.ParseInLocation("15:04", value, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q; use 15:04, YYYY-MM-DD HH:MM or RFC 3339", value)
	}
	return time.Date(date.Year(), date.Month(), date.Day(), clock.Hour(), clock.Minute(), 0, 0, loc), nil
}

// parseEnd reads an end instant, or a duration counted from start. A bare clock
// time not after start crosses local midnight, so 23:00 to 01:00 is two hours.
func parseEnd(value string, start, date time.Time, loc *time.Location) (time.Time, error) {
	if d, err := time.ParseDuration(value); err == nil {
		return start.Add(d), nil
	}
	end, err := parseInstant(value, date, loc)
	if err != nil {
		return time.Time{}, err
	}
	if isClockTime(value) && !end.After(start) {
		end = end.AddDate(0, 0, 1)
	}
	return end, nil
}

// isClockTime reports whether value is exactly HH:MM.
func isClockTime(value string) bool {
	_, err := time.Parse("15:04", value)
	return err == nil
}

// isDuration reports whether value is a Go duration such as 1h30m.
func isDuration(value string) bool {
	_, err := time.ParseDuration(value)
	return err == nil
}

// formatSeconds writes a duration as 2h30m, or 45m under an hour.
func formatSeconds(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	h, m := seconds/3600, (seconds%3600)/60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// secondsOf is the length of an entry; a running entry counts up to now.
func secondsOf(e client.Entry, now time.Time) int64 {
	end := now
	if e.EndAt != nil {
		end = *e.EndAt
	}
	return int64(end.Sub(e.StartAt).Seconds())
}

// formatClock is HH:MM in the caller's zone.
func formatClock(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("15:04")
}

// formatDate is YYYY-MM-DD in the caller's zone.
func formatDate(t time.Time, loc *time.Location) string {
	return t.In(loc).Format(dateLayout)
}
