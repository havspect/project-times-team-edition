package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/store"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr.String())
	}
	if got, want := stdout.String(), "times 0.1.0-dev\n"; got != want {
		t.Fatalf("stdout %q, want %q", got, want)
	}
}

func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		nil,
		{"unknown"},
		{"admin"},
		{"admin", "delete"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("run(%q) exit %d, want 2", args, code)
		}
		if stderr.Len() == 0 {
			t.Errorf("run(%q) wrote no usage to stderr", args)
		}
	}
}

func TestAdminCreate(t *testing.T) {
	db := filepath.Join(t.TempDir(), "times.db")

	var stdout, stderr bytes.Buffer
	code := run([]string{"admin", "create", "--email", "ada@example.com", "--name", "Ada Lovelace", "--db", db}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr.String())
	}
	if n := strings.Count(stdout.String(), "\n"); n != 1 {
		t.Fatalf("stdout has %d lines, want exactly 1: %q", n, stdout.String())
	}
	if !strings.Contains(stdout.String(), "ada@example.com") {
		t.Fatalf("stdout %q does not name the admin", stdout.String())
	}
	if strings.Contains(stderr.String(), "initial password") {
		t.Fatalf("password leaked to stderr: %q", stderr.String())
	}

	password := passwordFrom(t, stdout.String())

	st, err := store.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	u, err := st.UserByEmail("ada@example.com")
	if err != nil {
		t.Fatalf("load created user: %v", err)
	}
	if !u.IsAdmin || !u.Active {
		t.Fatalf("user admin=%v active=%v, want both true", u.IsAdmin, u.Active)
	}
	if u.Name != "Ada Lovelace" {
		t.Fatalf("name %q, want %q", u.Name, "Ada Lovelace")
	}
	if !auth.VerifyPassword(u.PasswordHash, password) {
		t.Fatalf("printed password does not verify against the stored hash")
	}
}

func TestAdminCreateDuplicateEmail(t *testing.T) {
	db := filepath.Join(t.TempDir(), "times.db")
	args := []string{"admin", "create", "--email", "ada@example.com", "--name", "Ada", "--db", db}

	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("first create exit %d, want 0 (stderr %q)", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(args, &stdout, &stderr); code != 1 {
		t.Fatalf("second create exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("stderr %q does not explain the duplicate", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("second create printed %q, want nothing", stdout.String())
	}
}

func TestAdminCreateMissingEmail(t *testing.T) {
	db := filepath.Join(t.TempDir(), "times.db")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"admin", "create", "--name", "Ada", "--db", db}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

// passwordFrom extracts the only secret on the single output line.
func passwordFrom(t *testing.T, line string) string {
	t.Helper()
	const marker = "initial password: "
	i := strings.Index(line, marker)
	if i < 0 {
		t.Fatalf("output %q has no %q", line, marker)
	}
	return strings.TrimSpace(line[i+len(marker):])
}
