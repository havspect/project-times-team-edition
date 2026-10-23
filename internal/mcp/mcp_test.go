package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/havspect/tstamps/internal/client"
)

// fake embeds the interface so a test implements only what it calls.
type fake struct {
	Client
	me        client.Me
	report    client.Report
	lastQuery client.ReportQuery
	lastEntry client.EntryQuery
	deletedID uint
	startErr  error
}

func (f *fake) Me() (*client.Me, error) { return &f.me, nil }

func (f *fake) Report(q client.ReportQuery) (*client.Report, error) {
	f.lastQuery = q
	return &f.report, nil
}

func (f *fake) ListEntries(q client.EntryQuery) ([]client.Entry, error) {
	f.lastEntry = q
	return []client.Entry{}, nil
}

func (f *fake) StartTimer(uint, string, string) (*client.Entry, error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	return &client.Entry{ID: 1}, nil
}

func (f *fake) DeleteEntry(id uint) error {
	f.deletedID = id
	return nil
}

// connect starts the server over in-memory transports and returns a client.
func connect(t *testing.T, c Client) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := New(c)
	t1, t2 := sdk.NewInMemoryTransports()
	ss, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestToolsAndAnnotations(t *testing.T) {
	cs := connect(t, &fake{})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	wantReadOnly := map[string]bool{
		"whoami": true, "list_projects": true, "list_entries": true, "report": true,
		"list_teams": true, "list_users": true,
		"start_timer": false, "stop_timer": false, "add_entry": false,
		"update_entry": false, "delete_entry": false,
	}
	got := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		if tool.Description == "" {
			t.Errorf("tool %q has no description; an agent cannot explain a refusal", tool.Name)
		}
		got[tool.Name] = tool.Annotations != nil && tool.Annotations.ReadOnlyHint
	}
	for name, readOnly := range wantReadOnly {
		flag, ok := got[name]
		if !ok {
			t.Errorf("tool %q is missing", name)
			continue
		}
		if flag != readOnly {
			t.Errorf("tool %q readOnly = %v, want %v", name, flag, readOnly)
		}
	}
	if len(got) != len(wantReadOnly) {
		t.Errorf("registered %d tools, want %d", len(got), len(wantReadOnly))
	}
}

func TestWhoamiAndReport(t *testing.T) {
	cs := connect(t, &fake{
		me: client.Me{ID: 7, Email: "ada@example.com", Name: "Ada Lovelace", Timezone: "UTC"},
		report: client.Report{
			Rows:         []client.ReportRow{{Bucket: "2026-W43", User: "Ada Lovelace", Seconds: 9000}},
			TotalSeconds: 9000,
		},
	})
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "whoami"})
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if res.IsError {
		t.Fatalf("whoami failed: %v", contentText(res))
	}
	if body := contentText(res); !strings.Contains(body, "ada@example.com") {
		t.Fatalf("whoami body %q does not name the user", body)
	}

	res, err = cs.CallTool(ctx, &sdk.CallToolParams{
		Name:      "report",
		Arguments: map[string]any{"from": "2026-10-19", "to": "2026-10-26", "by": "week"},
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if body := contentText(res); !strings.Contains(body, "9000") {
		t.Fatalf("report body %q does not carry the total", body)
	}
}

func TestDeleteAndError(t *testing.T) {
	f := &fake{startErr: errors.New("Conflict: already running")}
	cs := connect(t, f)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &sdk.CallToolParams{
		Name: "delete_entry", Arguments: map[string]any{"id": 7},
	})
	if err != nil {
		t.Fatalf("delete_entry: %v", err)
	}
	if res.IsError {
		t.Fatalf("delete_entry failed: %v", contentText(res))
	}
	if f.deletedID != 7 {
		t.Fatalf("deleted id %d, want 7", f.deletedID)
	}

	res, err = cs.CallTool(ctx, &sdk.CallToolParams{
		Name: "start_timer", Arguments: map[string]any{"project_id": 3},
	})
	if err != nil {
		t.Fatalf("start_timer: %v", err)
	}
	if !res.IsError {
		t.Fatal("start_timer succeeded, want a tool error")
	}
	if body := contentText(res); !strings.Contains(body, "already running") {
		t.Fatalf("error body %q does not carry the server detail", body)
	}
}

// contentText joins every text content item, which is where structured output
// and errors both land.
func contentText(res *sdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*sdk.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	if b.Len() == 0 && res.StructuredContent != nil {
		raw, _ := json.Marshal(res.StructuredContent)
		b.Write(raw)
	}
	return b.String()
}
