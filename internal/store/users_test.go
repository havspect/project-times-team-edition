package store

import (
	"errors"
	"testing"
	"time"
)

func addUser(t *testing.T, s *Store, email string, admin bool) *User {
	t.Helper()
	u := &User{Email: email, Name: email, PasswordHash: "hash", IsAdmin: admin, Active: true}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u
}

func TestListUsersOrdersByEmail(t *testing.T) {
	s := openTestStore(t)
	for _, email := range []string{"zoe@example.com", "Ada@example.com", "mia@example.com"} {
		addUser(t, s, email, false)
	}

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	want := []string{"ada@example.com", "mia@example.com", "zoe@example.com"}
	if len(users) != len(want) {
		t.Fatalf("got %d users, want %d", len(users), len(want))
	}
	for i, email := range want {
		if users[i].Email != email {
			t.Errorf("users[%d].Email = %q, want %q", i, users[i].Email, email)
		}
	}
}

func TestSaveUserUpdatesAdminFields(t *testing.T) {
	s := openTestStore(t)
	u := addUser(t, s, "ada@example.com", false)

	u.Name = "Ada Lovelace"
	u.Timezone = "Europe/Berlin"
	u.IsAdmin = true
	if err := s.SaveUser(u, at(9)); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}

	got, err := s.UserByID(u.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	if got.Name != "Ada Lovelace" || got.Timezone != "Europe/Berlin" || !got.IsAdmin || !got.Active {
		t.Fatalf("saved user = %+v, want the new fields", got)
	}

	if err := s.SaveUser(&User{ID: 999}, at(9)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SaveUser(unknown) = %v, want ErrNotFound", err)
	}
}

func TestSaveUserKeepsLastAdmin(t *testing.T) {
	s := openTestStore(t)
	admin := addUser(t, s, "ada@example.com", true)
	member := addUser(t, s, "linus@example.com", false)

	for name, change := range map[string]func(*User){
		"demote":     func(u *User) { u.IsAdmin = false },
		"deactivate": func(u *User) { u.Active = false },
	} {
		t.Run(name, func(t *testing.T) {
			u, err := s.UserByID(admin.ID)
			if err != nil {
				t.Fatalf("by id: %v", err)
			}
			change(u)
			if err := s.SaveUser(u, at(9)); !errors.Is(err, ErrLastAdmin) {
				t.Fatalf("SaveUser = %v, want ErrLastAdmin", err)
			}
			left, err := s.UserByID(admin.ID)
			if err != nil {
				t.Fatalf("by id: %v", err)
			}
			if !left.IsAdmin || !left.Active {
				t.Fatalf("last admin changed to %+v, want it untouched", left)
			}
		})
	}

	// A second admin makes both moves legal.
	addUser(t, s, "grace@example.com", true)
	u, err := s.UserByID(admin.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	u.IsAdmin = false
	if err := s.SaveUser(u, at(9)); err != nil {
		t.Fatalf("demote with a second admin: %v", err)
	}

	// The second admin is now the last one.
	last, err := s.UserByID(admin.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	last.Active = false
	if err := s.SaveUser(last, at(9)); err != nil {
		t.Fatalf("deactivate a non-admin: %v", err)
	}

	// Deactivating a plain member is always allowed.
	member.Active = false
	if err := s.SaveUser(member, at(9)); err != nil {
		t.Fatalf("deactivate a member: %v", err)
	}
}

func TestSetUserPassword(t *testing.T) {
	s := openTestStore(t)
	u := addUser(t, s, "ada@example.com", false)
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	earlier := now.Add(-time.Hour)

	kept := &Session{UserID: u.ID, TokenHash: "kept", ExpiresAt: now.Add(time.Hour)}
	dropped := &Session{UserID: u.ID, TokenHash: "dropped", ExpiresAt: now.Add(time.Hour)}
	for _, sess := range []*Session{kept, dropped} {
		if err := s.CreateSession(sess); err != nil {
			t.Fatalf("create session %s: %v", sess.TokenHash, err)
		}
	}
	live := &APIToken{UserID: u.ID, Name: "cli", TokenHash: "live"}
	old := &APIToken{UserID: u.ID, Name: "old", TokenHash: "old", RevokedAt: &earlier}
	for _, tok := range []*APIToken{live, old} {
		if err := s.CreateAPIToken(tok); err != nil {
			t.Fatalf("create token %s: %v", tok.Name, err)
		}
	}

	if err := s.SetUserPassword(u.ID, "newhash", true, kept.ID, now); err != nil {
		t.Fatalf("SetUserPassword: %v", err)
	}
	got, err := s.UserByID(u.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	if got.PasswordHash != "newhash" || !got.MustChangePassword {
		t.Fatalf("after reset = %+v, want the new hash and the flag", got)
	}

	// Every other session and every live token dies with the old password.
	if _, err := s.SessionByTokenHash("kept"); err != nil {
		t.Fatalf("kept session = %v, want it kept", err)
	}
	if _, err := s.SessionByTokenHash("dropped"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("dropped session = %v, want ErrNotFound", err)
	}
	revoked, err := s.APITokenByTokenHash("live")
	if err != nil {
		t.Fatalf("live token: %v", err)
	}
	if revoked.RevokedAt == nil || !revoked.RevokedAt.Equal(now) {
		t.Fatalf("live token revoked_at = %v, want %v", revoked.RevokedAt, now)
	}
	untouched, err := s.APITokenByTokenHash("old")
	if err != nil {
		t.Fatalf("old token: %v", err)
	}
	if untouched.RevokedAt == nil || !untouched.RevokedAt.Equal(earlier) {
		t.Fatalf("old token revoked_at = %v, want %v untouched", untouched.RevokedAt, earlier)
	}

	// keepSessionID 0 keeps none, which is what an admin reset uses.
	if err := s.SetUserPassword(u.ID, "otherhash", false, 0, now); err != nil {
		t.Fatalf("SetUserPassword: %v", err)
	}
	got, err = s.UserByID(u.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	if got.PasswordHash != "otherhash" || got.MustChangePassword {
		t.Fatalf("after change = %+v, want the new hash and no flag", got)
	}
	if _, err := s.SessionByTokenHash("kept"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("kept session = %v, want ErrNotFound", err)
	}

	if err := s.SetUserPassword(999, "hash", true, 0, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetUserPassword(unknown) = %v, want ErrNotFound", err)
	}
}
