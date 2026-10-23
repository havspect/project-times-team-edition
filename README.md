# Tstamps

Team time tracking for one workspace: a server binary holding the SQLite
database, the web timeline and the REST API, and a CLI that every user installs
on their own machine and points at that server. The CLI also speaks MCP.

What it does and does not do is [docs/SPEC.md](docs/SPEC.md). The vocabulary is
[GLOSSARY.md](GLOSSARY.md); decisions worth not reversing are in
[docs/adr](docs/adr), including
[why the server and the CLI are two binaries](docs/adr/0008-user-cli-and-server-are-two-binaries.md).

## Install the CLI

`tstamps` needs no database and no Go toolchain. Take the archive for your
machine from the [latest release](https://github.com/havspect/tstamps/releases/latest)
and put the binary on your `PATH`:

```sh
# Linux amd64 shown; swap amd64 for arm64, or tar for the windows zip.
curl -fsSL https://github.com/havspect/tstamps/releases/latest/download/tstamps_linux_amd64.tar.gz | tar -xz
sudo install -m 0755 tstamps /usr/local/bin/tstamps
```

Assets are `tstamps_{linux,darwin}_{amd64,arm64}.tar.gz`,
`tstamps_windows_amd64.zip` and a `checksums.txt`. Then log in against your
server, as shown under [CLI](#cli).

## Run a workspace

The server is `tstamps-server`; it embeds the web UI, so it is built where it
runs. Requirements: Go 1.26+, [pnpm](https://pnpm.io), [just](https://github.com/casey/just).

```sh
just build          # generate API types, build the SPA, embed it, write ./tstamps-server
sudo install -m 0755 tstamps-server /usr/local/bin/tstamps-server
```

`just build` regenerates the OpenAPI types, runs `vite build`, then compiles with
`-tags web` so the UI is embedded. `just build-prod` does the same with
`CGO_ENABLED=0`, `-trimpath` and stripped symbols, which is what you deploy. A
plain `go build ./cmd/tstamps-server` works for API work but serves a "UI is not
built" page. `just build-cli` builds `./tstamps` for local testing of the CLI.

## First run

```sh
tstamps-server admin create --email you@example.com --name "Your Name" --db ./tstamps.db
tstamps-server serve --addr :8080 --db ./tstamps.db
```

Open `http://localhost:8080` and log in with the initial password the first
command printed. Change it when prompted, then create teams, users and
memberships under **/admin**. The CLI reads those; it does not create them.

## CLI

The CLI talks to the API, never to the database. Log in once per machine; the
token is written to `~/.config/tstamps/config.json` with mode `0600`.

```sh
tstamps login --url http://localhost:8080

tstamps whoami
tstamps projects add Timeline --team Platform
tstamps add Timeline --date today --from 09:00 --to 11:30 --note "drag, resize"
tstamps start Timeline --note "OAuth callback"
tstamps stop
tstamps ls
tstamps report --from 2026-10-19 --to 2026-10-26 --by week
tstamps report --from 2026-10-19 --to 2026-10-26 --csv > week.csv
```

A user an admin created or reset must change the initial password in the web UI
before `tstamps login` can mint a token.

Times without a date mean today in your timezone; `--to 1h30m` and
`--to 01:00` (crossing midnight) both work. `tstamps help` lists every command.
Exit codes: `0` ok, `1` error, `2` usage.

Server settings come from flags, then the environment, then defaults:
`--addr`/`TSTAMPS_ADDR` (default `:8080`) and `--db`/`TSTAMPS_DB` (default
`./tstamps.db`). Client settings come from `--url`/`--token`, then `TSTAMPS_URL`/
`TSTAMPS_TOKEN`, then the config file.

## MCP

`tstamps mcp` speaks MCP over stdio and uses the same config and token as the CLI.
Read tools are annotated read-only; each description states the permission it
needs, so an agent can explain a refusal.

```json
{
  "mcpServers": {
    "tstamps": {
      "command": "tstamps",
      "args": ["mcp"],
      "env": {
        "TSTAMPS_URL": "https://tstamps.example.com",
        "TSTAMPS_TOKEN": "ts_..."
      }
    }
  }
}
```

Tools: `whoami`, `list_projects`, `list_entries`, `start_timer`, `stop_timer`,
`add_entry`, `update_entry`, `delete_entry`, `report`, `list_teams` (admin),
`list_users` (admin). Mint a token for an agent with `tstamps tokens create agent`;
it is shown once.

## Backup

Everything lives in one SQLite file (`tstamps.db` by default) plus its `-wal` and
`-shm` companions while the server runs.

- **With the server stopped:** copy `tstamps.db` (and the WAL/SHM files if they
  are still there), or copy the whole directory.
- **With the server running:** use SQLite's online backup, which is safe under
  WAL:

  ```sh
  sqlite3 tstamps.db ".backup 'tstamps-backup.db'"
  ```

- **Restore:** stop the server, replace `tstamps.db` (remove stale `-wal`/`-shm`),
  start the server. Migrations run at boot.
- The client config holds a bearer token and is a credential, not data worth
  backing up.

Nothing is hard-deleted: deleted entries keep their row with `deleted_at`, and
archived projects keep their hours, so an admin can restore either without a
backup.

## Upgrade

Stop the server, take a backup, install the new `tstamps-server`, start it. `AutoMigrate`
runs at boot; `GET /healthz` reports the schema version. There is no downgrade
path — restore the backup to go back.

## Development

```sh
just check       # go vet, go test, svelte-check, vitest
just dev         # API plus Vite dev server, both hot reloading
just serve-watch # API only, hot reloaded (UI comes from Vite)
```

`just dev` needs [hivemind](https://github.com/DarthSim/hivemind) and
[air](https://github.com/air-verse/air) (`go install
github.com/air-verse/air@latest`). Air rebuilds and restarts the API on Go
changes; Vite hot-reloads the UI on the same page.

`go test ./...` passes offline with no external services. The end-to-end test
builds the binary, serves on a free port, bootstraps an admin, logs an entry,
exports CSV and calls the MCP server over stdio.
