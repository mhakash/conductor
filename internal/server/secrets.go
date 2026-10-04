package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"conductor/internal/secrets"
)

// Secret values are write-only over the API: they can be set and replaced,
// never read back.

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	list, err := s.secrets.List(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handlePutSecret(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Value string `json:"value"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	name := chi.URLParam(r, "name")
	if !secrets.ValidName(name) {
		writeError(w, http.StatusUnprocessableEntity, "Use UPPER_SNAKE_CASE for secret names, e.g. GITLAB_TOKEN.")
		return
	}
	if body.Value == "" {
		writeError(w, http.StatusUnprocessableEntity, "The secret value is empty.")
		return
	}
	saved, err := s.secrets.Set(r.Context(), name, body.Value)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	err := s.secrets.Delete(r.Context(), chi.URLParam(r, "name"))
	if errors.Is(err, secrets.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
