package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/havspect/project-times/internal/api"
	"github.com/havspect/project-times/internal/clock"
	"github.com/havspect/project-times/internal/config"
	"github.com/havspect/project-times/internal/store"
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
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("serving", "addr", cfg.Addr, "db", cfg.DB)

	listenErr := make(chan error, 1)
	go func() { listenErr <- srv.ListenAndServe() }()

	select {
	case err := <-listenErr:
		if errors.Is(err, http.ErrServerClosed) {
			return 0
		}
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
