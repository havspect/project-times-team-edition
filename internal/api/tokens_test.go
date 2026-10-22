package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/store"
)

// tokenOrigin is required for cookie-authenticated mutations; tokens are one.
var tokenOrigin = map[string]string{"Origin": "http://example.com"}

// tokenDo sends a cookie-authenticated request with the Origin the middleware wants.
func tokenDo(t *testing.T, f *fixture, u *store.User, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(method, path, body, tokenOrigin, f.as(t, u))
}

type tokenCreateResponse struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

func TestTokensCreateListRevoke(t *testing.T) {
	f := seed(t)

	rec := tokenDo(t, f, f.LeadA, http.MethodPost, "/api/v1/me/tokens", map[string]string{"name": "laptop"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", rec.Code, rec.Body)
	}
	first := decode[tokenCreateResponse](t, rec)
	if first.Name != "laptop" {
		t.Fatalf("name = %q, want laptop", first.Name)
	}
	if !strings.HasPrefix(first.Token, "ts_") || len(first.Token) != len("ts_")+43 {
		t.Fatalf("token = %q, want ts_ and 43 URL-safe base64 chars", first.Token)
	}

	// The row holds the hash, never the value shown to the user.
	stored, err := f.st.APITokenByTokenHash(auth.HashToken(first.Token))
	if err != nil {
		t.Fatalf("lookup by hash: %v", err)
	}
	if stored.TokenHash == first.Token {
		t.Fatal("token_hash equals the raw token")
	}
	if _, err := f.st.APITokenByTokenHash(first.Token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("lookup by raw token = %v, want ErrNotFound", err)
	}

	rec = tokenDo(t, f, f.LeadA, http.MethodPost, "/api/v1/me/tokens", map[string]string{"name": "desktop"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("second create status = %d, want 201: %s", rec.Code, rec.Body)
	}
	second := decode[tokenCreateResponse](t, rec)

	rec = tokenDo(t, f, f.LeadA, http.MethodGet, "/api/v1/me/tokens", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", rec.Code, rec.Body)
	}
	var raw []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	for _, tok := range raw {
		for _, secret := range []string{"token", "token_hash"} {
			if _, ok := tok[secret]; ok {
				t.Fatalf("list item exposes %q: %v", secret, tok)
			}
		}
	}
	if strings.Contains(rec.Body.String(), first.Token) || strings.Contains(rec.Body.String(), second.Token) {
		t.Fatal("list reply contains a raw token")
	}
	list := decode[[]tokenResponse](t, rec)
	if len(list) != 2 {
		t.Fatalf("list has %d tokens, want 2", len(list))
	}
	if list[0].ID != first.ID || list[1].ID != second.ID || list[0].ID >= list[1].ID {
		t.Fatalf("list ids = %d, %d; want %d, %d in order", list[0].ID, list[1].ID, first.ID, second.ID)
	}
	if list[0].Name != "laptop" || list[0].CreatedAt.IsZero() || list[0].RevokedAt != nil {
		t.Fatalf("first list item = %+v, want a live laptop token", list[0])
	}

	// The raw token authenticates as its owner.
	rec = f.do(http.MethodGet, "/api/v1/me", nil, map[string]string{"Authorization": "Bearer " + first.Token}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bearer /me status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if me := decode[meResponse](t, rec); me.Email != f.LeadA.Email {
		t.Fatalf("bearer /me email = %q, want %q", me.Email, f.LeadA.Email)
	}

	rec = tokenDo(t, f, f.LeadA, http.MethodDelete, tokenPath(first.ID), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, want 204: %s", rec.Code, rec.Body)
	}

	list = decode[[]tokenResponse](t, tokenDo(t, f, f.LeadA, http.MethodGet, "/api/v1/me/tokens", nil))
	if len(list) != 2 {
		t.Fatalf("list after revoke has %d tokens, want 2 (rows are kept)", len(list))
	}
	if list[0].RevokedAt == nil || list[1].RevokedAt != nil {
		t.Fatalf("revoked state = %v, %v; want only the first revoked", list[0].RevokedAt, list[1].RevokedAt)
	}

	rec = f.do(http.MethodGet, "/api/v1/me", nil, map[string]string{"Authorization": "Bearer " + first.Token}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token /me status = %d, want 401", rec.Code)
	}
	if rec := f.do(http.MethodGet, "/api/v1/me", nil, map[string]string{"Authorization": "Bearer " + second.Token}, ""); rec.Code != http.StatusOK {
		t.Fatalf("live token /me status = %d, want 200", rec.Code)
	}
}

func TestTokensAreOwn(t *testing.T) {
	f := seed(t)
	created := decode[tokenCreateResponse](t, tokenDo(t, f, f.LeadA, http.MethodPost, "/api/v1/me/tokens", map[string]string{"name": "laptop"}))

	rec := tokenDo(t, f, f.LeadB, http.MethodGet, "/api/v1/me/tokens", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("other user's list status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Fatalf("other user's list = %s, want []", body)
	}

	rec = tokenDo(t, f, f.LeadB, http.MethodDelete, tokenPath(created.ID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleting another user's token = %d, want 404", rec.Code)
	}
	if body := decode[struct {
		Type string `json:"type"`
	}](t, rec); body.Type != urnNotFound {
		t.Fatalf("type = %q, want %q", body.Type, urnNotFound)
	}

	list := decode[[]tokenResponse](t, tokenDo(t, f, f.LeadA, http.MethodGet, "/api/v1/me/tokens", nil))
	if len(list) != 1 || list[0].RevokedAt != nil {
		t.Fatalf("owner's tokens = %+v, want one live token", list)
	}
}

func TestTokensRequireAuth(t *testing.T) {
	f := seed(t)
	requests := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/me/tokens"},
		{http.MethodPost, "/api/v1/me/tokens"},
		{http.MethodDelete, tokenPath(1)},
	}
	for _, tt := range requests {
		rec := f.do(tt.method, tt.path, map[string]string{"name": "x"}, nil, "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s = %d, want 401", tt.method, tt.path, rec.Code)
		}
		if body := decode[struct {
			Type string `json:"type"`
		}](t, rec); body.Type != urnUnauthorized {
			t.Fatalf("%s %s type = %q, want %q", tt.method, tt.path, body.Type, urnUnauthorized)
		}
	}
}

func tokenPath(id uint) string { return "/api/v1/me/tokens/" + strconv.FormatUint(uint64(id), 10) }
