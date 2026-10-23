// Package config resolves the server settings from flags and environment.
package config

import (
	"flag"
	"io"
	"log/slog"
)

// Config holds the server settings every command that opens the database needs.
type Config struct {
	Addr     string     // listen address
	DB       string     // sqlite file path
	LogLevel slog.Level // minimum level written to stderr
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
		Addr:     envOr("TSTAMPS_ADDR", DefaultAddr),
		DB:       envOr("TSTAMPS_DB", DefaultDB),
		LogLevel: slog.LevelInfo,
	}
	if v := getenv("TSTAMPS_LOG_LEVEL"); v != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			return Config{}, err
		}
	}

	fs := flag.NewFlagSet("tstamps-server", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller reports the error, so no flag chatter on stderr
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "listen address")
	fs.StringVar(&cfg.DB, "db", cfg.DB, "sqlite file path")
	level := fs.String("log-level", cfg.LogLevel.String(), "debug, info, warn or error")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(*level)); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
