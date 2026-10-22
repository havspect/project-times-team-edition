package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "times.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

func TestMigrateTwiceKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "times.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := s.CreateUser(&User{Email: "ada@example.com", Name: "Ada", PasswordHash: "x"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	if err := s.Migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if _, err := s.UserByEmail("ada@example.com"); err != nil {
		t.Fatalf("user lost after second migrate: %v", err)
	}
}

func TestUserRoundTrip(t *testing.T) {
	s := openTestStore(t)

	u := &User{Email: "Ada@Example.COM", Name: "Ada Lovelace", PasswordHash: "hash"}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("ID not set")
	}
	if u.Email != "ada@example.com" {
		t.Fatalf("email not lowercased: %q", u.Email)
	}
	if u.Timezone != "UTC" || !u.Active || u.IsAdmin {
		t.Fatalf("defaults not applied: %+v", u)
	}

	got, err := s.UserByID(u.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	if got.Name != "Ada Lovelace" || got.PasswordHash != "hash" {
		t.Fatalf("round trip mismatch: %+v", got)
	}

	for _, email := range []string{"ADA@example.com", "ada@EXAMPLE.COM"} {
		got, err := s.UserByEmail(email)
		if err != nil {
			t.Fatalf("by email %q: %v", email, err)
		}
		if got.ID != u.ID {
			t.Fatalf("by email %q got id %d, want %d", email, got.ID, u.ID)
		}
	}
}

func TestDuplicateEmailRejected(t *testing.T) {
	s := openTestStore(t)

	if err := s.CreateUser(&User{Email: "ada@example.com", Name: "Ada", PasswordHash: "x"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	err := s.CreateUser(&User{Email: "ADA@example.com", Name: "Ada Again", PasswordHash: "y"})
	if err == nil {
		t.Fatal("duplicate email accepted")
	}
}

func TestNotFound(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.UserByID(42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UserByID: got %v, want ErrNotFound", err)
	}
	if _, err := s.UserByEmail("nobody@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UserByEmail: got %v, want ErrNotFound", err)
	}
	if _, err := s.SessionByTokenHash("deadbeef"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SessionByTokenHash: got %v, want ErrNotFound", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := openTestStore(t)
	u := &User{Email: "ada@example.com", Name: "Ada", PasswordHash: "x"}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	sess := &Session{UserID: u.ID, TokenHash: "abc123", ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateSession(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := s.SessionByTokenHash("abc123")
	if err != nil {
		t.Fatalf("by token hash: %v", err)
	}
	if got.ID != sess.ID || got.UserID != u.ID || !got.ExpiresAt.Equal(sess.ExpiresAt) {
		t.Fatalf("round trip mismatch: %+v", got)
	}

	if err := s.DeleteSession(sess.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.SessionByTokenHash("abc123"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: got %v, want ErrNotFound", err)
	}
}
