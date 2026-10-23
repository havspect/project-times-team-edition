# Project Times

Team time tracking in one binary: a SQLite database, a personal timeline for
logging, a report table for whoever needs the numbers, plus a REST API, a CLI and
an MCP server over the same API.

What it does and does not do is [docs/SPEC.md](docs/SPEC.md). The vocabulary is
[GLOSSARY.md](GLOSSARY.md); decisions worth not reversing are in
[docs/adr](docs/adr).

## Build

Requirements: Go 1.26+, [pnpm](https://pnpm.io), [just](https://github.com/casey/just).

```sh
just build          # generate API types, build the SPA, embed it, write ./times
sudo install -m 0755 times /usr/local/bin/times
```

`just build` is the only supported pipeline: it regenerates the OpenAPI types,
runs `vite build`, then compiles with `-tags web` so the UI is embedded. A plain
`go build ./cmd/times` works for API work but serves a "UI is not built" page.

## First run

```sh
times admin create --email you@example.com --name "Your Name" --db ./times.db
times serve --addr :8080 --db ./times.db
```

Open `http://localhost:8080` and log in with the initial password the first
command printed. Change it when prompted, then create teams, users and
memberships under **/admin**. The CLI reads those; it does not create them.

## CLI

The CLI talks to the API, never to the database. Log in once per machine; the
token is written to `~/.config/times/config.json` with mode `0600`.

```sh
times login --url http://localhost:8080

times whoami
times projects add Timeline --team Platform
times add Timeline --date today --from 09:00 --to 11:30 --note "drag, resize"
times start Timeline --note "OAuth callback"
times stop
times ls
times report --from 2026-10-19 --to 2026-10-26 --by week
times report --from 2026-10-19 --to 2026-10-26 --csv > week.csv
```

A user an admin created or reset must change the initial password in the web UI
before `times login` can mint a token.

Times without a date mean today in your timezone; `--to 1h30m` and
`--to 01:00` (crossing midnight) both work. `times help` lists every command.
Exit codes: `0` ok, `1` error, `2` usage.

Server settings come from flags, then the environment, then defaults:
`--addr`/`TIMES_ADDR` (default `:8080`) and `--db`/`TIMES_DB` (default
`./times.db`). Client settings come from `--url`/`--token`, then `TIMES_URL`/
`TIMES_TOKEN`, then the config file.

## MCP

`times mcp` speaks MCP over stdio and uses the same config and token as the CLI.
Read tools are annotated read-only; each description states the permission it
needs, so an agent can explain a refusal.

```json
{
  "mcpServers": {
    "times": {
      "command": "times",
      "args": ["mcp"],
      "env": {
        "TIMES_URL": "https://times.example.com",
        "TIMES_TOKEN": "ts_..."
      }
    }
  }
}
```

Tools: `whoami`, `list_projects`, `list_entries`, `start_timer`, `stop_timer`,
`add_entry`, `update_entry`, `delete_entry`, `report`, `list_teams` (admin),
`list_users` (admin). Mint a token for an agent with `times tokens create agent`;
it is shown once.

## Backup

Everything lives in one SQLite file (`times.db` by default) plus its `-wal` and
`-shm` companions while the server runs.

- **With the server stopped:** copy `times.db` (and the WAL/SHM files if they
  are still there), or copy the whole directory.
- **With the server running:** use SQLite's online backup, which is safe under
  WAL:

  ```sh
  sqlite3 times.db ".backup 'times-backup.db'"
  ```

- **Restore:** stop the server, replace `times.db` (remove stale `-wal`/`-shm`),
  start the server. Migrations run at boot.
- The client config holds a bearer token and is a credential, not data worth
  backing up.

Nothing is hard-deleted: deleted entries keep their row with `deleted_at`, and
archived projects keep their hours, so an admin can restore either without a
backup.

## Upgrade

Stop the server, take a backup, install the new binary, start it. `AutoMigrate`
runs at boot; `GET /healthz` reports the schema version. There is no downgrade
path — restore the backup to go back.

## Development

```sh
just check      # go vet, go test, svelte-check, vitest
just dev        # API plus Vite dev server (needs hivemind)
```

`go test ./...` passes offline with no external services. The end-to-end test
builds the binary, serves on a free port, bootstraps an admin, logs an entry,
exports CSV and calls the MCP server over stdio.
