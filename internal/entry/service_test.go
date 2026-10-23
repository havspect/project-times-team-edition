package entry

import (
	"errors"
	"testing"
	"time"

	"github.com/havspect/project-times/internal/store"
)

// fixedClock keeps the service tests off the wall clock.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// fakeStore is the in-memory adapter; it records the last UpdateEntry changes.
type fakeStore struct {
	entries  map[uint]*store.Entry
	projects map[uint]store.Project
	nextID   uint
	changes  map[string]any
}

func (f *fakeStore) EntryByID(id uint) (*store.Entry, error) {
	e, ok := f.entries[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (f *fakeStore) CreateEntry(e *store.Entry) error {
	e.ID = f.nextID
	f.nextID++
	cp := *e
	f.entries[e.ID] = &cp
	return nil
}

func (f *fakeStore) UpdateEntry(id uint, _ time.Time, changes map[string]any) error {
	if _, ok := f.entries[id]; !ok {
		return store.ErrNotFound
	}
	f.changes = changes
	return nil
}

func (f *fakeStore) SoftDeleteEntry(id uint, at time.Time) error {
	e, ok := f.entries[id]
	if !ok {
		return store.ErrNotFound
	}
	e.DeletedAt = &at
	return nil
}

func (f *fakeStore) RestoreEntry(id uint, _ time.Time) error {
	e, ok := f.entries[id]
	if !ok {
		return store.ErrNotFound
	}
	e.DeletedAt = nil
	return nil
}

func (f *fakeStore) ProjectByID(id uint) (*store.Project, error) {
	p, ok := f.projects[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &p, nil
}

// seed inserts an entry and returns its id.
func (f *fakeStore) seed(t *testing.T, e store.Entry) uint {
	t.Helper()
	if err := f.CreateEntry(&e); err != nil {
		t.Fatalf("seed entry: %v", err)
	}
	return e.ID
}

func newService() (*Service, *fakeStore) {
	f := &fakeStore{
		entries:  map[uint]*store.Entry{},
		projects: map[uint]store.Project{1: {ID: 1, Name: "Timeline"}},
		nextID:   1,
	}
	return New(f, fixedClock{now}), f
}

func TestCreateRefusesTheRules(t *testing.T) {
	svc, st := newService()
	archived := now
	st.projects[2] = store.Project{ID: 2, Name: "Old", ArchivedAt: &archived}

	cases := []struct {
		name      string
		projectID uint
		start     time.Time
		end       time.Time
		want      error
	}{
		{"unknown project", 99, now.Add(-time.Hour), now, ErrUnknownProject},
		{"archived project", 2, now.Add(-time.Hour), now, ErrArchivedProject},
		{"too long", 1, now.Add(-25 * time.Hour), now, ErrTooLong},
		{"end before start", 1, now, now.Add(-time.Hour), ErrInvalidRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(1, tc.projectID, tc.start, tc.end, ""); !errors.Is(err, tc.want) {
				t.Errorf("Create = %v, want %v", err, tc.want)
			}
		})
	}

	e, err := svc.Create(1, 1, now.Add(-time.Hour), now, "note")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ID == 0 || !e.CreatedAt.Equal(now) {
		t.Errorf("created entry = %+v, want an id and CreatedAt = now", e)
	}
}

func TestPatchRules(t *testing.T) {
	finished := now.Add(-time.Minute)

	t.Run("foreign entry", func(t *testing.T) {
		svc, st := newService()
		id := st.seed(t, store.Entry{UserID: 1, ProjectID: 1, StartAt: now.Add(-time.Hour), EndAt: &finished})
		if _, err := svc.Patch(2, id, Patch{}); !errors.Is(err, ErrForbidden) {
			t.Errorf("Patch = %v, want ErrForbidden", err)
		}
	})

	t.Run("deleted entry is not found", func(t *testing.T) {
		svc, st := newService()
		deleted := now.Add(-time.Minute)
		id := st.seed(t, store.Entry{UserID: 1, ProjectID: 1, StartAt: now.Add(-time.Hour), EndAt: &finished, DeletedAt: &deleted})
		if _, err := svc.Patch(1, id, Patch{}); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("Patch = %v, want store.ErrNotFound", err)
		}
	})

	t.Run("unknown entry", func(t *testing.T) {
		svc, _ := newService()
		if _, err := svc.Patch(1, 99, Patch{}); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("Patch = %v, want store.ErrNotFound", err)
		}
	})

	t.Run("retiming a running entry needs an end", func(t *testing.T) {
		svc, st := newService()
		id := st.seed(t, store.Entry{UserID: 1, ProjectID: 1, StartAt: now.Add(-time.Hour)})
		start := now.Add(-2 * time.Hour)
		if _, err := svc.Patch(1, id, Patch{StartAt: &start}); !errors.Is(err, ErrRetimeRunning) {
			t.Errorf("Patch = %v, want ErrRetimeRunning", err)
		}
	})

	t.Run("a note edit leaves a running entry running", func(t *testing.T) {
		svc, st := newService()
		id := st.seed(t, store.Entry{UserID: 1, ProjectID: 1, StartAt: now.Add(-time.Hour)})
		note := "still going"
		if _, err := svc.Patch(1, id, Patch{Note: &note}); err != nil {
			t.Fatalf("Patch: %v", err)
		}
		if st.changes["note"] != note {
			t.Errorf("changes = %v, want the note", st.changes)
		}
		if _, ok := st.changes["end_at"]; ok {
			t.Error("changes include end_at, want the entry left running")
		}
	})

	t.Run("a retime writes both ends", func(t *testing.T) {
		svc, st := newService()
		id := st.seed(t, store.Entry{UserID: 1, ProjectID: 1, StartAt: now.Add(-2 * time.Hour), EndAt: &finished})
		start, end := now.Add(-3*time.Hour), now.Add(-time.Hour)
		if _, err := svc.Patch(1, id, Patch{StartAt: &start, EndAt: &end}); err != nil {
			t.Fatalf("Patch: %v", err)
		}
		for _, key := range []string{"start_at", "end_at"} {
			if _, ok := st.changes[key]; !ok {
				t.Errorf("changes = %v, want %s", st.changes, key)
			}
		}
	})

	t.Run("moving to an archived project", func(t *testing.T) {
		svc, st := newService()
		archived := now
		st.projects[2] = store.Project{ID: 2, ArchivedAt: &archived}
		id := st.seed(t, store.Entry{UserID: 1, ProjectID: 1, StartAt: now.Add(-time.Hour), EndAt: &finished})
		project := uint(2)
		if _, err := svc.Patch(1, id, Patch{ProjectID: &project}); !errors.Is(err, ErrArchivedProject) {
			t.Errorf("Patch = %v, want ErrArchivedProject", err)
		}
	})
}

func TestDeleteAndRestore(t *testing.T) {
	svc, st := newService()
	finished := now.Add(-time.Minute)
	id := st.seed(t, store.Entry{UserID: 1, ProjectID: 1, StartAt: now.Add(-time.Hour), EndAt: &finished})

	if err := svc.Delete(2, id); !errors.Is(err, ErrForbidden) {
		t.Errorf("Delete = %v, want ErrForbidden", err)
	}
	if err := svc.Delete(1, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	e, err := svc.Restore(id)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if e.DeletedAt != nil {
		t.Error("Restore returned a deleted entry")
	}
	if _, err := svc.Restore(99); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Restore = %v, want store.ErrNotFound", err)
	}
}
