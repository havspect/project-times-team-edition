package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/store"
)

// browsed is the Origin header a cookie-authenticated mutation needs; httptest
// requests are addressed to example.com.
var browsed = map[string]string{"Origin": "http://example.com"}

// sendAs sends a request as u the way the web client does, Origin included.
func sendAs(t *testing.T, f *fixture, u *store.User, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(method, path, body, browsed, f.as(t, u))
}

type problemBody struct {
	Type   string `json:"type"`
	Status int    `json:"status"`
}

func wantProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, urn string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	if urn == "" {
		return
	}
	if body := decode[problemBody](t, rec); body.Type != urn || body.Status != status {
		t.Fatalf("problem = %+v, want %s with status %d", body, urn, status)
	}
}

func TestUserRoutesPermissions(t *testing.T) {
	routes := []struct {
		name        string
		method      string
		path        func(*fixture) string
		body        any
		adminStatus int
	}{
		{"list", http.MethodGet, func(*fixture) string { return "/api/v1/users" }, nil, http.StatusOK},
		{"create", http.MethodPost, func(*fixture) string { return "/api/v1/users" },
			map[string]any{"email": "new@example.com", "name": "New Person", "timezone": "UTC"}, http.StatusCreated},
		{"update", http.MethodPatch, func(f *fixture) string { return fmt.Sprintf("/api/v1/users/%d", f.Outsider.ID) },
			map[string]any{"name": "Renamed"}, http.StatusOK},
		{"reset", http.MethodPost, func(f *fixture) string { return fmt.Sprintf("/api/v1/users/%d/password", f.Outsider.ID) },
			nil, http.StatusOK},
	}

	for _, tt := range routes {
		t.Run(tt.name, func(t *testing.T) {
			for _, actor := range []string{"member", "lead", "outsider"} {
				t.Run(actor, func(t *testing.T) {
					f := seed(t)
					u := f.Member
					switch actor {
					case "lead":
						u = f.LeadA
					case "outsider":
						u = f.Outsider
					}
					rec := sendAs(t, f, u, tt.method, tt.path(f), tt.body)
					wantProblem(t, rec, http.StatusForbidden, urnForbidden)
				})
			}

			t.Run("anonymous", func(t *testing.T) {
				f := seed(t)
				rec := f.do(tt.method, tt.path(f), tt.body, nil, "")
				wantProblem(t, rec, http.StatusUnauthorized, urnUnauthorized)
			})

			t.Run("admin", func(t *testing.T) {
				f := seed(t)
				rec := sendAs(t, f, f.Admin, tt.method, tt.path(f), tt.body)
				if rec.Code != tt.adminStatus {
					t.Fatalf("status = %d, want %d: %s", rec.Code, tt.adminStatus, rec.Body)
				}
			})
		})
	}
}

func TestListUsers(t *testing.T) {
	f := seed(t)
	rec := f.doAs(t, f.Admin, http.MethodGet, "/api/v1/users", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}

	users := decode[[]userResponse](t, rec)
	want := []string{"ada@example.com", "grace@example.com", "hedy@example.com", "linus@example.com", "margaret@example.com"}
	if len(users) != len(want) {
		t.Fatalf("got %d users, want %d: %+v", len(users), len(want), users)
	}
	for i, email := range want {
		if users[i].Email != email {
			t.Errorf("users[%d].Email = %q, want %q", i, users[i].Email, email)
		}
	}
	if !users[0].IsAdmin || !users[0].Active || users[0].MustChangePassword {
		t.Fatalf("seeded admin = %+v, want an active admin without the password flag", users[0])
	}
	if users[0].Timezone != "UTC" {
		t.Fatalf("timezone = %q, want UTC", users[0].Timezone)
	}
}

