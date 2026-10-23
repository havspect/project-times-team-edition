// Command times is the Project Times server, CLI and MCP entry point.
package main

import (
	"fmt"
	"io"
	"os"
	// Users pick an IANA timezone; the zoneinfo tree is not guaranteed to exist
	// on the hosts this ships to, so the database is embedded.
	_ "time/tzdata"

	"github.com/havspect/project-times/internal/api"
	"github.com/havspect/project-times/internal/cli"
	"github.com/havspect/project-times/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process, so tests drive subcommands directly.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "times %s\n", version.Version)
		return 0
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	case "serve":
		return serve(args[1:], stderr)
	case "openapi":
		spec, err := api.Spec()
		if err != nil {
			fmt.Fprintf(stderr, "times openapi: %v\n", err)
			return 1
		}
		if _, err := stdout.Write(spec); err != nil {
			fmt.Fprintf(stderr, "times openapi: write spec: %v\n", err)
			return 1
		}
		return 0
	case "admin":
		return admin(args[1:], stdout, stderr)
	case "mcp":
		return mcpCommand(args[1:], stderr)
	}

	// Everything else is a client command against a running server.
	return cli.New(stdout, stderr).Run(args)
}

const usageText = `usage: times <command> [flags]

server:
  serve                      run the web UI and API (--addr, --db)
  admin create               bootstrap an admin (--email, --name, --db)
  openapi                    print the OpenAPI document
  version                    print the version

client (needs a token from "times login"):
  login --url URL            store a personal API token
  whoami                     show the current user
  start <project>            start the running entry (--note)
  stop                       stop the running entry (--note)
  add <project>              log a finished entry (--from, --to, --date, --note)
  ls                         list entries (--from, --to, --user, --json)
  edit <id>                  change an entry (--project, --from, --to, --note)
  rm <id>                    delete an entry
  projects                   list projects (--all, --json)
  projects add               create a project (--team, --color)
  projects archive <id>      archive a project
  teams                      list teams (--json)
  teams members <team>       list a team's members
  report                     hours totals (--from, --to, --by, --json, --csv)
  tokens create|ls|rm        manage personal API tokens
  mcp                        MCP server on stdio

Exit codes: 0 ok, 1 error, 2 usage.
`

func usage(w io.Writer) {
	fmt.Fprint(w, usageText)
}
