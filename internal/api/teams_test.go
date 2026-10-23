package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/havspect/tstamps/internal/store"
)

// teamRequest sends a cookie-authenticated request with the same-origin header a
// mutation needs.
func (f *fixture) teamRequest(t *testing.T, u *store.User, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(method, path, body, map[string]string{"Origin": "http://example.com"}, f.as(t, u))
}

func teamPath(suffix string) string { return apiPrefix + "/teams" + suffix }

func teamRoles(members []teamMemberResponse) map[uint]string {
	out := make(map[uint]string, len(members))
	for _, m := range members {
		out[m.UserID] = m.Role
	}
	return out
}

func teamProblemType(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	return decode[struct {
		Type string `json:"type"`
	}](t, rec).Type
}

func TestTeamsAdminOnly(t *testing.T) {
	f := seed(t)
	cases := []struct {
		name, method, path string
		body               any
	}{
		{"list", http.MethodGet, teamPath(""), nil},
		{"create", http.MethodPost, teamPath(""), map[string]string{"name": "Design"}},
		{"rename", http.MethodPatch, teamPath(fmt.Sprintf("/%d", f.TeamA.ID)), map[string]string{"name": "Core"}},
		{"archive", http.MethodDelete, teamPath(fmt.Sprintf("/%d", f.TeamA.ID)), nil},
		{"members", http.MethodPut, teamPath(fmt.Sprintf("/%d/members", f.TeamA.ID)),
			[]map[string]any{{"user_id": f.Outsider.ID, "role": store.RoleMember}}},
	}
	for _, tc := range cases {
		for _, u := range []*store.User{f.LeadA, f.Member, f.Outsider} {
			rec := f.teamRequest(t, u, tc.method, tc.path, tc.body)
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s as %s: status = %d, want 403: %s", tc.name, u.Email, rec.Code, rec.Body)
			}
		}
		if rec := f.do(tc.method, tc.path, tc.body, nil, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s anonymously: status = %d, want 401: %s", tc.name, rec.Code, rec.Body)
		}
	}

	team, err := f.st.TeamByID(f.TeamA.ID)
	if err != nil {
		t.Fatalf("reload team: %v", err)
	}
	if team.ArchivedAt != nil || team.Name != "Platform" {
		t.Fatalf("a refused request changed the team: %+v", team)
	}
}

func TestCreateTeamThenList(t *testing.T) {
	f := seed(t)

	rec := f.teamRequest(t, f.Admin, http.MethodPost, teamPath(""), map[string]string{"name": "Design"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body)
	}
	created := decode[teamResponse](t, rec)
	if created.ID == 0 || created.Name != "Design" || created.ArchivedAt != nil || len(created.Members) != 0 {
		t.Fatalf("body = %+v, want a new team with no members", created)
	}

	rec = f.teamRequest(t, f.Admin, http.MethodGet, teamPath(""), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", rec.Code, rec.Body)
	}
	teams := decode[[]teamResponse](t, rec)
	byID := make(map[uint]teamResponse, len(teams))
	for _, tm := range teams {
		byID[tm.ID] = tm
	}
	if got, ok := byID[created.ID]; !ok || got.Name != "Design" {
		t.Fatalf("created team missing from list: %+v", teams)
	}
	if got := teamRoles(byID[f.TeamA.ID].Members); len(got) != 2 ||
		got[f.LeadA.ID] != store.RoleLead || got[f.Member.ID] != store.RoleMember {
		t.Fatalf("TeamA members = %+v, want LeadA lead and Member member", byID[f.TeamA.ID].Members)
	}
}

