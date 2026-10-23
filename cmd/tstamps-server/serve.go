package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/havspect/tstamps/internal/api"
	"github.com/havspect/tstamps/internal/clock"
	"github.com/havspect/tstamps/internal/config"
	"github.com/havspect/tstamps/internal/store"
)

func serve(args []string, stderr io.Writer) int {
	log := slog.New(slog.NewTextHandler(stderr, nil))

	cfg, err := config.Load(args, os.Getenv)
	if err != nil {
		log.Error("invalid flags", "error", err)
		return 2
	}

	st, err := store.Open(cfg.DB)
	if err != nil {
		log.Error("open database", "db", cfg.DB, "error", err)
		return 1
	}
	defer st.Close()

	if err := st.Migrate(); err != nil {
		log.Error("migrate database", "db", cfg.DB, "error", err)
		return 1
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.New(st, clock.System{}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		log.Error("listen", "addr", cfg.Addr, "error", err)
		return 1
	}

	log.Info("serving", "addr", ln.Addr().String(), "db", cfg.DB)

	listenErr := make(chan error, 1)
	go func() { listenErr <- srv.Serve(ln) }()

	select {
	case err := <-listenErr:
		log.Error("listen", "addr", cfg.Addr, "error", err)
		return 1
	case <-ctx.Done():
	}

	stop() // the second signal kills the process instead of waiting again

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "error", err)
		return 1
	}
	log.Info("stopped")
	return 0
}
