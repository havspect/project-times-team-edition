package client

import (
	"net/http"
	"time"
)

// Project is the unit of work entries are logged against.
type Project struct {
	ID         uint       `json:"id"`
	TeamID     uint       `json:"team_id"`
	Name       string     `json:"name"`
	Color      string     `json:"color"`
	ArchivedAt *time.Time `json:"archived_at"`
}

// ProjectInput creates a project.
type ProjectInput struct {
	TeamID uint   `json:"team_id"`
	Name   string `json:"name"`
	Color  string `json:"color,omitempty"`
}

// ProjectPatch changes only the fields that are set.
type ProjectPatch struct {
	Name     *string `json:"name,omitempty"`
	Color    *string `json:"color,omitempty"`
	TeamID   *uint   `json:"team_id,omitempty"`
	Archived *bool   `json:"archived,omitempty"`
}

// ListProjects returns the projects everyone can log on, or every project when
// all is set (admin only).
func (c *Client) ListProjects(all bool) ([]Project, error) {
	path := "/projects"
	if all {
		path += "?all=true"
	}
	var out []Project
	if err := c.do(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateProject creates a project in a team the caller leads, or any team as admin.
func (c *Client) CreateProject(in ProjectInput) (*Project, error) {
	var p Project
	if err := c.do(http.MethodPost, "/projects", in, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// PatchProject renames, recolors, moves or archives a project.
func (c *Client) PatchProject(id uint, p ProjectPatch) (*Project, error) {
	var out Project
	if err := c.do(http.MethodPatch, "/projects/"+itoa(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
