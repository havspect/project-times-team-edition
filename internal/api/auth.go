package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/store"
)

// dummyPasswordHash is verified when the email is unknown, so login costs one
// Argon2 run either way and timing does not reveal which emails exist.
const dummyPasswordHash = "$argon2id$v=19$m=65536,t=1,p=4$G1Xfy5Q8RzQI9T1v8nJHpQ$tU9jHulZdbMxUZLvpPk+rtjRCEyDSaFEf6OIH7lWLPs"

type meResponse struct {
	ID       uint   `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	IsAdmin  bool   `json:"is_admin"`
}

func (s *Server) routes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodPost,
		Path:        apiPrefix + "/auth/login",
		Tags:        []string{"auth"},
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity},
	}, s.login)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-logout",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/auth/logout",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden},
	}, s.logout)

	huma.Register(api, huma.Operation{
		OperationID: "me",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/me",
		Tags:        []string{"me"},
		Errors:      []int{http.StatusUnauthorized},
	}, s.me)
}

type loginInput struct {
	Body struct {
		Email    string `json:"email" required:"true" format:"email"`
		Password string `json:"password" required:"true" minLength:"1"`
	}
}

type loginOutput struct {
	SetCookie string `header:"Set-Cookie"`
	Body      meResponse
}

func (s *Server) login(ctx context.Context, in *loginInput) (*loginOutput, error) {
	u, lookupErr := s.Store.UserByEmail(in.Body.Email)
	passwordHash := dummyPasswordHash
	if lookupErr == nil {
		passwordHash = u.PasswordHash
	}
	ok := auth.VerifyPassword(passwordHash, in.Body.Password)
	if lookupErr != nil || !ok || !u.Active {
		return nil, problem(http.StatusUnauthorized, urnInvalidCredentials, "email or password is incorrect")
	}

	token, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		return nil, err
	}
	expires := s.Clock.Now().Add(auth.SessionTTL)
	if err := s.Store.CreateSession(&store.Session{UserID: u.ID, TokenHash: tokenHash, ExpiresAt: expires}); err != nil {
		return nil, err
	}
	return &loginOutput{
		SetCookie: cookie(sessionCookie, token, expires, requestFrom(ctx).Request),
		Body:      meOf(u),
	}, nil
}

type logoutOutput struct {
	SetCookie string `header:"Set-Cookie"`
}

func (s *Server) logout(ctx context.Context, _ *struct{}) (*logoutOutput, error) {
	ri := requestFrom(ctx)
	if ri == nil || ri.User == nil {
		return nil, problem(http.StatusUnauthorized, urnUnauthorized, "authentication required")
	}
	if ri.Session != nil {
		if err := s.Store.DeleteSession(ri.Session.ID); err != nil {
			return nil, err
		}
	}
	clear := &http.Cookie{
		Name: sessionCookie, Path: "/", MaxAge: -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(ri.Request),
	}
	return &logoutOutput{SetCookie: clear.String()}, nil
}

type meOutput struct{ Body meResponse }

func (s *Server) me(ctx context.Context, _ *struct{}) (*meOutput, error) {
	u := principal(ctx)
	if u == nil {
		return nil, problem(http.StatusUnauthorized, urnUnauthorized, "authentication required")
	}
	return &meOutput{Body: meOf(u)}, nil
}

func meOf(u *store.User) meResponse {
	return meResponse{ID: u.ID, Email: u.Email, Name: u.Name, Timezone: u.Timezone, IsAdmin: u.IsAdmin}
}

func cookie(name, value string, expires time.Time, r *http.Request) string {
	return (&http.Cookie{
		Name: name, Value: value, Path: "/",
		MaxAge:   int(auth.SessionTTL / time.Second),
		Expires:  expires.UTC(),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(r),
	}).String()
}
