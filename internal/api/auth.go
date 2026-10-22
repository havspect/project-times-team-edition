package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/store"
)

// dummyPasswordHash is verified when the email is unknown, so login costs one
// Argon2 run either way and timing does not reveal which emails exist.
const dummyPasswordHash = "$argon2id$v=19$m=65536,t=1,p=4$G1Xfy5Q8RzQI9T1v8nJHpQ$tU9jHulZdbMxUZLvpPk+rtjRCEyDSaFEf6OIH7lWLPs"

type membershipResponse struct {
	TeamID uint   `json:"team_id"`
	Team   string `json:"team"`
	Role   string `json:"role"`
}

type meResponse struct {
	ID          uint                 `json:"id"`
	Email       string               `json:"email"`
	Name        string               `json:"name"`
	Timezone    string               `json:"timezone"`
	IsAdmin     bool                 `json:"is_admin"`
	Memberships []membershipResponse `json:"memberships"`
}

func (s *Server) authRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodPost,
		Path:        apiPrefix + "/auth/login",
		Tags:        []string{"auth"},
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusTooManyRequests, http.StatusInternalServerError},
	}, s.login)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-logout",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/auth/logout",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError},
	}, s.logout)

	huma.Register(api, huma.Operation{
		OperationID: "me",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/me",
		Tags:        []string{"me"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
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
	r := requestFrom(ctx).Request
	if !s.limit.allow(clientIP(r), s.Clock.Now()) {
		return nil, problem(http.StatusTooManyRequests, urnRateLimited, "too many login attempts, try again later")
	}

	u, err := s.Store.UserByEmail(in.Body.Email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		// Unknown email: keep going so one Argon2 run happens either way.
	case err != nil:
		return nil, internal(err)
	}
	passwordHash := dummyPasswordHash
	if u != nil {
		passwordHash = u.PasswordHash
	}
	ok := auth.VerifyPassword(passwordHash, in.Body.Password)
	if u == nil || !ok || !u.Active {
		return nil, problem(http.StatusUnauthorized, urnInvalidCredentials, "email or password is incorrect")
	}

	token, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		return nil, internal(err)
	}
	expires := s.Clock.Now().Add(auth.SessionTTL)
	if err := s.Store.CreateSession(&store.Session{UserID: u.ID, TokenHash: tokenHash, ExpiresAt: expires}); err != nil {
		return nil, internal(err)
	}
	body, err := s.meOf(u)
	if err != nil {
		return nil, internal(err)
	}
	return &loginOutput{
		SetCookie: cookie(sessionCookieName(r), token, expires, r),
		Body:      body,
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
			return nil, internal(err)
		}
	}
	return &logoutOutput{SetCookie: clearCookie(ri.CookieName, ri.Request).String()}, nil
}

type meOutput struct{ Body meResponse }

func (s *Server) me(ctx context.Context, _ *struct{}) (*meOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	body, err := s.meOf(u)
	if err != nil {
		return nil, internal(err)
	}
	return &meOutput{Body: body}, nil
}

func (s *Server) meOf(u *store.User) (meResponse, error) {
	out := meResponse{
		ID: u.ID, Email: u.Email, Name: u.Name, Timezone: u.Timezone, IsAdmin: u.IsAdmin,
		Memberships: []membershipResponse{},
	}
	ms, err := s.Store.MembershipsOf(u.ID)
	if err != nil {
		return meResponse{}, err
	}
	for _, m := range ms {
		out.Memberships = append(out.Memberships, membershipResponse{TeamID: m.TeamID, Team: m.Team, Role: m.Role})
	}
	return out, nil
}

func cookie(name, value string, expires time.Time, r *http.Request) string {
	return (&http.Cookie{
		Name: name, Value: value, Path: "/",
		MaxAge:   int(auth.SessionTTL / time.Second),
		Expires:  expires.UTC(),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(r),
	}).String()
}
