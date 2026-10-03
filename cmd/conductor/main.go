package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"conductor/internal/block"
	"conductor/internal/config"
	"conductor/internal/server"
	"conductor/internal/store"
	"conductor/internal/version"
	"conductor/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "conductor:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}

	level := slog.LevelInfo
	if cfg.Dev {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, filepath.Join(cfg.DataDir, "conductor.db"), log)
	if err != nil {
		return err
	}
	defer db.Close()

	ui := web.UI()
	if cfg.Dev {
		ui = nil
	}

	httpServer := &http.Server{
		Addr: cfg.Addr,
		Handler: server.New(server.Deps{
			Log:       log,
			Workflows: store.NewWorkflows(db),
			Blocks:    block.Builtin(),
			UI:        ui,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("conductor started",
			"addr", "http://"+cfg.Addr,
			"version", version.Version,
			"data_dir", cfg.DataDir,
			"ui_embedded", ui != nil,
		)
		errc <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