func TestCreateUser(t *testing.T) {
	f := seed(t)
	rec := sendAs(t, f, f.Admin, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "New@Example.com", "name": "New Person", "timezone": "Europe/Berlin",
	})
	wantStatus := http.StatusCreated
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d: %s", rec.Code, wantStatus, rec.Body)
	}
	created := decode[createdUser](t, rec)
	if created.Email != "new@example.com" {
		t.Fatalf("email = %q, want it lowercased", created.Email)
	}
	if created.Name != "New Person" || created.Timezone != "Europe/Berlin" {
		t.Fatalf("created = %+v, want the submitted name and timezone", created)
	}
	if created.IsAdmin || !created.Active || !created.MustChangePassword {
		t.Fatalf("created = %+v, want a non-admin, active, must-change user", created)
	}
	if created.InitialPassword == "" {
		t.Fatal("initial_password is empty")
	}

	// The initial password logs in, and the account is confined until it changes.
	login := f.login(created.Email, created.InitialPassword)
	if login.Code != http.StatusOK {
		t.Fatalf("login with the initial password = %d, want 200: %s", login.Code, login.Body)
	}
	cookie := sessionOf(t, login).Value

	rec = f.do(http.MethodGet, "/api/v1/users", nil, nil, cookie)
	wantProblem(t, rec, http.StatusForbidden, urnPasswordChangeRequired)
	if rec = f.do(http.MethodGet, "/api/v1/me", nil, nil, cookie); rec.Code != http.StatusOK {
		t.Fatalf("GET /me while confined = %d, want 200: %s", rec.Code, rec.Body)
	}

	rec = f.do(http.MethodPost, "/api/v1/me/password", map[string]string{
		"current_password": created.InitialPassword,
		"new_password":     "a brand new password",
	}, browsed, cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("password change = %d, want 204: %s", rec.Code, rec.Body)
	}

	// Confinement is gone: the same session is refused as a plain member, not as
	// someone who must change their password.
	rec = f.do(http.MethodGet, "/api/v1/users", nil, nil, cookie)
	wantProblem(t, rec, http.StatusForbidden, urnForbidden)
}

func TestCreateUserAdminFlag(t *testing.T) {
	f := seed(t)
	rec := sendAs(t, f, f.Admin, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "boss@example.com", "name": "New Boss", "timezone": "UTC", "is_admin": true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body)
	}
	created := decode[createdUser](t, rec)
	if !created.IsAdmin {
		t.Fatalf("is_admin = false, want true")
	}

	users := decode[[]userResponse](t, sendAs(t, f, f.Admin, http.MethodGet, "/api/v1/users", nil))
	var found bool
	for _, u := range users {
		if u.ID == created.ID && u.IsAdmin {
			found = true
		}
	}
	if !found {
		t.Fatalf("created admin not listed: %+v", users)
	}
}

func TestCreateUserValidation(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
		urn  string
	}{
		{
			"invalid timezone",
			map[string]any{"email": "new@example.com", "name": "New Person", "timezone": "Mars/Olympus"},
			urnInvalidInput,
		},
		{
			"empty name",
			map[string]any{"email": "new@example.com", "name": "", "timezone": "UTC"},
			"",
		},
		{
			"duplicate email",
			map[string]any{"email": "Linus@Example.com", "name": "New Person", "timezone": "UTC"},
			urnInvalidInput,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := seed(t)
			rec := sendAs(t, f, f.Admin, http.MethodPost, "/api/v1/users", tt.body)
			wantProblem(t, rec, http.StatusUnprocessableEntity, tt.urn)
		})
	}
}

