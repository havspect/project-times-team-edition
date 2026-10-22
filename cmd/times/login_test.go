package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/havspect/project-times/internal/api"
	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/clock"
	"github.com/havspect/project-times/internal/store"
)

const loginPassword = "correct horse battery staple"

// loginServer starts the API over a store holding one active admin.
func loginServer(t *testing.T) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "times.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	hash, err := auth.HashPassword(loginPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &store.User{
		Email: "ada@example.com", Name: "Ada Lovelace", PasswordHash: hash,
		Timezone: "UTC", IsAdmin: true, Active: true,
	}
	if err := st.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	srv := httptest.NewServer(api.New(st, clock.System{}))
	t.Cleanup(srv.Close)
	return srv
}

// loginHome isolates the config file and the settings that could override it.
func loginHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TIMES_URL", "")
	t.Setenv("TIMES_TOKEN", "")
	return home
}

func configPath(home string) string {
	return filepath.Join(home, ".config", "times", "config.json")
}

func TestLogin(t *testing.T) {
	home := loginHome(t)
	srv := loginServer(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"login", "--url", srv.URL, "--email", "ada@example.com", "--password", loginPassword}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "logged in as Ada Lovelace <ada@example.com>") {
		t.Fatalf("stdout %q does not announce the user", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr %q, want nothing", stderr.String())
	}

	path := configPath(home)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
	var saved struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatalf("decode config %q: %v", b, err)
	}
	if saved.URL != srv.URL || saved.Token == "" {
		t.Fatalf("saved config = %+v, want the server URL and a token", saved)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/me", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+saved.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request /me: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("saved token /me = %d, want 200", resp.StatusCode)
	}
	var me struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		t.Fatalf("decode /me: %v", err)
	}
	if me.Email != "ada@example.com" {
		t.Fatalf("token belongs to %q, want ada@example.com", me.Email)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	home := loginHome(t)
	srv := loginServer(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"login", "--url", srv.URL, "--email", "ada@example.com", "--password", "wrong"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if n := strings.Count(stderr.String(), "\n"); n != 1 {
		t.Fatalf("stderr has %d lines, want one: %q", n, stderr.String())
	}
	if !strings.Contains(stderr.String(), "times login:") {
		t.Fatalf("stderr %q does not name the command", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout %q, want nothing", stdout.String())
	}
	if _, err := os.Stat(configPath(home)); !os.IsNotExist(err) {
		t.Fatalf("config file was written on a failed login: %v", err)
	}
}

func TestLoginMissingURL(t *testing.T) {
	loginHome(t)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"login", "--email", "ada@example.com", "--password", loginPassword}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage: times login") {
		t.Fatalf("stderr %q does not show the usage line", stderr.String())
	}
}
