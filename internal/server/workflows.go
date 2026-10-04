package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"conductor/internal/workflow"
)

// maxBodyBytes bounds request bodies; definitions are small JSON documents.
const maxBodyBytes = 4 << 20

type workflowInput struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Definition  *workflow.Definition `json:"definition"`
}

func (in workflowInput) toWorkflow() workflow.Workflow {
	def := workflow.EmptyDefinition()
	if in.Definition != nil {
		def = *in.Definition
	}
	if def.Inputs == nil {
		def.Inputs = []workflow.Input{}
	}
	if def.Steps == nil {
		def.Steps = []workflow.Step{}
	}
	if def.Edges == nil {
		def.Edges = []workflow.Edge{}
	}
	for i := range def.Steps {
		if len(def.Steps[i].Config) == 0 || string(def.Steps[i].Config) == "null" {
			def.Steps[i].Config = json.RawMessage("{}")
		}
	}
	return workflow.Workflow{
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Definition:  def,
	}
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	list, err := s.workflows.List(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	wf, err := s.workflows.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.repoError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var in workflowInput
	if !decodeJSON(w, r, &in) {
		return
	}
	wf := in.toWorkflow()
	if err := workflow.Validate(wf, s.blocks.Has); err != nil {
		s.repoError(w, r, err)
		return
	}
	created, err := s.workflows.Create(r.Context(), wf)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	var in workflowInput
	if !decodeJSON(w, r, &in) {
		return
	}
	wf := in.toWorkflow()
	wf.ID = chi.URLParam(r, "id")
	if err := workflow.Validate(wf, s.blocks.Has); err != nil {
		s.repoError(w, r, err)
		return
	}
	updated, err := s.workflows.Update(r.Context(), wf)
	if err != nil {
		s.repoError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	if err := s.workflows.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		s.repoError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListBlocks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.blocks.All())
}

// repoError maps domain errors to HTTP responses.
func (s *Server) repoError(w http.ResponseWriter, r *http.Request, err error) {
	var verr *workflow.ValidationError
	switch {
	case errors.Is(err, workflow.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.As(err, &verr):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":    "The workflow has problems that need fixing before it can be saved.",
			"problems": verr.Problems,
		})
	default:
		s.internalError(w, r, err)
	}
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.ErrorContext(r.Context(), "request failed", "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}
