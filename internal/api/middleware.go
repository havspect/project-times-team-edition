package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/store"
)

const sessionCookie = "times_session"

type requestInfo struct {
	Request *http.Request
	User    *store.User    // nil when unauthenticated
	Session *store.Session // nil when unauthenticated
	Cookie  bool           // credentials came from the cookie
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
func (s *Server) withRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ri := &requestInfo{Request: r}
		if c, err := r.Cookie(sessionCookie); err == nil {
			if sess, err := s.Store.SessionByTokenHash(auth.HashToken(c.Value)); err == nil && s.Clock.Now().Before(sess.ExpiresAt) {
				if u, err := s.Store.UserByID(sess.UserID); err == nil && u.Active {
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
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func writeProblem(w http.ResponseWriter, err error) {
	var em *huma.ErrorModel
	if !errors.As(err, &em) {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(em.Status)
	_ = json.NewEncoder(w).Encode(em)
}
