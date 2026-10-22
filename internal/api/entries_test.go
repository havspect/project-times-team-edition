package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/havspect/project-times/internal/store"
)

// entryPath is the path of one entry.
func entryPath(id uint) string { return fmt.Sprintf("%s/entries/%d", apiPrefix, id) }

// ownEntry seeds an entry owned by u; the anonymous matrix row gets a dummy id,
// because the middleware refuses it before any lookup.
func (f *fixture) ownEntry(t *testing.T, u *store.User) *store.Entry {
	t.Helper()
	if u == nil {
		return &store.Entry{ID: 1}
	}
	return f.entry(t, u, f.ProjectA, f.clock.now.Add(-2*time.Hour), f.clock.now.Add(-time.Hour), "matrix")
}

// ownRunning seeds a running entry owned by u.
func (f *fixture) ownRunning(t *testing.T, u *store.User) *store.Entry {
	t.Helper()
	if u == nil {
		return &store.Entry{ID: 1}
	}
	return f.running(t, u, f.ProjectA, f.clock.now.Add(-time.Hour), "matrix")
}

// entry seeds a finished entry straight through the store, for tests whose
// subject is not the create endpoint.
func (f *fixture) entry(t *testing.T, u *store.User, p *store.Project, start, end time.Time, note string) *store.Entry {
	t.Helper()
	e := &store.Entry{UserID: u.ID, ProjectID: p.ID, StartAt: start, EndAt: &end, Note: note}
	if err := f.st.CreateEntry(e); err != nil {
		t.Fatalf("create entry: %v", err)
	}
	return e
}

// running seeds a running entry.
func (f *fixture) running(t *testing.T, u *store.User, p *store.Project, start time.Time, note string) *store.Entry {
	t.Helper()
	e := &store.Entry{UserID: u.ID, ProjectID: p.ID, StartAt: start, Note: note}
	if err := f.st.CreateEntry(e); err != nil {
		t.Fatalf("create running entry: %v", err)
	}
	return e
}

func TestEntryCreateAndList(t *testing.T) {
	f := seed(t)
	start := f.clock.now.Add(-2 * time.Hour)
	end := f.clock.now.Add(-time.Hour)

	rec := sendAs(t, f, f.Member, http.MethodPost, apiPrefix+"/entries", map[string]any{
		"project_id": f.ProjectA.ID, "start_at": start, "end_at": end, "note": "drag, resize, snap",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201: %s", rec.Code, rec.Body)
	}
	created := decode[entryResponse](t, rec)
	if created.ID == 0 || created.UserID != f.Member.ID || created.ProjectID != f.ProjectA.ID || created.Note != "drag, resize, snap" {
		t.Fatalf("created = %+v", created)
	}
	if created.EndAt == nil || !created.EndAt.Equal(end) {
		t.Fatalf("end_at = %v, want %v", created.EndAt, end)
	}

	// The member reads it back; the other team's lead does not.
	rec = sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/entries", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", rec.Code, rec.Body)
	}
	if got := decode[[]entryResponse](t, rec); len(got) != 1 || got[0].ID != created.ID {
		t.Fatalf("member list = %+v", got)
	}
	if got := decode[[]entryResponse](t, sendAs(t, f, f.LeadB, http.MethodGet, apiPrefix+"/entries", nil)); len(got) != 0 {
		t.Fatalf("other lead list = %+v, want empty", got)
	}
	if got := decode[[]entryResponse](t, sendAs(t, f, f.LeadA, http.MethodGet, apiPrefix+"/entries", nil)); len(got) != 1 {
		t.Fatalf("team lead list = %+v, want the member's entry", got)
	}
}

func TestEntryCreateRejectsRuleBreaks(t *testing.T) {
	f := seed(t)
	now := f.clock.now
	archived := f.clock.now
	f.ProjectB.ArchivedAt = &archived
	if err := f.st.UpdateProject(f.ProjectB); err != nil {
		t.Fatalf("archive project: %v", err)
	}

	cases := []struct {
		name string
		body map[string]any
		urn  string
	}{
		{"end before start", map[string]any{"project_id": f.ProjectA.ID, "start_at": now.Add(-time.Hour), "end_at": now.Add(-2 * time.Hour)}, urnInvalidInput},
		{"longer than 24h", map[string]any{"project_id": f.ProjectA.ID, "start_at": now.Add(-25 * time.Hour), "end_at": now.Add(-time.Minute)}, urnEntryTooLong},
		{"starts in the future", map[string]any{"project_id": f.ProjectA.ID, "start_at": now.Add(6 * time.Minute), "end_at": now.Add(time.Hour)}, urnInvalidInput},
		{"archived project", map[string]any{"project_id": f.ProjectB.ID, "start_at": now.Add(-time.Hour), "end_at": now}, urnInvalidInput},
		{"unknown project", map[string]any{"project_id": uint(999), "start_at": now.Add(-time.Hour), "end_at": now}, urnInvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := sendAs(t, f, f.Member, http.MethodPost, apiPrefix+"/entries", tc.body)
			wantProblem(t, rec, http.StatusUnprocessableEntity, tc.urn)
		})
	}
}

