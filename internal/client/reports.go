package client

import (
	"net/http"
	"net/url"
)

// ReportQuery is the filter every report shares. Empty dates default to the
// caller's ISO week; By is week, day or none.
type ReportQuery struct {
	From      string
	To        string
	By        string
	TeamID    uint
	UserID    uint
	ProjectID uint
}

func (q ReportQuery) values() url.Values {
	v := url.Values{}
	setString(v, "from", q.From)
	setString(v, "to", q.To)
	setString(v, "by", q.By)
	setUint(v, "team_id", q.TeamID)
	setUint(v, "user_id", q.UserID)
	setUint(v, "project_id", q.ProjectID)
	return v
}

// ReportRow is one bucket, person and project of a report.
type ReportRow struct {
	Bucket    string `json:"bucket"`
	UserID    uint   `json:"user_id"`
	User      string `json:"user"`
	Email     string `json:"email"`
	TeamID    uint   `json:"team_id"`
	Team      string `json:"team"`
	ProjectID uint   `json:"project_id"`
	Project   string `json:"project"`
	Archived  bool   `json:"archived"`
	Seconds   int64  `json:"seconds"`
}

// Report is the aggregated table.
type Report struct {
	Rows         []ReportRow `json:"rows"`
	TotalSeconds int64       `json:"total_seconds"`
}

// Report aggregates the caller's readable hours.
func (c *Client) Report(q ReportQuery) (*Report, error) {
	var r Report
	if err := c.do(http.MethodGet, "/reports?"+q.values().Encode(), nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ReportCSV is the same report as one row per entry per local day.
func (c *Client) ReportCSV(q ReportQuery) ([]byte, error) {
	return c.bytes(http.MethodGet, "/reports.csv?"+q.values().Encode())
}
