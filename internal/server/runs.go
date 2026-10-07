package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"conductor/internal/engine"
	"conductor/internal/run"
	"conductor/internal/workflow"
)

func (s *Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	wf, err := s.workflows.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.repoError(w, r, err)
		return
	}
	var body struct {
		Inputs map[string]any `json:"inputs"`
	}
	if r.ContentLength != 0 && !decodeJSON(w, r, &body) {
		return
	}
	started, err := s.engine.Start(r.Context(), wf, body.Inputs)
	if err != nil {
		s.repoError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, started)
}

// handleStartScratchRun runs a workflow sent in the request without saving
// it: a test run of an editor's unsaved draft (given its workflowId, so the
// run belongs to that workflow) or of the scratch workflow (no id).
func (s *Server) handleStartScratchRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkflowID string         `json:"workflowId"`
		Workflow   workflowInput  `json:"workflow"`
		Inputs     map[string]any `json:"inputs"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	wf := body.Workflow.toWorkflow()
	if wf.Name == "" {
		wf.Name = "Scratch"
	}
	if body.WorkflowID != "" {
		if _, err := s.workflows.Get(r.Context(), body.WorkflowID); err != nil {
			s.repoError(w, r, err)
			return
		}
		wf.ID = body.WorkflowID
	}
	if err := s.validate(r.Context(), wf); err != nil {
		s.repoError(w, r, err)
		return
	}
	started, err := s.engine.Start(r.Context(), wf, body.Inputs)
	if err != nil {
		s.repoError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, started)
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	list, err := s.runs.List(r.Context(), run.ListFilter{
		WorkflowID: r.URL.Query().Get("workflowId"),
		Scratch:    r.URL.Query().Get("scratch") == "true",
		Status:     run.Status(r.URL.Query().Get("status")),
		TopLevel:   r.URL.Query().Get("topLevel") == "true",
		Limit:      limit,
		Offset:     offset,
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

// handleDecision approves or rejects an approval step that is waiting.
func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	var d engine.Decision
	if !decodeJSON(w, r, &d) {
		return
	}
	if err := s.engine.Decide(chi.URLParam(r, "id"), chi.URLParam(r, "step"), d); err != nil {
		s.runError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleResponse gives values to an input step that is waiting.
func (s *Server) handleResponse(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Values map[string]any `json:"values"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := s.engine.Respond(chi.URLParam(r, "id"), chi.URLParam(r, "step"), body.Values); err != nil {
		s.runError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) runError(w http.ResponseWriter, r *http.Request, err error) {
	var verr *workflow.ValidationError
	switch {
	case errors.As(err, &verr):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":    verr.Problems[0].Message,
			"problems": verr.Problems,
		})
	case errors.Is(err, run.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, engine.ErrNotActive):
		writeError(w, http.StatusConflict, "This run has already finished.")
	case errors.Is(err, engine.ErrNotWaiting):
		writeError(w, http.StatusConflict, "This step isn't waiting any more; it may already have been answered.")
	default:
		s.internalError(w, r, err)
	}
}
