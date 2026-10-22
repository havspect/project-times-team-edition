package store

import (
	"errors"
	"testing"
	"time"
)

func TestArchiveTeamArchivesProjects(t *testing.T) {
	s := openTestStore(t)
	at := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)

	team := &Team{Name: "Platform"}
	if err := s.CreateTeam(team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	project := &Project{TeamID: team.ID, Name: "Timeline"}
	if err := s.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	if err := s.ArchiveTeam(team.ID, at); err != nil {
		t.Fatalf("archive: %v", err)
	}
	var got Project
	if err := s.db.First(&got, project.ID).Error; err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if got.ArchivedAt == nil || !got.ArchivedAt.Equal(at) {
		t.Fatalf("project archived_at = %v, want %v", got.ArchivedAt, at)
	}

	// A repeat call succeeds and keeps the first archive time.
	if err := s.ArchiveTeam(team.ID, at.Add(time.Hour)); err != nil {
		t.Fatalf("second archive: %v", err)
	}
	if err := s.db.First(&got, project.ID).Error; err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if !got.ArchivedAt.Equal(at) {
		t.Fatalf("project archived_at = %v, want the first time %v", got.ArchivedAt, at)
	}

	if err := s.ArchiveTeam(9999, at); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown team: got %v, want ErrNotFound", err)
	}
}

func TestArchivedTeamGrantsNoLead(t *testing.T) {
	s := openTestStore(t)
	u := &User{Email: "grace@example.com", Name: "Grace", PasswordHash: "x"}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	team := &Team{Name: "Platform"}
	if err := s.CreateTeam(team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := s.CreateMembership(&TeamMembership{TeamID: team.ID, UserID: u.ID, Role: RoleLead}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	if ok, err := s.LeadsTeam(u.ID, team.ID); err != nil || !ok {
		t.Fatalf("LeadsTeam before archive = %t, %v, want true", ok, err)
	}
	if err := s.ArchiveTeam(team.ID, time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if ok, err := s.LeadsTeam(u.ID, team.ID); err != nil || ok {
		t.Fatalf("LeadsTeam after archive = %t, %v, want false", ok, err)
	}
	members, err := s.MembersOf(team.ID)
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if len(members) != 1 || members[0].UserID != u.ID || members[0].Role != RoleLead {
		t.Fatalf("membership row lost on archive: %+v", members)
	}
}

func TestReplaceMembershipsRollsBackOnUnknownUser(t *testing.T) {
	s := openTestStore(t)
	u := &User{Email: "linus@example.com", Name: "Linus", PasswordHash: "x"}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	team := &Team{Name: "Platform"}
	if err := s.CreateTeam(team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := s.CreateMembership(&TeamMembership{TeamID: team.ID, UserID: u.ID, Role: RoleMember}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	err := s.ReplaceMemberships(team.ID, []MemberRole{{UserID: 9999, Role: RoleLead}},
		time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrUnknownUser) {
		t.Fatalf("unknown user: got %v, want ErrUnknownUser", err)
	}
	members, err := s.MembersOf(team.ID)
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if len(members) != 1 || members[0].Role != RoleMember {
		t.Fatalf("memberships after a refused replace = %+v, want the old row", members)
	}
}