func TestEntryEdit(t *testing.T) {
	f := seed(t)
	e := f.entry(t, f.Member, f.ProjectA, f.clock.now.Add(-3*time.Hour), f.clock.now.Add(-2*time.Hour), "first")

	// Only the owner edits, whoever else can read it.
	if rec := sendAs(t, f, f.Outsider, http.MethodPatch, entryPath(e.ID), map[string]any{"note": "not mine"}); rec.Code != http.StatusForbidden {
		t.Fatalf("outsider patch = %d, want 403: %s", rec.Code, rec.Body)
	}
	if rec := sendAs(t, f, f.Admin, http.MethodPatch, entryPath(e.ID), map[string]any{"note": "admin"}); rec.Code != http.StatusForbidden {
		t.Fatalf("admin patch = %d, want 403: %s", rec.Code, rec.Body)
	}

	newStart := f.clock.now.Add(-90 * time.Minute)
	rec := sendAs(t, f, f.Member, http.MethodPatch, entryPath(e.ID), map[string]any{
		"start_at": newStart, "end_at": f.clock.now, "note": "edited", "project_id": f.ProjectB.ID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch = %d, want 200: %s", rec.Code, rec.Body)
	}
	got := decode[entryResponse](t, rec)
	if !got.StartAt.Equal(newStart) || got.Note != "edited" || got.ProjectID != f.ProjectB.ID {
		t.Fatalf("patched = %+v", got)
	}

	// A retime that breaks a rule is refused with the rule's own code.
	rec = sendAs(t, f, f.Member, http.MethodPatch, entryPath(e.ID), map[string]any{"end_at": newStart.Add(-time.Minute)})
	wantProblem(t, rec, http.StatusUnprocessableEntity, urnInvalidInput)
}

func TestRunningEntryGetsAnEndTime(t *testing.T) {
	f := seed(t)
	e := f.running(t, f.Member, f.ProjectA, f.clock.now.Add(-26*time.Hour), "overnight")

	// The 24h stop rule is why this path exists: the user picks the end instead
	// of losing the hours.
	rec := sendAs(t, f, f.Member, http.MethodPatch, entryPath(e.ID), map[string]any{"end_at": f.clock.now.Add(-2 * time.Hour)})
	if rec.Code != http.StatusOK {
		t.Fatalf("give a running entry an end = %d, want 200: %s", rec.Code, rec.Body)
	}
	if got := decode[entryResponse](t, rec); got.EndAt == nil {
		t.Fatalf("end_at still nil: %+v", got)
	}

	// Retiming it without an end would silently leave it running, so it is refused.
	rec = sendAs(t, f, f.Member, http.MethodPatch, entryPath(e.ID), map[string]any{"start_at": f.clock.now.Add(-time.Hour)})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("retime without end = %d, want 422: %s", rec.Code, rec.Body)
	}
}

func TestEntryDeleteAndRestore(t *testing.T) {
	f := seed(t)
	e := f.entry(t, f.Member, f.ProjectA, f.clock.now.Add(-2*time.Hour), f.clock.now.Add(-time.Hour), "mine")

	if rec := sendAs(t, f, f.Outsider, http.MethodDelete, entryPath(e.ID), nil); rec.Code != http.StatusForbidden {
		t.Fatalf("outsider delete = %d, want 403: %s", rec.Code, rec.Body)
	}
	if rec := sendAs(t, f, f.Member, http.MethodDelete, entryPath(e.ID), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204: %s", rec.Code, rec.Body)
	}
	if rec := sendAs(t, f, f.Member, http.MethodDelete, entryPath(e.ID), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete = %d, want 404: %s", rec.Code, rec.Body)
	}
	if got := decode[[]entryResponse](t, sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/entries", nil)); len(got) != 0 {
		t.Fatalf("deleted entry still listed: %+v", got)
	}

	if rec := sendAs(t, f, f.LeadA, http.MethodPost, entryPath(e.ID)+"/restore", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("lead restore = %d, want 403: %s", rec.Code, rec.Body)
	}
	rec := sendAs(t, f, f.Admin, http.MethodPost, entryPath(e.ID)+"/restore", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin restore = %d, want 200: %s", rec.Code, rec.Body)
	}
	if got := decode[entryResponse](t, rec); got.DeletedAt != nil {
		t.Fatalf("restored entry still deleted: %+v", got)
	}
	if got := decode[[]entryResponse](t, sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/entries", nil)); len(got) != 1 {
		t.Fatalf("restored entry not listed: %+v", got)
	}
}

func TestProjectWithEntriesKeepsItsTeam(t *testing.T) {
	f := seed(t)
	f.entry(t, f.Member, f.ProjectA, f.clock.now.Add(-2*time.Hour), f.clock.now.Add(-time.Hour), "booked")

	rec := sendAs(t, f, f.Admin, http.MethodPatch, projectPath(f.ProjectA.ID), map[string]any{"team_id": f.TeamB.ID})
	wantProblem(t, rec, http.StatusUnprocessableEntity, urnProjectHasEntries)

	// An empty project still moves.
	if rec := sendAs(t, f, f.Admin, http.MethodPatch, projectPath(f.ProjectB.ID), map[string]any{"team_id": f.TeamA.ID}); rec.Code != http.StatusOK {
		t.Fatalf("move empty project = %d, want 200: %s", rec.Code, rec.Body)
	}
}

func TestDeactivatingAUserStopsTheirTimer(t *testing.T) {
	f := seed(t)
	e := f.running(t, f.Member, f.ProjectA, f.clock.now.Add(-2*time.Hour), "keep me")

	rec := sendAs(t, f, f.Admin, http.MethodPatch, fmt.Sprintf("%s/users/%d", apiPrefix, f.Member.ID), map[string]any{"active": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("deactivate = %d, want 200: %s", rec.Code, rec.Body)
	}

	stopped, err := f.st.EntryByID(e.ID)
	if err != nil {
		t.Fatalf("load entry: %v", err)
	}
	if stopped.EndAt == nil || !stopped.EndAt.Equal(f.clock.now) {
		t.Fatalf("entry end = %v, want %v", stopped.EndAt, f.clock.now)
	}
	if stopped.Note != store.DeactivationNote {
		t.Fatalf("note = %q, want %q", stopped.Note, store.DeactivationNote)
	}
}
