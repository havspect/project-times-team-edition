package api

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTimerStartStop(t *testing.T) {
	f := seed(t)

	if rec := sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/timer", nil); strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("idle timer = %s, want null", rec.Body)
	}

	rec := sendAs(t, f, f.Member, http.MethodPost, apiPrefix+"/timer/start", map[string]any{
		"project_id": f.ProjectA.ID, "timezone": "UTC", "note": "writing the timeline",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("start = %d, want 201: %s", rec.Code, rec.Body)
	}
	started := decode[entryResponse](t, rec)
	if started.EndAt != nil || started.Note != "writing the timeline" || !started.StartAt.Equal(f.clock.now) {
		t.Fatalf("started = %+v", started)
	}

	if got := decode[entryResponse](t, sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/timer", nil)); got.ID != started.ID {
		t.Fatalf("timer = %+v, want entry %d", got, started.ID)
	}

	// The second start is the spec's 409, carrying the entry that is running.
	rec = sendAs(t, f, f.Member, http.MethodPost, apiPrefix+"/timer/start", map[string]any{"project_id": f.ProjectA.ID, "timezone": "UTC"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("second start = %d, want 409: %s", rec.Code, rec.Body)
	}
	conflict := decode[struct {
		Type         string        `json:"type"`
		RunningEntry entryResponse `json:"running_entry"`
	}](t, rec)
	if conflict.Type != urnTimerAlreadyRunning {
		t.Fatalf("type = %q, want %q", conflict.Type, urnTimerAlreadyRunning)
	}
	if conflict.RunningEntry.ID != started.ID {
		t.Fatalf("running_entry = %+v, want entry %d", conflict.RunningEntry, started.ID)
	}

	// Stopping at a later instant ends it there.
	f.clock.now = f.clock.now.Add(90 * time.Minute)
	rec = sendAs(t, f, f.Member, http.MethodPost, apiPrefix+"/timer/stop", map[string]any{"note": "done"})
	if rec.Code != http.StatusOK {
		t.Fatalf("stop = %d, want 200: %s", rec.Code, rec.Body)
	}
	stopped := decode[entryResponse](t, rec)
	if stopped.EndAt == nil || !stopped.EndAt.Equal(f.clock.now) || stopped.Note != "done" {
		t.Fatalf("stopped = %+v", stopped)
	}
	if rec := sendAs(t, f, f.Member, http.MethodGet, apiPrefix+"/timer", nil); strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("timer after stop = %s, want null", rec.Body)
	}
}

func TestTimerStopWithNothingRunning(t *testing.T) {
	f := seed(t)
	wantProblem(t, sendAs(t, f, f.Member, http.MethodPost, apiPrefix+"/timer/stop", map[string]any{}),
		http.StatusNotFound, urnNotFound)
}

func TestTimerStopPast24HoursKeepsItRunning(t *testing.T) {
	f := seed(t)
	e := f.running(t, f.Member, f.ProjectA, f.clock.now.Add(-25*time.Hour), "long night")

	wantProblem(t, sendAs(t, f, f.Member, http.MethodPost, apiPrefix+"/timer/stop", map[string]any{}),
		http.StatusUnprocessableEntity, urnEntryTooLong)

	still, err := f.st.EntryByID(e.ID)
	if err != nil {
		t.Fatalf("load entry: %v", err)
	}
	if still.EndAt != nil {
		t.Fatalf("entry was stopped despite entry-too-long: %+v", still)
	}
}

func TestTimerStartRejectsRules(t *testing.T) {
	f := seed(t)
	archived := f.clock.now
	f.ProjectB.ArchivedAt = &archived
	if err := f.st.UpdateProject(f.ProjectB); err != nil {
		t.Fatalf("archive project: %v", err)
	}

	for name, tc := range map[string]struct {
		projectID uint
		urn       string
	}{
		"archived project": {f.ProjectB.ID, urnInvalidInput},
		"unknown project":  {999, urnInvalidInput},
	} {
		t.Run(name, func(t *testing.T) {
			wantProblem(t, sendAs(t, f, f.Member, http.MethodPost, apiPrefix+"/timer/start", map[string]any{"project_id": tc.projectID, "timezone": "UTC"}),
				http.StatusUnprocessableEntity, tc.urn)
		})
	}
}
