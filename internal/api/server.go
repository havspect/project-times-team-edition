package api

import (
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

type Server struct {
	Store *store.Store
	Clock clock.Clock
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

	mux.HandleFunc("GET /healthz", health)

	s := &Server{Store: st, Clock: clk}
	s.routes(api)
	return mux
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

// routes is where commit 5 registers its auth and API operations.
func (s *Server) routes(api huma.API) {}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","schema_version":%d}`, store.SchemaVersion)
}
