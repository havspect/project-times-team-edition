package store

import (
	"errors"
	"testing"
	"time"
)

func TestProjectList(t *testing.T) {
	s := openTestStore(t)
	team := &Team{Name: "Platform"}
	if err := s.CreateTeam(team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	at := time.Now()
	for _, p := range []*Project{
		{TeamID: team.ID, Name: "Timeline"},
		{TeamID: team.ID, Name: "Archive", ArchivedAt: &at},
	} {
		if err := s.CreateProject(p); err != nil {
			t.Fatalf("create project %s: %v", p.Name, err)
		}
	}

	active, err := s.Projects(false)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || active[0].Name != "Timeline" {
		t.Fatalf("active = %+v, want only Timeline", active)
	}

	all, err := s.Projects(true)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 || all[0].Name != "Archive" || all[1].Name != "Timeline" {
		t.Fatalf("all = %+v, want Archive then Timeline by name", all)
	}
}

func TestUpdateProject(t *testing.T) {
	s := openTestStore(t)
	team := &Team{Name: "Platform"}
	if err := s.CreateTeam(team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	p := &Project{TeamID: team.ID, Name: "Timeline"}
	if err := s.CreateProject(p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	at := time.Now().UTC().Truncate(time.Second)
	p.Name, p.Color, p.ArchivedAt = "Timeline 2", "#ff0000", &at
	if err := s.UpdateProject(p); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got, err := s.ProjectByID(p.ID)
	if err != nil {
		t.Fatalf("by id: %v", err)
	}
	if got.Name != "Timeline 2" || got.Color != "#ff0000" || got.ArchivedAt == nil || !got.ArchivedAt.Equal(at) {
		t.Fatalf("after archive = %+v", got)
	}

	p.ArchivedAt = nil
	if err := s.UpdateProject(p); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got, err = s.ProjectByID(p.ID); err != nil {
		t.Fatalf("by id: %v", err)
	}
	if got.ArchivedAt != nil {
		t.Fatalf("archived_at = %v, want nil after restore", got.ArchivedAt)
	}
}

func TestProjectByIDNotFound(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.ProjectByID(42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestTeamAcceptsProjects(t *testing.T) {
	s := openTestStore(t)
	live := &Team{Name: "Platform"}
	at := time.Now()
	archived := &Team{Name: "Old", ArchivedAt: &at}
	for _, team := range []*Team{live, archived} {
		if err := s.CreateTeam(team); err != nil {
			t.Fatalf("create team %s: %v", team.Name, err)
		}
	}

	for _, tt := range []struct {
		name   string
		teamID uint
		want   bool
	}{
		{"live", live.ID, true},
		{"archived", archived.ID, false},
		{"unknown", 42, false},
	} {
		got, err := s.TeamAcceptsProjects(tt.teamID)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("%s = %t, want %t", tt.name, got, tt.want)
		}
	}
}
