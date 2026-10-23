// Package mcp exposes the Project Times API as MCP tools over stdio. It talks
// through internal/client with the same config and token as the CLI, so it needs
// no database and no server internals.
package mcp

import (
	"context"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/havspect/project-times/internal/client"
	"github.com/havspect/project-times/internal/version"
)

// Client is the API surface the tools use. *client.Client is the production
// implementation; tests pass a fake.
type Client interface {
	Me() (*client.Me, error)
	ListProjects(all bool) ([]client.Project, error)
	ListTeams() ([]client.Team, error)
	ListUsers() ([]client.User, error)
	ListEntries(q client.EntryQuery) ([]client.Entry, error)
	StartTimer(projectID uint, timezone, note string) (*client.Entry, error)
	StopTimer(note string) (*client.Entry, error)
	CreateEntry(in client.EntryInput) (*client.Entry, error)
	PatchEntry(id uint, p client.EntryPatch) (*client.Entry, error)
	DeleteEntry(id uint) error
	Report(q client.ReportQuery) (*client.Report, error)
}

// New returns the MCP server with every tool registered.
func New(c Client) *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{
		Name: "times", Title: "Project Times", Version: version.Version,
	}, nil)
	register(s, c)
	return s
}

// Serve runs the server over stdin and stdout until the client closes the pipe.
func Serve(ctx context.Context, c Client) error {
	return New(c).Run(ctx, &sdk.StdioTransport{})
}

// readOnly builds a tool whose annotation tells an agent it changes nothing.
func readOnly(name, description string) *sdk.Tool {
	return &sdk.Tool{
		Name: name, Description: description,
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true},
	}
}

func write(name, description string) *sdk.Tool {
	return &sdk.Tool{Name: name, Description: description}
}

// deleted is the reply a delete tool returns.
type deleted struct {
	Deleted bool `json:"deleted" jsonschema:"true once the entry is soft-deleted"`
}

func register(s *sdk.Server, c Client) {
	sdk.AddTool(s, readOnly("whoami",
		"Return the authenticated user: id, email, name, timezone, admin flag and team roles. Needs only a valid token."),
		func(_ context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, client.Me, error) {
			me, err := c.Me()
			if err != nil {
				return nil, client.Me{}, err
			}
			return nil, *me, nil
		})

	sdk.AddTool(s, readOnly("list_projects",
		"List projects to log against, with their team and archive state. Any member may list non-archived projects; all=true includes archived projects and needs an admin."),
		func(_ context.Context, _ *sdk.CallToolRequest, in listProjectsInput) (*sdk.CallToolResult, []client.Project, error) {
			projects, err := c.ListProjects(in.All)
			return nil, projects, err
		})

	sdk.AddTool(s, readOnly("list_entries",
		"List entries the token holder may read: own entries for a member, own plus their teams' projects for a lead, everything for an admin. Dates are YYYY-MM-DD, from inclusive and to exclusive; both default to the current ISO week. include_deleted needs an admin."),
		func(_ context.Context, _ *sdk.CallToolRequest, in listEntriesInput) (*sdk.CallToolResult, []client.Entry, error) {
			entries, err := c.ListEntries(client.EntryQuery{
				From: in.From, To: in.To, UserID: in.UserID,
				ProjectID: in.ProjectID, IncludeDeleted: in.IncludeDeleted,
			})
			return nil, entries, err
		})

	sdk.AddTool(s, readOnly("report",
		"Totals of logged hours per person and project. A member reads their own hours, a lead their own plus their teams' projects, an admin everything. Dates are YYYY-MM-DD, from inclusive and to exclusive; both default to the current ISO week. by is week, day or none."),
		func(_ context.Context, _ *sdk.CallToolRequest, in reportInput) (*sdk.CallToolResult, client.Report, error) {
			r, err := c.Report(client.ReportQuery{
				From: in.From, To: in.To, By: in.By,
				TeamID: in.TeamID, UserID: in.UserID, ProjectID: in.ProjectID,
			})
			if err != nil {
				return nil, client.Report{}, err
			}
			return nil, *r, nil
		})

	sdk.AddTool(s, readOnly("list_teams",
		"List every team with its members and roles. Admin only."),
		func(_ context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, []client.Team, error) {
			teams, err := c.ListTeams()
			return nil, teams, err
		})

	sdk.AddTool(s, readOnly("list_users",
		"List every user with timezone, admin flag and active state. Admin only."),
		func(_ context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, []client.User, error) {
			users, err := c.ListUsers()
			return nil, users, err
		})

	sdk.AddTool(s, write("start_timer",
		"Start the token holder's running entry on a project. timezone is the IANA zone it is logged in, defaulting to this machine's. A user has at most one running entry; a second start is refused with 409 and the running entry."),
		func(_ context.Context, _ *sdk.CallToolRequest, in startTimerInput) (*sdk.CallToolResult, client.Entry, error) {
			tz, err := zoneFor(c, in.Timezone)
			if err != nil {
				return nil, client.Entry{}, err
			}
			e, err := c.StartTimer(in.ProjectID, tz, in.Note)
			if err != nil {
				return nil, client.Entry{}, err
			}
			return nil, *e, nil
		})

	sdk.AddTool(s, write("stop_timer",
		"Stop the token holder's running entry at the server's now, optionally adding a note. Fails if nothing runs."),
		func(_ context.Context, _ *sdk.CallToolRequest, in stopTimerInput) (*sdk.CallToolResult, client.Entry, error) {
			e, err := c.StopTimer(in.Note)
			if err != nil {
				return nil, client.Entry{}, err
			}
			return nil, *e, nil
		})

	sdk.AddTool(s, write("add_entry",
		"Log a finished entry for the token holder on a project. Times are RFC 3339 instants; timezone is the IANA zone it was logged in, defaulting to this machine's. The entry may not exceed 24 hours or start more than 5 minutes in the future. The project must be live."),
		func(_ context.Context, _ *sdk.CallToolRequest, in addEntryInput) (*sdk.CallToolResult, client.Entry, error) {
			tz, err := zoneFor(c, in.Timezone)
			if err != nil {
				return nil, client.Entry{}, err
			}
			e, err := c.CreateEntry(client.EntryInput{
				ProjectID: in.ProjectID, StartAt: in.StartAt, EndAt: in.EndAt,
				Timezone: tz, Note: in.Note,
			})
			if err != nil {
				return nil, client.Entry{}, err
			}
			return nil, *e, nil
		})

	sdk.AddTool(s, write("update_entry",
		"Change one of the token holder's own entries: project, start, end, timezone or note. A running entry must be given an end to be retimed. Same rules as add_entry."),
		func(_ context.Context, _ *sdk.CallToolRequest, in updateEntryInput) (*sdk.CallToolResult, client.Entry, error) {
			e, err := c.PatchEntry(in.ID, client.EntryPatch{
				ProjectID: in.ProjectID, StartAt: in.StartAt, EndAt: in.EndAt,
				Timezone: in.Timezone, Note: in.Note,
			})
			if err != nil {
				return nil, client.Entry{}, err
			}
			return nil, *e, nil
		})

	sdk.AddTool(s, write("delete_entry",
		"Soft-delete one of the token holder's own entries. It leaves every list and total but stays in storage; only an admin can restore it."),
		func(_ context.Context, _ *sdk.CallToolRequest, in deleteEntryInput) (*sdk.CallToolResult, deleted, error) {
			if err := c.DeleteEntry(in.ID); err != nil {
				return nil, deleted{}, err
			}
			return nil, deleted{Deleted: true}, nil
		})
}

