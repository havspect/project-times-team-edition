// Command times is the Project Times server, CLI and MCP entry point.
package main

import (
	"fmt"
	"io"
	"os"
)

const version = "0.1.0-dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process, so tests drive subcommands directly.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: times <command> [flags]")
		return 2
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "times %s\n", version)
		return 0
	}

	fmt.Fprintln(stderr, "usage: times <command> [flags]")
	return 2
}
