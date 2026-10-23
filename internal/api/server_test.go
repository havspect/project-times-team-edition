package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/clock"
	"github.com/havspect/project-times/internal/store"
)

func handler() http.Handler { return New(nil, clock.System{}) }

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthz(t *testing.T) {
	rec := get(t, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type = %q, want application/json", ct)
	}
	var body struct {
		Status        string `json:"status"`
		SchemaVersion int    `json:"schema_version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" || body.SchemaVersion != store.SchemaVersion {
		t.Fatalf("body = %+v, want status ok and schema_version %d", body, store.SchemaVersion)
	}
}

type openAPI struct {
	OpenAPI string `json:"openapi"`
	Info    struct {
		Title string `json:"title"`
	} `json:"info"`
}

func TestOpenAPI(t *testing.T) {
	rec := get(t, "/api/v1/openapi.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var doc openAPI
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.1") {
		t.Fatalf("openapi = %q, want 3.1.x", doc.OpenAPI)
	}
	if doc.Info.Title != "Project Times" {
		t.Fatalf("info.title = %q, want Project Times", doc.Info.Title)
	}
}

func TestDocs(t *testing.T) {
	rec := get(t, "/api/v1/docs")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content type = %q, want text/html", ct)
	}
}

func TestSpecMatchesRoute(t *testing.T) {
	spec, err := Spec()
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if want := get(t, "/api/v1/openapi.json").Body.String(); string(spec) != want {
		t.Fatalf("Spec differs from route:\n%s\nwant:\n%s", spec, want)
	}
}

func TestUnknownAPIPathIsProblemJSON(t *testing.T) {
	rec := get(t, "/api/v1/unknown")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("content type = %q, want application/problem+json", ct)
	}
}

func TestWebServesTheAppShell(t *testing.T) {
	// The stub build answers every non-API path with the same page; the embedded
	// build answers index.html for both. Either way the shell must load.
	for _, path := range []string{"/", "/timeline"} {
		rec := get(t, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("%s content type = %q, want text/html", path, ct)
		}
	}
}

func TestMustChangePasswordLoadsTheAppButNotTheAPI(t *testing.T) {
	h := newHarness(t)
	hash, err := auth.HashPassword("initial password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &store.User{
		Email: "new@example.com", Name: "New Person", PasswordHash: hash,
		Timezone: "UTC", Active: true, MustChangePassword: true,
	}
	if err := h.st.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	c := h.as(t, u)
	if rec := h.do(http.MethodGet, "/", nil, nil, c); rec.Code != http.StatusOK {
		t.Fatalf("shell status = %d, want 200: %s", rec.Code, rec.Body)
	}
	wantProblem(t, h.do(http.MethodGet, apiPrefix+"/entries", nil, nil, c),
		http.StatusForbidden, urnPasswordChangeRequired)
}
