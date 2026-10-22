package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/havspect/project-times/internal/store"
)

// matrixOrigin satisfies the same-origin rule every cookie mutation carries.
var matrixOrigin = map[string]string{"Origin": "http://example.com"}

// cells are the expected statuses in Roles-table order:
// member, lead of the team, lead of another team, outsider, admin, anonymous.
type cells struct {
	member, lead, otherLead, outsider, admin, anon int
}

type permissionCase struct {
	name   string
	method string
	path   func(t *testing.T, f *fixture, u *store.User) string
	body   func(f *fixture) any // nil for requests without a body
	want   cells
}

// bodyFor returns the JSON body for the acting user, or nil.
func (c permissionCase) bodyFor(f *fixture) any {
	if c.body == nil {
		return nil
	}
	return c.body(f)
}

// TestPermissionMatrix walks every stage-2 endpoint once per identity. A fresh
// seed per row keeps archiving, renaming and password resets out of the next row.
func TestPermissionMatrix(t *testing.T) {
	cases := []permissionCase{
		{
			name: "projects list", method: http.MethodGet,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/projects" },
			want: cells{200, 200, 200, 200, 200, 401},
		},
		{
			name: "project create in TeamA", method: http.MethodPost,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/projects" },
			body: func(f *fixture) any {
				return map[string]any{"team_id": f.TeamA.ID, "name": "Matrix"}
			},
			want: cells{403, 201, 403, 403, 201, 401},
		},
		{
			name: "project update in TeamA", method: http.MethodPatch,
			path: func(_ *testing.T, f *fixture, _ *store.User) string { return projectPath(f.ProjectA.ID) },
			body: func(*fixture) any { return map[string]any{"name": "Matrix"} },
			want: cells{403, 200, 403, 403, 200, 401},
		},
		{
			// Roles table: only an admin restores archived projects.
			name: "project restore", method: http.MethodPost,
			path: func(_ *testing.T, f *fixture, _ *store.User) string {
				return projectPath(f.ProjectA.ID) + "/restore"
			},
			want: cells{403, 403, 403, 403, 200, 401},
		},
		{
			name: "teams list", method: http.MethodGet,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/teams" },
			want: cells{403, 403, 403, 403, 200, 401},
		},
		{
			name: "team create", method: http.MethodPost,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/teams" },
			body: func(*fixture) any { return map[string]any{"name": "Matrix"} },
			want: cells{403, 403, 403, 403, 201, 401},
		},
		{
			name: "team rename", method: http.MethodPatch,
			path: func(_ *testing.T, f *fixture, _ *store.User) string {
				return teamPath("/" + strconv.FormatUint(uint64(f.TeamA.ID), 10))
			},
			body: func(*fixture) any { return map[string]any{"name": "Matrix"} },
			want: cells{403, 403, 403, 403, 200, 401},
		},
		{
			name: "team archive", method: http.MethodDelete,
			path: func(_ *testing.T, f *fixture, _ *store.User) string {
				return teamPath("/" + strconv.FormatUint(uint64(f.TeamA.ID), 10))
			},
			want: cells{403, 403, 403, 403, 204, 401},
		},
		{
			name: "team members replace", method: http.MethodPut,
			path: func(_ *testing.T, f *fixture, _ *store.User) string {
				return teamPath("/" + strconv.FormatUint(uint64(f.TeamA.ID), 10) + "/members")
			},
			body: func(f *fixture) any {
				return []map[string]any{{"user_id": f.Member.ID, "role": store.RoleMember}}
			},
			want: cells{403, 403, 403, 403, 200, 401},
		},
		{
			name: "users list", method: http.MethodGet,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/users" },
			want: cells{403, 403, 403, 403, 200, 401},
		},
		{
			name: "user create", method: http.MethodPost,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/users" },
			body: func(*fixture) any {
				return map[string]any{"email": "matrix@example.com", "name": "Matrix", "timezone": "UTC"}
			},
			want: cells{403, 403, 403, 403, 201, 401},
		},
		{
			name: "user update", method: http.MethodPatch,
			path: func(_ *testing.T, f *fixture, _ *store.User) string {
				return apiPrefix + "/users/" + strconv.FormatUint(uint64(f.Member.ID), 10)
			},
			body: func(*fixture) any { return map[string]any{"name": "Matrix"} },
			want: cells{403, 403, 403, 403, 200, 401},
		},
		{
			name: "user password reset", method: http.MethodPost,
			path: func(_ *testing.T, f *fixture, _ *store.User) string {
				return apiPrefix + "/users/" + strconv.FormatUint(uint64(f.Member.ID), 10) + "/password"
			},
			want: cells{403, 403, 403, 403, 200, 401},
		},
		{
			// A member manages their own tokens, so every identity gets its own list.
			name: "me tokens list", method: http.MethodGet,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/me/tokens" },
			want: cells{200, 200, 200, 200, 200, 401},
		},
		{
			name: "me token create", method: http.MethodPost,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/me/tokens" },
			body: func(*fixture) any { return map[string]any{"name": "matrix"} },
			want: cells{201, 201, 201, 201, 201, 401},
		},
		{
			name: "me token revoke", method: http.MethodDelete,
			path: func(t *testing.T, f *fixture, u *store.User) string {
				// Anonymous is refused before any lookup, so id 1 only has to be a path.
				id := uint(1)
				if u != nil {
					tok := &store.APIToken{UserID: u.ID, Name: "matrix", TokenHash: fmt.Sprintf("matrix-hash-%d", u.ID)}
					if err := f.st.CreateAPIToken(tok); err != nil {
						t.Fatalf("create token for %s: %v", u.Email, err)
					}
					id = tok.ID
				}
				return tokenPath(id)
			},
			want: cells{204, 204, 204, 204, 204, 401},
		},
		{
			name: "me password change", method: http.MethodPost,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/me/password" },
			body: func(*fixture) any {
				return map[string]any{
					"current_password": fixturePassword,
					"new_password":     "a longer password",
				}
			},
			want: cells{204, 204, 204, 204, 204, 401},
		},
		{
			name: "me", method: http.MethodGet,
			path: func(*testing.T, *fixture, *store.User) string { return apiPrefix + "/me" },
			want: cells{200, 200, 200, 200, 200, 401},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := seed(t)
			for _, c := range []struct {
				name string
				user *store.User
				want int
			}{
				{"member", f.Member, tc.want.member},
				{"lead of the team", f.LeadA, tc.want.lead},
				{"lead of another team", f.LeadB, tc.want.otherLead},
				{"outsider", f.Outsider, tc.want.outsider},
				{"admin", f.Admin, tc.want.admin},
			} {
				path := tc.path(t, f, c.user)
				rec := f.do(tc.method, path, tc.bodyFor(f), matrixOrigin, f.as(t, c.user))
				checkStatus(t, c.name, tc.method+" "+path, rec, c.want)
			}

			path := tc.path(t, f, nil)
			rec := f.do(tc.method, path, tc.bodyFor(f), matrixOrigin, "")
			checkStatus(t, "anonymous", tc.method+" "+path, rec, tc.want.anon)
		})
	}
}

