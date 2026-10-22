package client

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/havspect/project-times/internal/api"
	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/clock"
	"github.com/havspect/project-times/internal/store"
)

const testPassword = "correct horse battery staple"

// newServer starts the real API over a store with one active user.
func newServer(t *testing.T, email string) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "times.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &store.User{Email: email, Name: "Ada Lovelace", PasswordHash: hash, Timezone: "UTC", Active: true}
	if err := st.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	srv := httptest.NewServer(api.New(st, clock.System{}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLogin(t *testing.T) {
	srv := newServer(t, "ada@example.com")
	c := New(Config{URL: srv.URL})

	me, err := c.Login("ada@example.com", testPassword)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if me.Email != "ada@example.com" || me.Name != "Ada Lovelace" || me.ID == 0 {
		t.Fatalf("me = %+v, want the seeded user", me)
	}

	_, err = c.Login("ada@example.com", "wrong")
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
		t.Fatalf("wrong password error = %v, want a 401 *Error", err)
	}
	if !strings.Contains(apiErr.Error(), "incorrect") {
		t.Fatalf("error %q does not carry the server detail", apiErr)
	}
}

func TestCreateTokenThenBearer(t *testing.T) {
	srv := newServer(t, "ada@example.com")
	c := New(Config{URL: srv.URL})
	if _, err := c.Login("ada@example.com", testPassword); err != nil {
		t.Fatalf("login: %v", err)
	}

	tok, err := c.CreateToken("laptop")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if tok.ID == 0 || tok.Name != "laptop" || tok.Token == "" {
		t.Fatalf("created token = %+v, want an id, name and raw value", tok)
	}
	if !strings.HasPrefix(tok.Token, "ts_") {
		t.Fatalf("token %q does not start with ts_", tok.Token)
	}

	bearer := New(Config{URL: srv.URL, Token: tok.Token})
	me, err := bearer.Me()
	if err != nil {
		t.Fatalf("me with bearer: %v", err)
	}
	if me.Email != "ada@example.com" {
		t.Fatalf("bearer me email = %q, want ada@example.com", me.Email)
	}

	_, err = New(Config{URL: srv.URL, Token: "ts_bogus"}).Me()
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
		t.Fatalf("bogus token error = %v, want a 401 *Error", err)
	}
}