func TestRenameTeam(t *testing.T) {
	f := seed(t)

	rec := f.teamRequest(t, f.Admin, http.MethodPatch, teamPath(fmt.Sprintf("/%d", f.TeamA.ID)),
		map[string]string{"name": "Core"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	renamed := decode[teamResponse](t, rec)
	if renamed.Name != "Core" || renamed.ID != f.TeamA.ID || len(renamed.Members) != 2 {
		t.Fatalf("body = %+v, want TeamA renamed with its members", renamed)
	}

	rec = f.teamRequest(t, f.Admin, http.MethodPatch, teamPath("/99999"), map[string]string{"name": "Core"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: status = %d, want 404: %s", rec.Code, rec.Body)
	}
	if got := teamProblemType(t, rec); got != urnNotFound {
		t.Fatalf("unknown id: type = %q, want %q", got, urnNotFound)
	}
}

func TestArchiveTeam(t *testing.T) {
	f := seed(t)
	path := teamPath(fmt.Sprintf("/%d", f.TeamA.ID))

	for range 2 {
		rec := f.teamRequest(t, f.Admin, http.MethodDelete, path, nil)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body)
		}
	}

	team, err := f.st.TeamByID(f.TeamA.ID)
	if err != nil {
		t.Fatalf("reload team: %v", err)
	}
	if team.ArchivedAt == nil || !team.ArchivedAt.Equal(f.clock.now) {
		t.Fatalf("archived_at = %v, want %v", team.ArchivedAt, f.clock.now)
	}
	// Membership rows stay, so editing them after archive is allowed.
	rec := f.teamRequest(t, f.Admin, http.MethodPut, path+"/members",
		[]map[string]any{{"user_id": f.Member.ID, "role": store.RoleMember}})
	if rec.Code != http.StatusOK {
		t.Fatalf("members of archived team: status = %d, want 200: %s", rec.Code, rec.Body)
	}

	// The list still shows the archived team.
	rec = f.teamRequest(t, f.Admin, http.MethodGet, teamPath(""), nil)
	teams := decode[[]teamResponse](t, rec)
	var found bool
	for _, tm := range teams {
		if tm.ID == f.TeamA.ID && tm.ArchivedAt != nil {
			found = true
		}
	}
	if !found {
		t.Fatalf("archived team missing from list: %+v", teams)
	}

	if rec := f.teamRequest(t, f.Admin, http.MethodDelete, teamPath("/99999"), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: status = %d, want 404: %s", rec.Code, rec.Body)
	}
}

func TestReplaceMembers(t *testing.T) {
	f := seed(t)
	path := teamPath(fmt.Sprintf("/%d/members", f.TeamA.ID))

	// LeadA changes role in place, Member is dropped and Outsider joins.
	rec := f.teamRequest(t, f.Admin, http.MethodPut, path, []map[string]any{
		{"user_id": f.LeadA.ID, "role": store.RoleMember},
		{"user_id": f.Outsider.ID, "role": store.RoleLead},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	want := map[uint]string{f.LeadA.ID: store.RoleMember, f.Outsider.ID: store.RoleLead}
	if got := teamRoles(decode[[]teamMemberResponse](t, rec)); len(got) != 2 ||
		got[f.LeadA.ID] != want[f.LeadA.ID] || got[f.Outsider.ID] != want[f.Outsider.ID] {
		t.Fatalf("members = %+v, want %+v", got, want)
	}

	if rec := f.teamRequest(t, f.Admin, http.MethodGet, teamPath(""), nil); rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", rec.Code, rec.Body)
	}
	teams := decode[[]teamResponse](t, rec)
	for _, tm := range teams {
		if tm.ID == f.TeamA.ID && len(tm.Members) != 2 {
			t.Fatalf("TeamA members after replace = %+v, want two", tm.Members)
		}
	}

	// A bad user_id leaves the whole list untouched.
	rec = f.teamRequest(t, f.Admin, http.MethodPut, path, []map[string]any{
		{"user_id": f.LeadA.ID, "role": store.RoleLead},
		{"user_id": 99999, "role": store.RoleMember},
	})
	if rec.Code != http.StatusUnprocessableEntity || teamProblemType(t, rec) != urnInvalidInput {
		t.Fatalf("unknown user: status = %d, want 422 %q: %s", rec.Code, urnInvalidInput, rec.Body)
	}
	rows, err := f.st.MembersOf(f.TeamA.ID)
	if err != nil {
		t.Fatalf("members of TeamA: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("memberships after a refused replace = %+v, want the previous two", rows)
	}

	for _, tc := range []struct {
		name string
		body []map[string]any
	}{
		{"duplicate", []map[string]any{
			{"user_id": f.LeadA.ID, "role": store.RoleLead},
			{"user_id": f.LeadA.ID, "role": store.RoleMember},
		}},
		{"bad role", []map[string]any{{"user_id": f.LeadA.ID, "role": "boss"}}},
	} {
		rec := f.teamRequest(t, f.Admin, http.MethodPut, path, tc.body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422: %s", tc.name, rec.Code, rec.Body)
		}
	}

	// Clearing the list removes every row.
	rec = f.teamRequest(t, f.Admin, http.MethodPut, path, []map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("clear status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if got := decode[[]teamMemberResponse](t, rec); len(got) != 0 {
		t.Fatalf("members after clear = %+v, want none", got)
	}
	rows, err = f.st.MembersOf(f.TeamA.ID)
	if err != nil {
		t.Fatalf("members of TeamA: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("memberships after clear = %+v, want none", rows)
	}

	if rec := f.teamRequest(t, f.Admin, http.MethodPut, teamPath("/99999/members"), []map[string]any{}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown team: status = %d, want 404: %s", rec.Code, rec.Body)
	}
}
