package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Stable machine-readable codes; the type URI is the contract, the prose is not.
const (
	urnInvalidCredentials = "urn:times:error:invalid-credentials"
	urnUnauthorized       = "urn:times:error:unauthorized"
	urnInvalidOrigin      = "urn:times:error:invalid-origin"
)

func problem(status int, urn, detail string) error {
	return &huma.ErrorModel{Type: urn, Title: http.StatusText(status), Status: status, Detail: detail}
}
