# Project Times — specification

A team time tracker in the spirit of timeTagger, minus the one-person focus: a personal infinite timeline for logging, a plain table for whoever needs the numbers. One Go binary, one SQLite file, one workspace.

Vocabulary lives in [GLOSSARY.md](../GLOSSARY.md). Decisions that are expensive to reverse live in [docs/adr](./adr/).

## Scope

In v1:

- Users log hours on a personal timeline, with one running entry at a time.
- Leads create projects for their teams and read hours logged on those projects.
- Admins manage users, teams, memberships, and restore deleted data.
- Reports per person / project / day or week, plus CSV.
- A REST API, a CLI, and an MCP server (stdio), all over the same HTTP API.

Out of v1, deliberately: approvals, leave and holidays, invoicing and rates, reminders and notifications, mobile app, screenshots, Jira/Linear sync, realtime multi-user editing, project budgets, a team-wide timeline, SSO, audit log, i18n.

## Roles

A user has exactly one of two global states — admin or not — plus zero or more team memberships, each with role `lead` or `member`.

| Action | Member | Lead | Admin |
| --- | --- | --- | --- |
| Log / edit / delete own entries | yes | yes | yes |
| Read own hours | yes | yes | yes |
| Read hours on their teams' projects | no | yes | yes |
| Create / edit / archive projects | no | their teams | all |
| Manage teams and memberships | no | no | yes |
| Create / deactivate users, reset passwords | no | no | yes |
| Restore deleted entries / archived projects | no | no | yes |
| Any project picker | all non-archived | all non-archived | all |

A lead's report scope is *entries booked on projects owned by a team they lead*, whoever logged them, plus their own entries. Admin scope is everything.

## Domain model

```
users(id, email, name, password_hash, timezone, is_admin, active, must_change_password, created_at, updated_at)
teams(id, name, created_at, updated_at, archived_at)
team_memberships(id, team_id, user_id, role, created_at)         unique(team_id, user_id)
projects(id, team_id, name, color, created_at, updated_at, archived_at)
entries(id, user_id, project_id, start_at, end_at, note, created_at, updated_at, deleted_at)
sessions(id, user_id, token_hash, expires_at, created_at)
api_tokens(id, user_id, name, token_hash, created_at, last_used_at, revoked_at)
```

Rules the store enforces, with tests:

- A user has at most one entry with `end_at IS NULL` and `deleted_at IS NULL` (partial unique index).
- `end_at > start_at`, and a finished entry is no longer than 24 hours. Stopping a running entry that would exceed 24 hours returns `422 entry-too-long` and leaves it running, so the user picks an end time instead of losing hours.
- Start and end may not be more than 5 minutes in the future.
- Overlapping entries are allowed. The timeline marks them; the API does not reject them, because parallel work is real.
- Entries may span midnight; reports split them at the subject's local midnight.
- Archived projects reject new entries but keep existing ones. Archiving a team archives its projects in the same step; the membership rows stay but grant nothing. A project of an archived team cannot be restored (`422 invalid-input`) until the team is live again.
- A project's team cannot change once the project has entries (`422 project-has-entries`); while empty it can.
- Deactivated users cannot log in or use tokens; their hours stay. Deactivating a user stops their running entry at that instant, with the note `stopped on deactivation`.
- A user created or reset by an admin carries `must_change_password`; until it clears, only `/me`, `/me/password` and `/auth/logout` work (`403 password-change-required`).
- A password change or reset ends every other login of that user in the same transaction: the sessions are deleted and every live token is revoked. A self-change keeps only the session that made it, so a token client logs in again.
- The last active admin cannot be demoted or deactivated (`422 last-admin`).
- Deleting an entry sets `deleted_at`; restoring clears it. No hard deletes anywhere.

Email comparison is case-insensitive; emails are stored lowercased.

## Time and reporting

All instants are UTC. Each user has an IANA timezone; days and ISO weeks are computed in the timezone of the person whose hours they are ([ADR-0003](./adr/0003-user-timezones-decide-buckets.md)).

- A report takes `from` and `to` as plain dates, `from` inclusive, `to` exclusive.
- Each query fetches entries overlapping `[from−14h, to+14h)` in UTC, then buckets and splits them in Go.
- Buckets are `week` (ISO, Monday start), `day` (subject-local date), or `none` (whole range).
- A 12-hour entry crossing local midnight becomes two rows, and their seconds still sum to 12h across DST shifts.
- Reports include hours on archived projects, labelled as archived. They exclude deleted entries.

CSV export is the detail behind the table: one row per entry per local day.

```csv
date,iso_week,user,email,team,project,start,end,seconds,note
2026-10-19,2026-W43,Ada Lovelace,ada@example.com,Platform,Timeline,2026-10-19T09:00:00+02:00,2026-10-19T11:30:00+02:00,9000,"drag, resize, snap"
```

