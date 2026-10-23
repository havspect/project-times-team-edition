# List the recipes.
default:
    @just --list

# Regenerate the API types from the Go OpenAPI document.
types:
    pnpm --dir web gen

# Run every check the build gate runs.
check:
    go vet ./...
    go test ./...
    pnpm --dir web check
    pnpm --dir web test

# Build the binary with the UI embedded.
build: types
    pnpm --dir web build
    go build -tags web -o times ./cmd/times

# Run the API and the Vite dev server together (needs hivemind: brew install hivemind).
dev: types
    hivemind Procfile.dev

# Run the API only; the UI is embedded in the binary.
serve:
    go run ./cmd/times serve
