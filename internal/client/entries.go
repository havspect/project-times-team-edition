package client

import (
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Entry is one stretch of work; EndAt is nil while it runs.
type Entry struct {
	ID        uint       `json:"id"`
	UserID    uint       `json:"user_id"`
	ProjectID uint       `json:"project_id"`
	StartAt   time.Time  `json:"start_at"`
	EndAt     *time.Time `json:"end_at"`
	Timezone  string     `json:"timezone"`
	Note      string     `json:"note"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// EntryQuery narrows a listing. Empty dates default to the caller's ISO week.
type EntryQuery struct {
	From           string
	To             string
	UserID         uint
	ProjectID      uint
	IncludeDeleted bool
}

func (q EntryQuery) values() url.Values {
	v := url.Values{}
	setString(v, "from", q.From)
	setString(v, "to", q.To)
	setUint(v, "user_id", q.UserID)
	setUint(v, "project_id", q.ProjectID)
	if q.IncludeDeleted {
		v.Set("include_deleted", "true")
	}
	return v
}

// EntryInput is the body that creates a finished entry.
type EntryInput struct {
	ProjectID uint      `json:"project_id"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	Timezone  string    `json:"timezone"`
	Note      string    `json:"note,omitempty"`
}

// EntryPatch changes only the fields that are set.
type EntryPatch struct {
	ProjectID *uint      `json:"project_id,omitempty"`
	StartAt   *time.Time `json:"start_at,omitempty"`
	EndAt     *time.Time `json:"end_at,omitempty"`
	Timezone  *string    `json:"timezone,omitempty"`
	Note      *string    `json:"note,omitempty"`
}

// ListEntries returns the entries the caller may read in the queried range.
func (c *Client) ListEntries(q EntryQuery) ([]Entry, error) {
	var out []Entry
	if err := c.do(http.MethodGet, "/entries?"+q.values().Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateEntry logs a finished entry.
func (c *Client) CreateEntry(in EntryInput) (*Entry, error) {
	var e Entry
	if err := c.do(http.MethodPost, "/entries", in, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// PatchEntry changes an entry the caller owns.
func (c *Client) PatchEntry(id uint, p EntryPatch) (*Entry, error) {
	var e Entry
	if err := c.do(http.MethodPatch, "/entries/"+itoa(id), p, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// DeleteEntry soft-deletes an entry.
func (c *Client) DeleteEntry(id uint) error {
	return c.do(http.MethodDelete, "/entries/"+itoa(id), nil, nil)
}

// Timer returns the caller's running entry, or nil.
func (c *Client) Timer() (*Entry, error) {
	var e *Entry
	if err := c.do(http.MethodGet, "/timer", nil, &e); err != nil {
		return nil, err
	}
	return e, nil
}

// StartTimer starts a running entry in timezone, or returns a 409 *Error
// carrying the entry that already runs.
func (c *Client) StartTimer(projectID uint, timezone, note string) (*Entry, error) {
	body := struct {
		ProjectID uint   `json:"project_id"`
		Timezone  string `json:"timezone"`
		Note      string `json:"note,omitempty"`
	}{projectID, timezone, note}
	var e Entry
	if err := c.do(http.MethodPost, "/timer/start", body, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// StopTimer ends the running entry at the server's now.
func (c *Client) StopTimer(note string) (*Entry, error) {
	body := struct {
		Note string `json:"note,omitempty"`
	}{note}
	var e Entry
	if err := c.do(http.MethodPost, "/timer/stop", body, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func setString(v url.Values, key, value string) {
	if value != "" {
		v.Set(key, value)
	}
}

func setUint(v url.Values, key string, value uint) {
	if value != 0 {
		v.Set(key, strconv.FormatUint(uint64(value), 10))
	}
}

func itoa(v uint) string { return strconv.FormatUint(uint64(v), 10) }
