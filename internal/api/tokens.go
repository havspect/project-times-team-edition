package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/tstamps/internal/auth"
	"github.com/havspect/tstamps/internal/store"
)

// tokenResponse is a token without its hash: the raw value appears only in the
// reply that creates it.
type tokenResponse struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// tokenRoutes registers the personal API token endpoints.
func (s *Server) tokenRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "tokens-list",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/me/tokens",
		Tags:        []string{"tokens"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, s.listTokens)

	huma.Register(api, huma.Operation{
		OperationID:   "tokens-create",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/me/tokens",
		Tags:          []string{"tokens"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.createToken)

	huma.Register(api, huma.Operation{
		OperationID:   "tokens-revoke",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/me/tokens/{id}",
		Tags:          []string{"tokens"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError},
	}, s.revokeToken)
}

type tokenListOutput struct{ Body []tokenResponse }

func (s *Server) listTokens(ctx context.Context, _ *struct{}) (*tokenListOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.APITokensOf(u.ID)
	if err != nil {
		return nil, internal(err)
	}
	out := &tokenListOutput{Body: make([]tokenResponse, 0, len(rows))}
	for _, t := range rows {
		out.Body = append(out.Body, tokenView(t))
	}
	return out, nil
}

type createTokenInput struct {
	Body struct {
		Name string `json:"name" required:"true" minLength:"1"`
	}
}

type createTokenOutput struct {
	Body struct {
		ID    uint   `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
	}
}

func (s *Server) createToken(ctx context.Context, in *createTokenInput) (*createTokenOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	raw, hash, err := auth.NewAPIToken()
	if err != nil {
		return nil, internal(err)
	}
	t := &store.APIToken{UserID: u.ID, Name: in.Body.Name, TokenHash: hash}
	if err := s.Store.CreateAPIToken(t); err != nil {
		return nil, internal(err)
	}
	out := &createTokenOutput{}
	out.Body.ID, out.Body.Name, out.Body.Token = t.ID, t.Name, raw
	return out, nil
}

type revokeTokenInput struct {
	ID uint `path:"id"`
}

type revokeTokenOutput struct{}

// revokeToken looks the id up among the caller's own tokens, so another user's id
// is a 404 rather than a 403 that would confirm it exists.
func (s *Server) revokeToken(ctx context.Context, in *revokeTokenInput) (*revokeTokenOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.APITokensOf(u.ID)
	if err != nil {
		return nil, internal(err)
	}
	for _, t := range rows {
		if t.ID != in.ID {
			continue
		}
		if t.RevokedAt == nil {
			if err := s.Store.RevokeAPIToken(t.ID, s.Clock.Now()); err != nil {
				return nil, internal(err)
			}
		}
		return &revokeTokenOutput{}, nil
	}
	return nil, problem(http.StatusNotFound, urnNotFound, "token not found")
}

func tokenView(t store.APIToken) tokenResponse {
	return tokenResponse{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt, RevokedAt: t.RevokedAt}
}
