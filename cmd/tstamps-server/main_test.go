package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/havspect/tstamps/internal/auth"
	"github.com/havspect/tstamps/internal/store"
)

func TestUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"none", nil},
		{"unknown command", []string{"unknown"}},
		{"admin without subcommand", []string{"admin"}},
		{"unknown admin subcommand", []string{"admin", "delete"}},
		{"backup without destination", []string{"backup"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != 2 {
				t.Errorf("run(%q) exit %d, want 2", tt.args, code)
			}
			if stderr.Len() == 0 {
				t.Errorf("run(%q) wrote no usage to stderr", tt.args)
			}
		})
	}
}

func TestAdminCreate(t *testing.T) {
	db := filepath.Join(t.TempDir(), "tstamps.db")

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

func TestAdminCreateUsesEnvDB(t *testing.T) {
	db := filepath.Join(t.TempDir(), "env.db")
	t.Setenv("TSTAMPS_DB", db)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"admin", "create", "--email", "grace@example.com", "--name", "Grace Hopper"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr.String())
	}
	if _, err := os.Stat(db); err != nil {
		t.Fatalf("TSTAMPS_DB %s was not written: %v", db, err)
	}

	st, err := store.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if _, err := st.UserByEmail("grace@example.com"); err != nil {
		t.Fatalf("admin not in TSTAMPS_DB database: %v", err)
	}
}

func TestAdminCreateDuplicateEmail(t *testing.T) {
	db := filepath.Join(t.TempDir(), "tstamps.db")
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
	db := filepath.Join(t.TempDir(), "tstamps.db")
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

func TestBackup(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "tstamps.db")

	var stdout, stderr bytes.Buffer
	if code := run([]string{"admin", "create", "--email", "ada@example.com", "--name", "Ada", "--db", db}, &stdout, &stderr); code != 0 {
		t.Fatalf("admin create exit %d: %s", code, stderr.String())
	}

	dest := filepath.Join(dir, "snapshot.db")
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"backup", "--db", db, dest}, &stdout, &stderr); code != 0 {
		t.Fatalf("backup exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), dest) {
		t.Fatalf("stdout %q does not name the destination", stdout.String())
	}

	backup, err := store.Open(dest)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer backup.Close()
	if _, err := backup.UserByEmail("ada@example.com"); err != nil {
		t.Fatalf("admin missing from backup: %v", err)
	}

	// A second backup to the same name must fail rather than replace it.
	if code := run([]string{"backup", "--db", db, dest}, &stdout, &stderr); code != 1 {
		t.Fatalf("backup over an existing file exit %d, want 1", code)
	}
}

func TestBackupMissingDatabase(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "missing.db")

	var stdout, stderr bytes.Buffer
	if code := run([]string{"backup", "--db", db, filepath.Join(dir, "snapshot.db")}, &stdout, &stderr); code != 1 {
		t.Fatalf("backup missing database exit %d, want 1", code)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatalf("backup created the missing database %s", db)
	}
}
