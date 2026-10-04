// Package server wires HTTP routes, middleware and the embedded UI.
package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"conductor/internal/block"
	"conductor/internal/engine"
	"conductor/internal/events"
	"conductor/internal/run"
	"conductor/internal/secrets"
	"conductor/internal/version"
	"conductor/internal/workflow"
)

type Deps struct {
	Log       *slog.Logger
	Workflows workflow.Repository
	Runs      run.Repository
	Engine    *engine.Engine
	Broker    *events.Broker
	Secrets   *secrets.Store
	Blocks    *block.Registry
	// UI is the built frontend; nil serves only the API.
	UI fs.FS
}

type Server struct {
	log       *slog.Logger
	workflows workflow.Repository
	runs      run.Repository
	engine    *engine.Engine
	broker    *events.Broker
	secrets   *secrets.Store
	blocks    *block.Registry
	ui        fs.FS
	started   time.Time

	// closing is closed on shutdown to end long-lived event streams, which
	// would otherwise keep http.Server.Shutdown waiting.
	closing   chan struct{}
	closeOnce sync.Once
}

func New(d Deps) *Server {
	return &Server{
		log:       d.Log,
		workflows: d.Workflows,
		runs:      d.Runs,
		engine:    d.Engine,
		broker:    d.Broker,
		secrets:   d.Secrets,
		closing:   make(chan struct{}),
		blocks:    d.Blocks,
		ui:        d.UI,
		started:   time.Now(),
	}
}

// CloseStreams ends all open event streams. Register it with
// http.Server.RegisterOnShutdown.
func (s *Server) CloseStreams() {
	s.closeOnce.Do(func() { close(s.closing) })
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(s.log))
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/blocks", s.handleListBlocks)
		r.Route("/workflows", func(r chi.Router) {
			r.Get("/", s.handleListWorkflows)
			r.Post("/", s.handleCreateWorkflow)
			r.Get("/{id}", s.handleGetWorkflow)
			r.Put("/{id}", s.handleUpdateWorkflow)
			r.Delete("/{id}", s.handleDeleteWorkflow)
			r.Post("/{id}/runs", s.handleStartRun)
		})
		r.Route("/secrets", func(r chi.Router) {
			r.Get("/", s.handleListSecrets)
			r.Put("/{name}", s.handlePutSecret)
			r.Delete("/{name}", s.handleDeleteSecret)
		})
		r.Route("/runs", func(r chi.Router) {
			r.Get("/", s.handleListRuns)
			r.Get("/{id}", s.handleGetRun)
			r.Get("/{id}/logs", s.handleRunLogs)
			r.Get("/{id}/events", s.handleRunEvents)
			r.Post("/{id}/cancel", s.handleCancelRun)
		})
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusNotFound, "no such endpoint")
		})
	})

	if s.ui != nil {
		r.Handle("/*", spaHandler(s.ui))
	}
	return r
}

type healthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	StartedAt string `json:"startedAt"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Version:   version.Version,
		Commit:    version.Commit,
		StartedAt: s.started.UTC().Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
