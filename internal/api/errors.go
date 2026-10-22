package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
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
	urnLastAdmin              = "urn:times:error:last-admin"
	urnPasswordChangeRequired = "urn:times:error:password-change-required"
)

func problem(status int, urn, detail string) *huma.ErrorModel {
	return &huma.ErrorModel{Type: urn, Title: http.StatusText(status), Status: status, Detail: detail}
}