func TestUpdateUser(t *testing.T) {
	f := seed(t)

	rec := sendAs(t, f, f.Admin, http.MethodPatch, fmt.Sprintf("/api/v1/users/%d", f.Outsider.ID),
		map[string]any{"name": "Maggie", "timezone": "America/New_York"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	updated := decode[userResponse](t, rec)
	if updated.Name != "Maggie" || updated.Timezone != "America/New_York" {
		t.Fatalf("updated = %+v, want the new name and timezone", updated)
	}
	if !updated.Active || updated.IsAdmin {
		t.Fatalf("updated = %+v, want the flags untouched", updated)
	}

	got, err := f.st.UserByID(f.Outsider.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Name != "Maggie" || got.Timezone != "America/New_York" {
		t.Fatalf("stored = %+v, want the patch applied", got)
	}
}

func TestUpdateUserErrors(t *testing.T) {
	t.Run("unknown id", func(t *testing.T) {
		f := seed(t)
		rec := sendAs(t, f, f.Admin, http.MethodPatch, "/api/v1/users/9999", map[string]any{"name": "Ghost"})
		wantProblem(t, rec, http.StatusNotFound, urnNotFound)
	})

	t.Run("invalid timezone", func(t *testing.T) {
		f := seed(t)
		rec := sendAs(t, f, f.Admin, http.MethodPatch, fmt.Sprintf("/api/v1/users/%d", f.Outsider.ID),
			map[string]any{"timezone": "Mars/Olympus"})
		wantProblem(t, rec, http.StatusUnprocessableEntity, urnInvalidInput)
		got, err := f.st.UserByID(f.Outsider.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.Timezone != "UTC" {
			t.Fatalf("timezone = %q, want it untouched", got.Timezone)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		f := seed(t)
		rec := sendAs(t, f, f.Admin, http.MethodPatch, fmt.Sprintf("/api/v1/users/%d", f.Outsider.ID),
			map[string]any{"name": ""})
		wantProblem(t, rec, http.StatusUnprocessableEntity, "")
	})
}

func TestDeactivationEndsSession(t *testing.T) {
	f := seed(t)
	cookie := f.as(t, f.Outsider)
	if rec := f.do(http.MethodGet, "/api/v1/me", nil, nil, cookie); rec.Code != http.StatusOK {
		t.Fatalf("status before deactivation = %d, want 200", rec.Code)
	}

	rec := sendAs(t, f, f.Admin, http.MethodPatch, fmt.Sprintf("/api/v1/users/%d", f.Outsider.ID),
		map[string]any{"active": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if decode[userResponse](t, rec).Active {
		t.Fatal("active = true after deactivation")
	}

	wantProblem(t, f.do(http.MethodGet, "/api/v1/me", nil, nil, cookie), http.StatusUnauthorized, urnUnauthorized)
}

func TestLastAdminIsProtected(t *testing.T) {
	f := seed(t)
	path := fmt.Sprintf("/api/v1/users/%d", f.Admin.ID)

	for name, body := range map[string]map[string]any{
		"demote":     {"is_admin": false},
		"deactivate": {"active": false},
	} {
		t.Run(name, func(t *testing.T) {
			wantProblem(t, sendAs(t, f, f.Admin, http.MethodPatch, path, body),
				http.StatusUnprocessableEntity, urnLastAdmin)
			got, err := f.st.UserByID(f.Admin.ID)
			if err != nil {
				t.Fatalf("reload: %v", err)
			}
			if !got.IsAdmin || !got.Active {
				t.Fatalf("last admin = %+v, want it untouched", got)
			}
		})
	}
}

func TestLastAdminAllowsChangesWithSecondAdmin(t *testing.T) {
	t.Run("demote", func(t *testing.T) {
		f := seed(t)
		sendAs(t, f, f.Admin, http.MethodPost, "/api/v1/users", map[string]any{
			"email": "second@example.com", "name": "Second Admin", "timezone": "UTC", "is_admin": true,
		})
		rec := sendAs(t, f, f.Admin, http.MethodPatch, fmt.Sprintf("/api/v1/users/%d", f.Admin.ID),
			map[string]any{"is_admin": false})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
		}
		if decode[userResponse](t, rec).IsAdmin {
			t.Fatal("is_admin = true after demotion")
		}
	})

	t.Run("deactivate", func(t *testing.T) {
		f := seed(t)
		sendAs(t, f, f.Admin, http.MethodPost, "/api/v1/users", map[string]any{
			"email": "second@example.com", "name": "Second Admin", "timezone": "UTC", "is_admin": true,
		})
		rec := sendAs(t, f, f.Admin, http.MethodPatch, fmt.Sprintf("/api/v1/users/%d", f.Admin.ID),
			map[string]any{"active": false})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
		}
		if decode[userResponse](t, rec).Active {
			t.Fatal("active = true after deactivation")
		}
	})
}

func TestResetUserPassword(t *testing.T) {
	f := seed(t)
	session := f.as(t, f.Member)
	rec := sendAs(t, f, f.Admin, http.MethodPost, fmt.Sprintf("/api/v1/users/%d/password", f.Member.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	password := decode[struct {
		InitialPassword string `json:"initial_password"`
	}](t, rec).InitialPassword
	if password == "" || password == fixturePassword {
		t.Fatalf("initial_password = %q, want a fresh value", password)
	}

	got, err := f.st.UserByID(f.Member.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !got.MustChangePassword {
		t.Fatal("must_change_password = false after a reset")
	}

	// The reset ends every login the old password could reach, rather than only
	// confining them: a deleted session answers 401, not 403.
	if rec := f.do(http.MethodGet, "/api/v1/me", nil, nil, session); rec.Code != http.StatusUnauthorized {
		t.Fatalf("session after the reset = %d, want 401: %s", rec.Code, rec.Body)
	}

	if rec := f.login(f.Member.Email, fixturePassword); rec.Code != http.StatusUnauthorized {
		t.Fatalf("login with the old password = %d, want 401: %s", rec.Code, rec.Body)
	}
	if rec := f.login(f.Member.Email, password); rec.Code != http.StatusOK {
		t.Fatalf("login with the initial password = %d, want 200: %s", rec.Code, rec.Body)
	}
}

func TestResetUserPasswordUnknown(t *testing.T) {
	f := seed(t)
	rec := sendAs(t, f, f.Admin, http.MethodPost, "/api/v1/users/9999/password", nil)
	wantProblem(t, rec, http.StatusNotFound, urnNotFound)
}

func TestChangeOwnPassword(t *testing.T) {
	f := seed(t)
	cookie := f.as(t, f.Member)
	other := f.as(t, f.Member)
	token, hash, err := auth.NewAPIToken()
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	if err := f.st.CreateAPIToken(&store.APIToken{UserID: f.Member.ID, Name: "cli", TokenHash: hash}); err != nil {
		t.Fatalf("create token: %v", err)
	}
	before, err := f.st.UserByID(f.Member.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	rec := f.do(http.MethodPost, "/api/v1/me/password", map[string]string{
		"current_password": "wrong password", "new_password": "a brand new password",
	}, browsed, cookie)
	wantProblem(t, rec, http.StatusUnprocessableEntity, urnInvalidInput)

	// Too short: huma rejects it before the handler runs.
	rec = f.do(http.MethodPost, "/api/v1/me/password", map[string]string{
		"current_password": fixturePassword, "new_password": "short",
	}, browsed, cookie)
	wantProblem(t, rec, http.StatusUnprocessableEntity, "")

	after, err := f.st.UserByID(f.Member.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.PasswordHash != before.PasswordHash {
		t.Fatal("hash changed after a rejected attempt")
	}

	rec = f.do(http.MethodPost, "/api/v1/me/password", map[string]string{
		"current_password": fixturePassword, "new_password": "a brand new password",
	}, browsed, cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body)
	}

	if rec := f.login(f.Member.Email, fixturePassword); rec.Code != http.StatusUnauthorized {
		t.Fatalf("login with the old password = %d, want 401: %s", rec.Code, rec.Body)
	}
	if rec := f.login(f.Member.Email, "a brand new password"); rec.Code != http.StatusOK {
		t.Fatalf("login with the new password = %d, want 200: %s", rec.Code, rec.Body)
	}

	changed, err := f.st.UserByID(f.Member.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if changed.MustChangePassword {
		t.Fatal("must_change_password = true after changing it")
	}

	// The session doing the change survives; every other login the old password
	// could reach is dead.
	if rec := f.do(http.MethodGet, "/api/v1/me", nil, nil, cookie); rec.Code != http.StatusOK {
		t.Fatalf("caller session after the change = %d, want 200: %s", rec.Code, rec.Body)
	}
	if rec := f.do(http.MethodGet, "/api/v1/me", nil, nil, other); rec.Code != http.StatusUnauthorized {
		t.Fatalf("other session after the change = %d, want 401: %s", rec.Code, rec.Body)
	}
	if rec := f.do(http.MethodGet, "/api/v1/me", nil, map[string]string{"Authorization": "Bearer " + token}, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token after the change = %d, want 401: %s", rec.Code, rec.Body)
	}
}

func TestChangePasswordAnonymous(t *testing.T) {
	f := seed(t)
	rec := f.do(http.MethodPost, "/api/v1/me/password", map[string]string{
		"current_password": "x", "new_password": "a brand new password",
	}, nil, "")
	wantProblem(t, rec, http.StatusUnauthorized, urnUnauthorized)
}

func TestSpecListsUserRoutes(t *testing.T) {
	rec := get(t, "/api/v1/openapi.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var doc struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []struct {
		method string
		path   string
	}{
		{"get", "/api/v1/users"},
		{"post", "/api/v1/users"},
		{"patch", "/api/v1/users/{id}"},
		{"post", "/api/v1/users/{id}/password"},
		{"post", "/api/v1/me/password"},
	}
	for _, w := range want {
		if doc.Paths[w.path][w.method] == nil {
			t.Errorf("spec is missing %s %s", strings.ToUpper(w.method), w.path)
		}
	}
}
