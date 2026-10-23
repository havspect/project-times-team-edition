package api

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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

func TestAccessLogRecordsTheRequest(t *testing.T) {
	f := seed(t)
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	if rec := f.do(http.MethodGet, "/api/v1/me", nil, nil, f.as(t, f.LeadA)); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	line := buf.String()
	for _, want := range []string{"msg=request", "method=GET", "path=/api/v1/me", "status=200", fmt.Sprintf("user_id=%d", f.LeadA.ID)} {
		if !strings.Contains(line, want) {
			t.Errorf("access log %q does not contain %q", line, want)
		}
	}

	// A rejected request is still a request; an early return must not vanish.
	buf.Reset()
	if rec := f.do(http.MethodGet, "/api/v1/me", nil, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if line := buf.String(); !strings.Contains(line, "status=401") || strings.Contains(line, "user_id=") {
		t.Errorf("access log %q, want status=401 and no user id", line)
	}
}

func TestRequestBodyIsCapped(t *testing.T) {
	f := seed(t)
	rec := f.do(http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": strings.Repeat("a", maxRequestBody)}, nil, "")
	if rec.Code == http.StatusOK || rec.Code >= http.StatusInternalServerError {
		t.Fatalf("status = %d, want a client error below 500", rec.Code)
	}
}
