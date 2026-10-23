# Go handlers generate the API contract; types are built, never stored

Request and response structs live next to the handlers (Huma), and the server emits OpenAPI 3.1 at `/api/v1/openapi.json`. The web build reads that document and runs `openapi-typescript` + `openapi-fetch` to produce a typed client in `web/src/lib/api/`, which is gitignored. The CLI and MCP server are in the same Go module and share the API's structs, so only the SPA needs generation.

Why: front end and back end ship as one artifact, so a second hand-maintained schema would be one more place to forget. Generating at build time means a renamed field breaks `svelte-check` instead of production. Errors use RFC 9457 problem+json with `type: urn:tstamps:error:<code>`, which gives stable machine-readable codes without a custom error schema.

Consequence: every Huma operation must declare its response shapes, including the error statuses it can return. Handlers outside Huma (CSV, healthz, static assets) are invisible to the contract and stay covered by tests instead of by types. If the API is ever needed by another consumer, the same `/openapi.json` is the input — no new source of truth.