## HTTP API

`/api/v1`, JSON in and out, generated from the Go handlers by Huma. The OpenAPI 3.1 document is served at `/api/v1/openapi.json` and drives both the web client's types ([ADR-0006](./adr/0006-generated-api-contract.md)) and the API reference at `/api/v1/docs`. Huma validates bodies and query parameters from struct tags (`422` for malformed input); authentication failures stay `401`/`403` from middleware.

Errors are RFC 9457 problem+json. The stable machine-readable code lives in the `type` URI, not in prose:

```json
{"type":"urn:times:error:timer-already-running","title":"Conflict","status":409,"detail":"Nadia already has a running entry.","instance":"/api/v1/timer/start"}
```

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/auth/login` | email + password, sets session cookie |
| POST | `/auth/logout` | clears session |
| GET | `/me` | current user, memberships, admin flag |
| POST | `/me/password` | change own password |
| GET/POST | `/me/tokens` | list / create personal API tokens |
| DELETE | `/me/tokens/{id}` | revoke |
| GET/POST | `/users` | admin: list / create |
| PATCH | `/users/{id}` | admin: name, timezone, admin flag, active |
| POST | `/users/{id}/password` | admin: reset |
| GET/POST | `/teams` | list / create (admin) |
| PATCH/DELETE | `/teams/{id}` | rename / archive, which archives its projects (admin) |
| PUT | `/teams/{id}/members` | admin: replace membership list `[{user_id, role}]` |
| GET/POST | `/projects` | list / create (lead for own team, admin) |
| PATCH | `/projects/{id}` | name, color, archive; team only while it has no entries |
| POST | `/projects/{id}/restore` | admin |
| GET | `/entries?from&to[&user_id]` | range query, default current week, max 366 days |
| POST | `/entries` | create a finished entry |
| PATCH | `/entries/{id}` | move, retime, re-project, re-note |
| DELETE | `/entries/{id}` | soft delete |
| POST | `/entries/{id}/restore` | admin |
| GET | `/timer` | current running entry or null |
| POST | `/timer/start` | `{project_id, note}`; 409 with the running entry if one exists |
| POST | `/timer/stop` | ends at now, optional note |
| GET | `/reports?from&to&by=week\|day\|none[&team_id][&user_id][&project_id]` | aggregated seconds |
| GET | `/reports.csv` | same filters, entry-per-day rows |
| GET | `/healthz` | liveness + schema version |
| GET | `/openapi.json` | OpenAPI 3.1 document, generated from handlers |
| GET | `/docs` | browsable API reference |

Authentication: `POST /auth/login` returns an opaque session token in an `HttpOnly`, `SameSite=Lax` cookie, 30-day fixed expiry, stored hashed. Cookies are `Secure` unless `--insecure-cookies` is passed, so production sits behind a TLS-terminating proxy. Every other request may instead send `Authorization: Bearer ts_...`. Tokens are shown once at creation, stored as SHA-256, and act as the owning user. Login is rate limited per IP in memory. A user with `must_change_password` can authenticate but is confined to the three endpoints listed in the rules above.

Mutations over the cookie require a same-origin `Origin` header. There is no CORS: the SPA is served from the same origin.

Admin edits are not audited in v1. That is an accepted risk, recorded here rather than solved.

## CLI

Talks to the API, never to the database. Config from `--url` / `--token`, then `TIMES_URL` / `TIMES_TOKEN`, then `~/.config/times/config.json` (mode 0600). The CLI and the MCP server share `internal/client` and the API's Go structs, so neither needs generated code.

```
times serve [--addr :8080] [--db ./times.db]     # runs the server (web UI + API)
times admin create --email x --name y            # bootstraps the first admin, prints an initial password
times login --url https://times.example.com      # password prompt, stores a personal token
times whoami
times start <project> [--note "..."]             # project matched by name, then id
times stop [--note "..."]
times add <project> --from 09:00 --to 11:30 [--date today] [--note "..."]
times ls [--from 2026-10-19 --to 2026-10-26] [--user me|email] [--json]
times edit <id> [--project ...] [--from ...] [--to ...] [--note ...]
times rm <id>
times projects [--all] | times projects add <name> --team <team> | times projects archive <id>
times teams | times teams members <team>
times report --from 2026-10-19 --to 2026-10-26 [--by week|day] [--json|--csv]
times tokens create <name> | times tokens ls | times tokens rm <id>
times mcp                                        # MCP server on stdio
times version
```

Times without a date mean today in the caller's timezone. Durations are accepted as `1h30m`. Exit codes: 0 ok, 1 error, 2 usage.

## MCP

`times mcp` speaks MCP over stdio and uses the same config and token as the CLI. Tools mirror the API, with read tools annotated read-only:

`whoami`, `list_projects`, `list_entries`, `start_timer`, `stop_timer`, `add_entry`, `update_entry`, `delete_entry`, `report`, `list_teams`, `list_users` (admin).

Tool descriptions state the permission each one needs, so an agent can explain a refusal instead of retrying. No MCP resources; the API is the source of truth.

## Web UI

Svelte 5 + Vite, built to static assets and embedded with `go:embed`. The SPA calls the same API; no server-side rendering.

- **/timeline** — personal canvas timeline, infinite horizontally, wheel to zoom (day ↔ week), drag to create, drag edges to retime, drag body to move, click to edit project and note, running entry pinned at "now", overlaps marked, project colors, 15-minute snap with `Alt` for 1-minute precision, `space` start/stop, `n` new, `Esc` cancel, `Delete` remove.
- **/reports** — date range, team, person, project filters; totals table per person × project; week/day toggle; total row; CSV button. Read-only; leads and admins only.
- **/admin/users**, **/admin/teams**, **/admin/projects** — tables with inline create and edit. No separate settings screen; the workspace name comes from config.

Leads do not get a team timeline in v1; the report table answers their question.

## Layout and stack

```
cmd/times/          main, subcommand dispatch
internal/config/    flags, env, config file
internal/store/     GORM models, AutoMigrate, queries
internal/auth/      argon2id passwords, sessions, tokens, rate limit
internal/entry/     entry and timer rules
internal/report/    bucketing, aggregation, CSV
internal/api/       Huma operations, middleware, permission checks
internal/client/    typed HTTP client shared by CLI and MCP
internal/cli/       subcommands
internal/mcp/       MCP tool definitions
web/                Svelte 5 + Vite source; web/dist is embedded
web/src/lib/api/    generated schema + typed fetch client (gitignored)
```

Go stdlib `net/http` behind Huma's adapter. `log/slog` for logs. No DI framework: `main` wires a `store`, an `auth`, and an `api.Server` explicitly. The GORM models are the schema: `AutoMigrate` runs at boot, followed by one raw `CREATE UNIQUE INDEX ... WHERE end_at IS NULL` for the single running entry, which tags cannot express ([ADR-0005](./adr/0005-gorm-pure-go-sqlite.md)).

One build pipeline, in this order: `go run ./cmd/times openapi > web/openapi.json`, `openapi-typescript` into `web/src/lib/api/schema.d.ts`, `vite build` into `web/dist`, `go build` embeds it. `make dev` and `make build` run exactly that; no generated types are committed.

## Testing

`go test ./...` must pass offline, with no external services.

- **entry/report unit tests** — local-midnight splits, DST spring-forward and fall-back, ISO week edges around New Year, 14-hour padding, overlap detection, CSV escaping.
- **store tests** — `AutoMigrate` on an empty temp-dir database, a second boot being a no-op, the partial index actually rejecting a second running entry, archived-project filtering, deleted-entry exclusion, soft delete and restore.
- **API tests** — `httptest` with the full router; table-driven permission matrix (member / lead of that team / lead of another team / admin) over every endpoint; session and bearer auth incl. expired and revoked; range validation; 409 on double start. One test asserts `/openapi.json` lists every route in the spec table, so a missing Huma registration fails CI.
- **CLI tests** — subcommands against a fake `client.Interface`; one end-to-end test builds the binary, bootstraps an admin, serves on `:0`, logs in, creates a project and entry, reads a report and a CSV, and performs an MCP initialize + tool call over stdio.
- **Frontend** — `pnpm run check` (svelte-check + tsc) runs against freshly generated types, so an API change the UI did not follow fails the build. Vitest covers timeline math (pixels ↔ time, snapping, resize clamps). Browser tests are deferred; the timeline gets a manual checklist in the PR until they exist.

Time is injected (`Clock` interface) anywhere "now" matters; tests never sleep.

## Build order

1. **Walking skeleton** — config, SQLite, AutoMigrate, Huma + `/openapi.json`, `users`, password login, session cookie, `/healthz`, `times serve`, `times admin create`, first tests.
2. **Structure** — teams, memberships, projects, permission middleware, personal API tokens, `times login`.
3. **Time** — entries, timer, overlap handling, report bucketing, CSV, permission matrix tests.
4. **Web** — login, timeline, reports, admin screens, typed client generation, embedded build.
5. **Agents** — CLI completion, MCP tools, end-to-end test, README with install and backup notes.

v1 is done when a fresh install can: create an admin from the CLI, add two teams and a lead each, create a member, log a day on the timeline, start and stop the timer from the CLI, export the week as CSV, and let an MCP client report last week's hours.
