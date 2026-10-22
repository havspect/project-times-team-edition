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
	"github.com/havspect/project-times/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process, so tests drive subcommands directly.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return usage(stderr)
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "times %s\n", version.Version)
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
	}

	return usage(stderr)
}

func usage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: times <version|serve|openapi|admin>")
	return 2
}
