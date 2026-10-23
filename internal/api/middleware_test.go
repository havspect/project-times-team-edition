package api

import (
	"net/http"
	"testing"

	"github.com/havspect/tstamps/internal/auth"
	"github.com/havspect/tstamps/internal/store"
)

func TestBearerTokenAuthenticatesAsItsUser(t *testing.T) {
	f := seed(t)
	token, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	apiToken := &store.APIToken{UserID: f.LeadA.ID, Name: "cli", TokenHash: hash}
	if err := f.st.CreateAPIToken(apiToken); err != nil {
		t.Fatalf("create token: %v", err)
	}

	rec := f.do(http.MethodGet, "/api/v1/me", nil, map[string]string{"Authorization": "Bearer " + token}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if me := decode[meResponse](t, rec); me.Email != f.LeadA.Email {
		t.Fatalf("email = %q, want %q", me.Email, f.LeadA.Email)
	}

	used, err := f.st.APITokenByTokenHash(hash)
	if err != nil {
		t.Fatalf("reload token: %v", err)
	}
	if used.LastUsedAt == nil {
		t.Fatal("last_used_at not recorded")
	}

	// The session cookie must not rescue a revoked token.
	if err := f.st.RevokeAPIToken(apiToken.ID, f.clock.now); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	cookie := f.as(t, f.LeadA)
	rec = f.do(http.MethodGet, "/api/v1/me", nil,
		map[string]string{"Authorization": "Bearer " + token}, cookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status with revoked token = %d, want 401: %s", rec.Code, rec.Body)
	}
}
