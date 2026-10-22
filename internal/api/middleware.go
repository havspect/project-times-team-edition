package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/store"
)

const (
	sessionCookie = "times_session"
	// The __Host- prefix makes browsers require https, Path=/ and no Domain;
	// plain http cannot set it, so the bare name is used there instead.
	secureSessionCookie = "__Host-" + sessionCookie
)

type requestInfo struct {
	Request    *http.Request
	User       *store.User    // nil when unauthenticated
	Session    *store.Session // nil when unauthenticated
	Cookie     bool           // credentials came from the cookie
	CookieName string         // which session cookie the request carried, for clearing it
}

type requestKey struct{}

func requestFrom(ctx context.Context) *requestInfo {
	ri, _ := ctx.Value(requestKey{}).(*requestInfo)
	return ri
}

func principal(ctx context.Context) *store.User {
	if ri := requestFrom(ctx); ri != nil {
		return ri.User
	}
	return nil
}

// withRequest resolves the session and enforces the CSRF rule for cookie-authenticated
// mutations; it never rejects an unauthenticated request, so unknown paths still 404.
// A database error is not an anonymous request, so it answers 500 instead.
func (s *Server) withRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ri := &requestInfo{Request: r}
		if name, value, ok := sessionToken(r); ok {
			ri.CookieName = name
			sess, err := s.Store.SessionByTokenHash(auth.HashToken(value))
			switch {
			case errors.Is(err, store.ErrNotFound):
				// Stale cookie: continue as anonymous.
			case err != nil:
				slog.Error("session lookup", "error", err)
				writeProblem(w, problem(http.StatusInternalServerError, urnInternal, "internal error"))
				return
			case !s.Clock.Now().Before(sess.ExpiresAt):
				s.expireSession(w, name, sess.ID, r)
			default:
				u, err := s.Store.UserByID(sess.UserID)
				switch {
				case errors.Is(err, store.ErrNotFound):
					// The user is gone: continue as anonymous.
				case err != nil:
					slog.Error("session user lookup", "error", err)
					writeProblem(w, problem(http.StatusInternalServerError, urnInternal, "internal error"))
					return
				case u.Active:
					ri.User, ri.Session, ri.Cookie = u, sess, true
				}
			}
		}
		// Bearer API tokens are stage 2; only the session cookie authenticates here.
		if ri.Cookie && !safeMethod(r.Method) && !sameOrigin(r) {
			writeProblem(w, problem(http.StatusForbidden, urnInvalidOrigin, "cross-origin request rejected"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestKey{}, ri)))
	})
}

// expireSession drops a session the clock has passed and asks the browser to forget
// the cookie; both are best effort so the request still proceeds as anonymous.
func (s *Server) expireSession(w http.ResponseWriter, name string, id uint, r *http.Request) {
	if err := s.Store.DeleteSession(id); err != nil {
		slog.Error("delete expired session", "error", err)
	}
	http.SetCookie(w, clearCookie(name, r))
}

// sessionToken returns the session cookie presented with r, preferring the secure name.
func sessionToken(r *http.Request) (name, value string, ok bool) {
	for _, n := range []string{secureSessionCookie, sessionCookie} {
		if c, err := r.Cookie(n); err == nil {
			return c.Name, c.Value, true
		}
	}
	return "", "", false
}

// sessionCookieName is the name a response should set: browsers reject __Host- over http.
func sessionCookieName(r *http.Request) string {
	if isHTTPS(r) {
		return secureSessionCookie
	}
	return sessionCookie
}

func clearCookie(name string, r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name: name, Path: "/", MaxAge: -1, Expires: time.Unix(0, 0),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(r),
	}
}

func safeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func sameOrigin(r *http.Request) bool {
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	return r.Header.Get("Origin") == scheme+"://"+r.Host
}

// shortcut: X-Forwarded-Proto is trusted as-is, upgrade when requests can reach the server without a proxy that sets it
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// Proxies append their own scheme, so only the first value is the client's.
	proto, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.TrimSpace(proto) == "https"
}

// writeProblem skips the encode error: the status and content type are already
// sent, and a failed write has nothing left to report to.
func writeProblem(w http.ResponseWriter, em *huma.ErrorModel) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(em.Status)
	_ = json.NewEncoder(w).Encode(em)
}