// The input types are the tool schemas; jsonschema tags become the descriptions
// an agent sees.

type listProjectsInput struct {
	All bool `json:"all,omitempty" jsonschema:"include archived projects (admin only)"`
}

type listEntriesInput struct {
	From           string `json:"from,omitempty" jsonschema:"inclusive start date, YYYY-MM-DD; omit both from and to for the current ISO week"`
	To             string `json:"to,omitempty" jsonschema:"exclusive end date, YYYY-MM-DD; required with from"`
	UserID         uint   `json:"user_id,omitempty" jsonschema:"only this user's entries, within what the caller may read"`
	ProjectID      uint   `json:"project_id,omitempty" jsonschema:"only this project's entries"`
	IncludeDeleted bool   `json:"include_deleted,omitempty" jsonschema:"include soft-deleted entries (admin only)"`
}

type reportInput struct {
	From      string `json:"from,omitempty" jsonschema:"inclusive start date, YYYY-MM-DD; omit both from and to for the current ISO week"`
	To        string `json:"to,omitempty" jsonschema:"exclusive end date, YYYY-MM-DD; required with from"`
	By        string `json:"by,omitempty" jsonschema:"bucket: week (default), day or none"`
	TeamID    uint   `json:"team_id,omitempty" jsonschema:"only entries on this team's projects"`
	UserID    uint   `json:"user_id,omitempty" jsonschema:"only this user's entries, within what the caller may read"`
	ProjectID uint   `json:"project_id,omitempty" jsonschema:"only this project's entries"`
}

type startTimerInput struct {
	ProjectID uint   `json:"project_id" jsonschema:"the project to log against; list_projects returns the ids"`
	Timezone  string `json:"timezone,omitempty" jsonschema:"IANA zone the work happens in, e.g. Australia/Sydney; omit for this machine's zone"`
	Note      string `json:"note,omitempty" jsonschema:"what you are working on"`
}

type stopTimerInput struct {
	Note string `json:"note,omitempty" jsonschema:"note to add as the entry ends"`
}

type addEntryInput struct {
	ProjectID uint      `json:"project_id" jsonschema:"the project to log against; list_projects returns the ids"`
	StartAt   time.Time `json:"start_at" jsonschema:"RFC 3339 start instant, e.g. 2026-10-19T09:00:00+02:00"`
	EndAt     time.Time `json:"end_at" jsonschema:"RFC 3339 end instant"`
	Timezone  string    `json:"timezone,omitempty" jsonschema:"IANA zone the work happened in, e.g. Australia/Sydney; omit for this machine's zone"`
	Note      string    `json:"note,omitempty" jsonschema:"what was done"`
}

type updateEntryInput struct {
	ID        uint       `json:"id" jsonschema:"the entry id from list_entries"`
	ProjectID *uint      `json:"project_id,omitempty" jsonschema:"move the entry to this project"`
	StartAt   *time.Time `json:"start_at,omitempty" jsonschema:"new RFC 3339 start instant"`
	EndAt     *time.Time `json:"end_at,omitempty" jsonschema:"new RFC 3339 end instant"`
	Timezone  *string    `json:"timezone,omitempty" jsonschema:"correct the IANA zone the entry was logged in"`
	Note      *string    `json:"note,omitempty" jsonschema:"new note; an empty string clears it"`
}

type deleteEntryInput struct {
	ID uint `json:"id" jsonschema:"the entry id from list_entries"`
}

// zoneFor falls back to the machine's zone when the tool caller names none.
func zoneFor(c Client, given string) (string, error) {
	if given != "" {
		return given, nil
	}
	me, err := c.Me()
	if err != nil {
		return "", err
	}
	return client.MachineTimezone(me.Timezone), nil
}
