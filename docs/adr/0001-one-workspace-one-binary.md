# One workspace per install, shipped as one binary

A workspace is one company. The Go binary serves the HTTP API, the embedded web UI and an embedded SQLite file; there is no separate database, cache or frontend host. Self-hosting is `times serve --db ./times.db`, and a backup is a file copy.

Considered multi-tenant SaaS (rejected: every table and query grows an org scope, and cross-tenant leaks are the expensive failure mode) and Postgres (rejected: kills the single-binary install for a team-sized product).

Consequence: one server process per workspace, SQLite's single-writer limit sets the ceiling, and horizontal scaling is out of scope by design.
