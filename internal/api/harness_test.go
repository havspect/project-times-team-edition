package api

import (
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/havspect/project-times/internal/auth"
	"github.com/havspect/project-times/internal/store"
)

// fixture is the cast every stage-2 test shares: an admin, two leads of
// different teams, a member of one of them, and a user in no team.
type fixture struct {
	*harness
	Admin, LeadA, LeadB, Member, Outsider *store.User
	TeamA, TeamB                          *store.Team
	ProjectA, ProjectB                    *store.Project
}

const fixturePassword = "correct horse battery staple"

// fixtureHash hashes the shared password once: argon2 is deliberately slow, and
// every seeded test would pay for it again.
var fixtureHash = sync.OnceValue(func() string {
	hash, err := auth.HashPassword(fixturePassword)
	if err != nil {
		panic(err)
	}
	return hash
})

func seed(t *testing.T) *fixture {
	t.Helper()
	h := newHarness(t)
	f := &fixture{harness: h, Admin: h.user}

	f.LeadA = f.user(t, "grace@example.com", "Grace Hopper")
	f.LeadB = f.user(t, "hedy@example.com", "Hedy Lamarr")
	f.Member = f.user(t, "linus@example.com", "Linus Torvalds")
	f.Outsider = f.user(t, "margaret@example.com", "Margaret Hamilton")

	f.TeamA = f.team(t, "Platform")
	f.TeamB = f.team(t, "Mobile")
	f.membership(t, f.TeamA, f.LeadA, store.RoleLead)
	f.membership(t, f.TeamA, f.Member, store.RoleMember)
	f.membership(t, f.TeamB, f.LeadB, store.RoleLead)
	f.ProjectA = f.project(t, f.TeamA, "Timeline")
	f.ProjectB = f.project(t, f.TeamB, "Sync")
	return f
}

// The rows below are inserted through gorm directly, so the fixture does not
// depend on store helpers that the stage-2 handlers add later.

func (f *fixture) user(t *testing.T, email, name string) *store.User {
	t.Helper()
	u := &store.User{Email: email, Name: name, PasswordHash: fixtureHash(), Timezone: "UTC", Active: true}
	if err := f.st.CreateUser(u); err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u
}

func (f *fixture) team(t *testing.T, name string) *store.Team {
	t.Helper()
	team := &store.Team{Name: name}
	if err := f.st.CreateTeam(team); err != nil {
		t.Fatalf("create team %s: %v", name, err)
	}
	return team
}

func (f *fixture) membership(t *testing.T, team *store.Team, u *store.User, role string) {
	t.Helper()
	m := &store.TeamMembership{TeamID: team.ID, UserID: u.ID, Role: role}
	if err := f.st.CreateMembership(m); err != nil {
		t.Fatalf("join %s as %s: %v", u.Email, role, err)
	}
}

func (f *fixture) project(t *testing.T, team *store.Team, name string) *store.Project {
	t.Helper()
	p := &store.Project{TeamID: team.ID, Name: name}
	if err := f.st.CreateProject(p); err != nil {
		t.Fatalf("create project %s: %v", name, err)
	}
	return p
}

// as mints a session for u without going through login, which its own tests cover.
func (h *harness) as(t *testing.T, u *store.User) string {
	t.Helper()
	token, hash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("new session token: %v", err)
	}
	sess := &store.Session{UserID: u.ID, TokenHash: hash, ExpiresAt: h.clock.Now().Add(auth.SessionTTL)}
	if err := h.st.CreateSession(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token
}

// doAs sends the request as u over a session cookie.
func (h *harness) doAs(t *testing.T, u *store.User, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return h.do(method, path, body, nil, h.as(t, u))
}
