// Package api wires the HTTP surface: Huma operations over net/http, the session
// middleware, the generated OpenAPI contract, and /healthz.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/havspect/project-times/internal/clock"
	"github.com/havspect/project-times/internal/store"
	"github.com/havspect/project-times/internal/version"
)

const (
	apiPrefix   = "/api/v1"
	openAPIPath = apiPrefix + "/openapi"
)

// Server is the state the handlers share: the store, the injected clock, and the
// in-memory login limiter.
type Server struct {
	Store *store.Store
	Clock clock.Clock
	limit loginLimiter
}

// New returns the http.Handler for the whole API and web surface.
func New(st *store.Store, clk clock.Clock) http.Handler {
	mux := http.NewServeMux()

	cfg := huma.DefaultConfig("Project Times", version.Version)
	cfg.OpenAPIPath = openAPIPath
	cfg.DocsPath = apiPrefix + "/docs"
	cfg.SchemasPath = ""
	// The default hook injects $schema and a Link header into every response body.
	cfg.CreateHooks = nil
	api := humago.New(mux, cfg)

	// The document advertises both ways to authenticate; withRequest enforces them.
	doc := api.OpenAPI()
	doc.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearerAuth": {
			Type: "http", Scheme: "bearer", BearerFormat: "ts_",
			Description: "Personal API token, shown once when it is created.",
		},
		"sessionCookie": {
			Type: "apiKey", In: "cookie", Name: sessionCookie,
			Description: "Session cookie set by POST /auth/login.",
		},
	}
	doc.Security = []map[string][]string{{"bearerAuth": {}}, {"sessionCookie": {}}}

	mux.HandleFunc("GET /healthz", health)

	s := &Server{Store: st, Clock: clk}
	s.authRoutes(api)
	s.projectRoutes(api)
	s.teamRoutes(api)
	s.tokenRoutes(api)
	s.userRoutes(api)
	return s.withRequest(mux)
}

// Spec returns the generated OpenAPI document, for `times openapi`.
func Spec() ([]byte, error) {
	rec := httptest.NewRecorder()
	New(nil, clock.System{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, openAPIPath+".json", nil))
	if rec.Code != http.StatusOK {
		return nil, fmt.Errorf("openapi: status %d", rec.Code)
	}
	return rec.Body.Bytes(), nil
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Nothing to do if the write fails: the status and headers are already sent.
	_ = json.NewEncoder(w).Encode(struct {
		Status        string `json:"status"`
		SchemaVersion int    `json:"schema_version"`
	}{"ok", store.SchemaVersion})
}
