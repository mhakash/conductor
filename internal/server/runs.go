package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"conductor/internal/engine"
	"conductor/internal/run"
)

func (s *Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	wf, err := s.workflows.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.repoError(w, r, err)
		return
	}
	started, err := s.engine.Start(r.Context(), wf)
	if err != nil {
		s.repoError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, started)
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.runs.List(r.Context(), run.ListFilter{
		WorkflowID: r.URL.Query().Get("workflowId"),
		Limit:      limit,
	})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	got, err := s.runs.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.runError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) handleRunLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	after, _ := strconv.ParseInt(q.Get("after"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	lines, err := s.runs.Logs(r.Context(), chi.URLParam(r, "id"), q.Get("step"), after, limit)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, lines)
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	if err := s.engine.Cancel(chi.URLParam(r, "id")); err != nil {
		s.runError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) runError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, run.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, engine.ErrNotActive):
		writeError(w, http.StatusConflict, "This run has already finished.")
	default:
		s.internalError(w, r, err)
	}
}
