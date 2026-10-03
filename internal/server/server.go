// Package server wires HTTP routes, middleware and the embedded UI.
package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"conductor/internal/version"
)

type Server struct {
	log     *slog.Logger
	ui      fs.FS
	started time.Time
}

// New returns a server. ui may be nil, in which case only the API is served.
func New(log *slog.Logger, ui fs.FS) *Server {
	return &Server{log: log, ui: ui, started: time.Now()}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(s.log))
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
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
