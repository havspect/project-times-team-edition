package client

import (
	"net/http"
	"time"
)

// Member is one membership row as a team reply shows it.
type Member struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

// Team owns projects and has one or more leads.
type Team struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	ArchivedAt *time.Time `json:"archived_at"`
	Members    []Member   `json:"members"`
}

// ListTeams returns every team with its members; admin only.
func (c *Client) ListTeams() ([]Team, error) {
	var out []Team
	if err := c.do(http.MethodGet, "/teams", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