// checkStatus asserts the exact code, and for a 403 the role refusal the Roles
// table encodes. The rest of the body belongs to the endpoint's own tests.
func checkStatus(t *testing.T, identity, endpoint string, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Errorf("%s %s = %d, want %d: %s", identity, endpoint, rec.Code, want, rec.Body)
		return
	}
	if want != http.StatusForbidden {
		return
	}
	body := decode[struct {
		Type string `json:"type"`
	}](t, rec)
	if body.Type != urnForbidden {
		t.Errorf("%s %s type = %q, want %q", identity, endpoint, body.Type, urnForbidden)
	}
}

// TestOpenAPIListsStage2Routes is the spec's guard against a forgotten Huma
// registration: the document must list every route the HTTP API table names.
func TestOpenAPIListsStage2Routes(t *testing.T) {
	rec := get(t, apiPrefix+"/openapi.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi status = %d, want 200", rec.Code)
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}

	routes := []struct{ method, path string }{
		{http.MethodPost, apiPrefix + "/auth/login"},
		{http.MethodPost, apiPrefix + "/auth/logout"},
		{http.MethodGet, apiPrefix + "/me"},
		{http.MethodPost, apiPrefix + "/me/password"},
		{http.MethodGet, apiPrefix + "/me/tokens"},
		{http.MethodPost, apiPrefix + "/me/tokens"},
		{http.MethodDelete, apiPrefix + "/me/tokens/{id}"},
		{http.MethodGet, apiPrefix + "/users"},
		{http.MethodPost, apiPrefix + "/users"},
		{http.MethodPatch, apiPrefix + "/users/{id}"},
		{http.MethodPost, apiPrefix + "/users/{id}/password"},
		{http.MethodGet, apiPrefix + "/teams"},
		{http.MethodPost, apiPrefix + "/teams"},
		{http.MethodPatch, apiPrefix + "/teams/{id}"},
		{http.MethodDelete, apiPrefix + "/teams/{id}"},
		{http.MethodPut, apiPrefix + "/teams/{id}/members"},
		{http.MethodGet, apiPrefix + "/projects"},
		{http.MethodPost, apiPrefix + "/projects"},
		{http.MethodPatch, apiPrefix + "/projects/{id}"},
		{http.MethodPost, apiPrefix + "/projects/{id}/restore"},
	}
	for _, r := range routes {
		if _, ok := doc.Paths[r.path][strings.ToLower(r.method)]; !ok {
			paths := make([]string, 0, len(doc.Paths))
			for p := range doc.Paths {
				paths = append(paths, p)
			}
			t.Errorf("openapi.json has no %s %s (paths: %s)", r.method, r.path, strings.Join(paths, " "))
		}
	}
}
