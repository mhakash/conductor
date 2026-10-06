package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"conductor/internal/wfyaml"
	"conductor/internal/workflow"
)

// YAML is how workflows are viewed, exported and imported (ADR 0009).

const yamlType = "application/yaml; charset=utf-8"

// handleToYAML renders a workflow sent as JSON (such as the editor's
// unsaved draft) as YAML.
func (s *Server) handleToYAML(w http.ResponseWriter, r *http.Request) {
	var in workflowInput
	if !decodeJSON(w, r, &in) {
		return
	}
	s.writeYAML(w, r, in.toWorkflow(), "")
}

// handleFromYAML parses YAML into a workflow without saving it, for the
// editor to load as a draft.
func (s *Server) handleFromYAML(w http.ResponseWriter, r *http.Request) {
	wf, ok := s.readYAML(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, workflowInput{Name: wf.Name, Description: wf.Description, Definition: &wf.Definition})
}

// handleExportWorkflow downloads a saved workflow as a YAML file.
func (s *Server) handleExportWorkflow(w http.ResponseWriter, r *http.Request) {
	wf, err := s.workflows.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		s.repoError(w, r, err)
		return
	}
	s.writeYAML(w, r, wf, fileName(wf.Name)+".yaml")
}

// handleImportWorkflow creates a workflow from YAML.
func (s *Server) handleImportWorkflow(w http.ResponseWriter, r *http.Request) {
	wf, ok := s.readYAML(w, r)
	if !ok {
		return
	}
	if err := s.validate(r.Context(), wf); err != nil {
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

func (s *Server) writeYAML(w http.ResponseWriter, r *http.Request, wf workflow.Workflow, download string) {
	list, err := s.workflows.List(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	callsByName(&wf.Definition, list)
	data, err := wfyaml.Encode(wf, s.fieldOrder)
	if err != nil {
		s.repoError(w, r, &workflow.ValidationError{Problems: []workflow.Problem{{Message: err.Error()}}})
		return
	}
	w.Header().Set("Content-Type", yamlType)
	if download != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", download))
	}
	_, _ = w.Write(data)
}

// readYAML parses the request body as a workflow and points steps that run
// other workflows at their ids. Problems are written as a 422.
func (s *Server) readYAML(w http.ResponseWriter, r *http.Request) (workflow.Workflow, bool) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "couldn't read the request: "+err.Error())
		return workflow.Workflow{}, false
	}
	wf, err := wfyaml.Decode(data)
	if err != nil {
		s.repoError(w, r, &workflow.ValidationError{Problems: []workflow.Problem{{Message: err.Error()}}})
		return workflow.Workflow{}, false
	}
	list, err := s.workflows.List(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return workflow.Workflow{}, false
	}
	if problems := callsByID(&wf.Definition, list); len(problems) > 0 {
		s.repoError(w, r, &workflow.ValidationError{Problems: problems})
		return workflow.Workflow{}, false
	}
	return wf, true
}

func (s *Server) fieldOrder(block string) []string {
	b, _ := s.blocks.Get(block)
	keys := make([]string, len(b.Fields))
	for i, f := range b.Fields {
		keys[i] = f.Key
	}
	return keys
}

// callsByName writes the workflows that steps run by name where the name is
// unique, so the YAML reads well and works on another machine.
func callsByName(def *workflow.Definition, list []workflow.Summary) {
	count := map[string]int{}
	for _, wf := range list {
		count[wf.Name]++
	}
	names := map[string]string{}
	for _, wf := range list {
		if count[wf.Name] == 1 {
			names[wf.ID] = wf.Name
		}
	}
	rewriteCalls(def, func(ref string) string {
		if name, ok := names[ref]; ok {
			return name
		}
		return ref
	})
}

// callsByID turns workflow names in steps that run workflows back into ids.
// An id is kept as is; a name must match exactly one workflow.
func callsByID(def *workflow.Definition, list []workflow.Summary) []workflow.Problem {
	ids := map[string]bool{}
	byName := map[string][]string{}
	for _, wf := range list {
		ids[wf.ID] = true
		byName[wf.Name] = append(byName[wf.Name], wf.ID)
	}
	var problems []workflow.Problem
	rewriteCallsOf(def, func(stepID, ref string) string {
		if ids[ref] {
			return ref
		}
		switch matches := byName[ref]; len(matches) {
		case 1:
			return matches[0]
		case 0:
			problems = append(problems, workflow.Problem{Step: stepID, Message: fmt.Sprintf("step %q runs %q, but there is no workflow with that name", stepID, ref)})
		default:
			problems = append(problems, workflow.Problem{Step: stepID, Message: fmt.Sprintf("step %q runs %q, but %d workflows have that name; use its id instead", stepID, ref, len(matches))})
		}
		return ref
	})
	return problems
}

func rewriteCalls(def *workflow.Definition, fn func(ref string) string) {
	rewriteCallsOf(def, func(_, ref string) string { return fn(ref) })
}

// rewriteCallsOf replaces the workflow reference of every step that runs a
// workflow with fn's result.
func rewriteCallsOf(def *workflow.Definition, fn func(stepID, ref string) string) {
	for i, st := range def.Steps {
		if st.Block != workflow.CallBlock {
			continue
		}
		var config map[string]any
		if json.Unmarshal(st.Config, &config) != nil {
			continue
		}
		ref, _ := config["workflow"].(string)
		if ref == "" {
			continue
		}
		config["workflow"] = fn(st.ID, ref)
		if data, err := json.Marshal(config); err == nil {
			def.Steps[i].Config = data
		}
	}
}

var unsafeFileChars = regexp.MustCompile(`[^a-z0-9]+`)

// fileName makes a workflow name into a file name: "Build prep" → "build-prep".
func fileName(name string) string {
	if s := strings.Trim(unsafeFileChars.ReplaceAllString(strings.ToLower(name), "-"), "-"); s != "" {
		return s
	}
	return "workflow"
}
