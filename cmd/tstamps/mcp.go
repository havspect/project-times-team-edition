package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/havspect/tstamps/internal/client"
	"github.com/havspect/tstamps/internal/config"
	"github.com/havspect/tstamps/internal/mcp"
)

// mcpCommand runs the MCP server over stdio, with the same config and token as
// every other client command. Nothing may be written to stdout but protocol
// frames, so diagnostics go to stderr.
func mcpCommand(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller reports the error in one line
	urlFlag := fs.String("url", "", "server URL")
	tokenFlag := fs.String("token", "", "personal API token")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "tstamps mcp: %v\n", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: tstamps mcp [--url URL] [--token TOKEN]")
		return 2
	}

	cfg, err := config.LoadClient(*urlFlag, *tokenFlag, os.Getenv)
	if err != nil {
		fmt.Fprintf(stderr, "tstamps mcp: %v\n", err)
		return 1
	}
	if cfg.URL == "" {
		fmt.Fprintln(stderr, "tstamps mcp: no server URL; run tstamps login --url URL or set TSTAMPS_URL")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := mcp.Serve(ctx, client.New(cfg)); err != nil {
		fmt.Fprintf(stderr, "tstamps mcp: %v\n", err)
		return 1
	}
	return 0
}
