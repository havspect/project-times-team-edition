// Package config resolves the server settings from flags and environment.
package config

import (
	"flag"
	"io"
)

// Config holds the server settings every command that opens the database needs.
type Config struct {
	Addr string // listen address
	DB   string // sqlite file path
}

const (
	DefaultAddr = ":8080"
	DefaultDB   = "./tstamps.db"
)

// Load resolves flags over env over defaults. getenv is injected so tests need no
// process environment. args excludes the program name.
func Load(args []string, getenv func(string) string) (Config, error) {
	envOr := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		Addr: envOr("TSTAMPS_ADDR", DefaultAddr),
		DB:   envOr("TSTAMPS_DB", DefaultDB),
	}

	fs := flag.NewFlagSet("tstamps", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller reports the error, so no flag chatter on stderr
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "listen address")
	fs.StringVar(&cfg.DB, "db", cfg.DB, "sqlite file path")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
