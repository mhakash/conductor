package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"conductor/internal/preset"
	"conductor/internal/workflow"
)

type presetInput struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Block       string      `json:"block"`
	Step        preset.Step `json:"step"`
}

func (in presetInput) toPreset() preset.Preset {
	return preset.Preset{
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Block:       in.Block,
		Step:        in.Step,
	}
}

func (s *Server) handleListPresets(w http.ResponseWriter, r *http.Request) {
	list, err := s.presets.List(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreatePreset(w http.ResponseWriter, r *http.Request) {
	var in presetInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p := in.toPreset()
	if err := preset.Validate(p, s.blocks); err != nil {
		s.presetError(w, r, err)
		return
	}
	created, err := s.presets.Create(r.Context(), p)
	if err != nil {
		s.presetError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdatePreset(w http.ResponseWriter, r *http.Request) {
	var in presetInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p := in.toPreset()
	p.ID = chi.URLParam(r, "id")
	if err := preset.Validate(p, s.blocks); err != nil {
		s.presetError(w, r, err)
		return
	}
	updated, err := s.presets.Update(r.Context(), p)
	if err != nil {
		s.presetError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeletePreset(w http.ResponseWriter, r *http.Request) {
	if err := s.presets.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		s.presetError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) presetError(w http.ResponseWriter, r *http.Request, err error) {
	var verr *workflow.ValidationError
	switch {
	case errors.Is(err, preset.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, preset.ErrNameTaken):
		writeError(w, http.StatusConflict, "A preset with this name already exists.")
	case errors.As(err, &verr):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":    "The preset has problems that need fixing before it can be saved.",
			"problems": verr.Problems,
		})
	default:
		s.internalError(w, r, err)
	}
}
