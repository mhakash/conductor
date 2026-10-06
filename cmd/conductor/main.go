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
	"conductor/internal/engine"
	"conductor/internal/events"
	"conductor/internal/secrets"
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

	runs := store.NewRuns(db)

	key, err := secrets.LoadKey(cfg.DataDir, log)
	if err != nil {
		return err
	}
	cipher, err := secrets.NewCipher(key)
	if err != nil {
		return err
	}
	secretStore := secrets.NewStore(db, cipher)

	workflows := store.NewWorkflows(db)
	blocks := block.Builtin()
	broker := events.NewBroker()
	eng := engine.New(engine.Options{
		Runs:        runs,
		Workflows:   workflows.Get,
		Blocks:      blocks,
		Events:      broker,
		Secrets:     secretStore,
		Log:         log,
		MaxParallel: cfg.MaxParallel,
	})
	if err := eng.Recover(ctx); err != nil {
		return err
	}

	ui := web.UI()
	if cfg.Dev {
		ui = nil
	}

	srv := server.New(server.Deps{
		Log:       log,
		Workflows: workflows,
		Presets:   store.NewPresets(db),
		Runs:      runs,
		Engine:    eng,
		Broker:    broker,
		Secrets:   secretStore,
		Blocks:    blocks,
		UI:        ui,
	})
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	httpServer.RegisterOnShutdown(srv.CloseStreams)

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
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown", "err", err)
	}
	// Cancel in-flight runs and let them record their final state before the
	// database closes.
	return eng.Shutdown(shutdownCtx)
}
