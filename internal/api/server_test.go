package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestNotFound(t *testing.T) {
	if rec := get(t, "/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
