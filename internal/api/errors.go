package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/project-times/internal/store"
)

// Stable machine-readable codes; the type URI is the contract, the prose is not.
const (
	urnInvalidCredentials     = "urn:times:error:invalid-credentials"
	urnUnauthorized           = "urn:times:error:unauthorized"
	urnInvalidOrigin          = "urn:times:error:invalid-origin"
	urnInternal               = "urn:times:error:internal"
	urnRateLimited            = "urn:times:error:rate-limited"
	urnForbidden              = "urn:times:error:forbidden"
	urnNotFound               = "urn:times:error:not-found"
	urnInvalidInput           = "urn:times:error:invalid-input"
	urnLastAdmin              = "urn:times:error:last-admin"
	urnPasswordChangeRequired = "urn:times:error:password-change-required"
	urnTimerAlreadyRunning    = "urn:times:error:timer-already-running"
	urnEntryTooLong           = "urn:times:error:entry-too-long"
	urnProjectHasEntries      = "urn:times:error:project-has-entries"
)

func problem(status int, urn, detail string) *huma.ErrorModel {
	return &huma.ErrorModel{Type: urn, Title: http.StatusText(status), Status: status, Detail: detail}
}

// fail maps a store error to the reply, so each rule code has one status, one URN
// and one wording. Handlers use it for errors they cannot explain better.
func fail(err error) *huma.ErrorModel {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return problem(http.StatusNotFound, urnNotFound, "not found")
	case errors.Is(err, store.ErrDuplicateEmail):
		return problem(http.StatusUnprocessableEntity, urnInvalidInput, "email is already in use")
	case errors.Is(err, store.ErrLastAdmin):
		return problem(http.StatusUnprocessableEntity, urnLastAdmin, "the last active admin cannot be demoted or deactivated")
	case errors.Is(err, store.ErrUnknownUser):
		return problem(http.StatusUnprocessableEntity, urnInvalidInput, "unknown user")
	case errors.Is(err, store.ErrEntryRunning):
		return problem(http.StatusConflict, urnTimerAlreadyRunning, "the user already has a running entry")
	}
	return internal(err)
}

// internal logs a store failure and answers 500, so a broken database never looks
// like a permission decision.
func internal(err error) *huma.ErrorModel {
	slog.Error("store", "error", err)
	return problem(http.StatusInternalServerError, urnInternal, "internal error")
}
