package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/havspect/project-times/internal/store"
)

// TestFail pins the one place a store rule becomes a status and a URN.
func TestFail(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
		urn  string
	}{
		{"not found", store.ErrNotFound, http.StatusNotFound, urnNotFound},
		{"wrapped not found", fmt.Errorf("load project: %w", store.ErrNotFound), http.StatusNotFound, urnNotFound},
		{"duplicate email", store.ErrDuplicateEmail, http.StatusUnprocessableEntity, urnInvalidInput},
		{"last admin", store.ErrLastAdmin, http.StatusUnprocessableEntity, urnLastAdmin},
		{"unknown user", fmt.Errorf("%w: 42", store.ErrUnknownUser), http.StatusUnprocessableEntity, urnInvalidInput},
		{"running entry", store.ErrEntryRunning, http.StatusConflict, urnTimerAlreadyRunning},
		{"unexpected failure", errors.New("disk is on fire"), http.StatusInternalServerError, urnInternal},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := fail(tt.err)
			if p.Status != tt.want || p.Type != tt.urn {
				t.Fatalf("fail(%v) = %d %s, want %d %s", tt.err, p.Status, p.Type, tt.want, tt.urn)
			}
		})
	}
}
