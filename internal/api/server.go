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

	"github.com/havspect/tstamps/internal/clock"
	"github.com/havspect/tstamps/internal/entry"
	"github.com/havspect/tstamps/internal/store"
	"github.com/havspect/tstamps/internal/version"
	"github.com/havspect/tstamps/web"
)

const (
	apiPrefix   = "/api/v1"
	openAPIPath = apiPrefix + "/openapi"
)

// Server is the state the handlers share: the store, the entry service, the
// injected clock, and the in-memory login limiter.
type Server struct {
	Store   *store.Store
	Clock   clock.Clock
	entries *entry.Service
	limit   loginLimiter
}

// New returns the http.Handler for the whole API and web surface.
func New(st *store.Store, clk clock.Clock) http.Handler {
	apiMux := http.NewServeMux()

	cfg := huma.DefaultConfig("Tstamps", version.Version)
	cfg.OpenAPIPath = openAPIPath
	cfg.DocsPath = apiPrefix + "/docs"
	cfg.SchemasPath = ""
	// The default hook injects $schema and a Link header into every response body.
	cfg.CreateHooks = nil
	api := humago.New(apiMux, cfg)

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

	s := &Server{Store: st, Clock: clk, entries: entry.New(st, clk)}
	s.authRoutes(api)
	s.entryRoutes(api)
	s.projectRoutes(api)
	s.reportRoutes(api)
	s.teamRoutes(api)
	s.timerRoutes(api)
	s.tokenRoutes(api)
	s.userRoutes(api)

	// An unknown path under /api/ is still an API client: answer in the same
	// problem+json shape rather than net/http's plain-text 404.
	apiMux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, problem(http.StatusNotFound, urnNotFound, "not found"))
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	// Only the API subtree needs a session: the SPA shell must load so a user
	// with must_change_password can reach the password screen.
	mux.Handle("/api/", s.withRequest(apiMux))
	mux.Handle("/", web.Handler())
	return mux
}

// Spec returns the generated OpenAPI document, for `tstamps openapi`.
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
