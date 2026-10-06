package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// CallBlock is the block of a step that runs another workflow. The engine
// runs it itself: the called workflow becomes a child run (ADR 0007).
const CallBlock = "workflow"

// MaxCallDepth bounds how deeply workflows may run one another.
const MaxCallDepth = 8

// Call is the config of a CallBlock step.
type Call struct {
	Workflow string `json:"workflow"`
	// Inputs are values for the called workflow's inputs, by name; strings
	// may contain expressions.
	Inputs map[string]any `json:"inputs"`
}

// ParseCall reads the config of a step that runs a workflow.
func ParseCall(config json.RawMessage) (Call, error) {
	var c Call
	if len(config) > 0 {
		if err := json.Unmarshal(config, &c); err != nil {
			return Call{}, fmt.Errorf("config: %w", err)
		}
	}
	return c, nil
}

// Getter loads a workflow by id; it returns ErrNotFound for unknown ids.
type Getter func(ctx context.Context, id string) (Workflow, error)

// ValidateCalls checks w's steps that run other workflows: the workflow
// exists, it declares the inputs given to it, and running it can't lead
// back to w. Other workflows are loaded with get; a failure to load them is
// returned as the error, problems as a *ValidationError.
func ValidateCalls(ctx context.Context, w Workflow, get Getter) error {
	var problems []Problem
	// w as it is about to be saved, not as stored.
	memo := map[string]Workflow{w.ID: w}
	load := func(id string) (Workflow, bool, error) {
		if wf, ok := memo[id]; ok {
			return wf, true, nil
		}
		wf, err := get(ctx, id)
		if errors.Is(err, ErrNotFound) {
			return Workflow{}, false, nil
		}
		if err != nil {
			return Workflow{}, false, err
		}
		memo[id] = wf
		return wf, true, nil
	}

	for _, s := range w.Definition.Steps {
		if s.Block != CallBlock {
			continue
		}
		fail := func(format string, args ...any) {
			problems = append(problems, Problem{Step: s.ID, Message: fmt.Sprintf("step %q ", s.ID) + fmt.Sprintf(format, args...)})
		}
		call, err := ParseCall(s.Config)
		if err != nil {
			fail("has an invalid %v", err)
			continue
		}
		if call.Workflow == "" {
			continue // a draft; reported when the workflow runs
		}
		if w.ID != "" && call.Workflow == w.ID {
			fail("can't run the workflow it belongs to")
			continue
		}
		target, ok, err := load(call.Workflow)
		if err != nil {
			return err
		}
		if !ok {
			fail("runs a workflow that doesn't exist; choose another")
			continue
		}
		declared := map[string]bool{}
		for _, in := range target.Definition.Inputs {
			declared[in.Name] = true
		}
		for _, name := range sortedKeys(call.Inputs) {
			if !declared[name] {
				fail("gives input %q, which %q doesn't have", name, target.Name)
			}
		}
		if w.ID == "" {
			continue // a new workflow can't be called by anything yet
		}
		path, err := callPath(target, w.ID, load)
		if err != nil {
			return err
		}
		if path != nil {
			names := []string{w.Name}
			for _, wf := range path {
				names = append(names, wf.Name)
			}
			fail("would make the workflows run each other forever: %s", strings.Join(names, " → "))
		}
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

// callPath finds a chain of calls from start that leads to the workflow
// with id target, returned as the workflows along it (start first, target
// last), or nil if there is none.
func callPath(start Workflow, target string, load func(string) (Workflow, bool, error)) ([]Workflow, error) {
	visited := map[string]bool{}
	var walk func(wf Workflow, path []Workflow) ([]Workflow, error)
	walk = func(wf Workflow, path []Workflow) ([]Workflow, error) {
		path = append(path, wf)
		if wf.ID == target {
			return path, nil
		}
		if visited[wf.ID] {
			return nil, nil
		}
		visited[wf.ID] = true
		for _, id := range CalledWorkflows(wf.Definition) {
			next, ok, err := load(id)
			if err != nil || !ok {
				if err != nil {
					return nil, err
				}
				continue
			}
			if found, err := walk(next, slices.Clone(path)); found != nil || err != nil {
				return found, err
			}
		}
		return nil, nil
	}
	return walk(start, nil)
}

// CalledWorkflows lists the ids of the workflows def's steps run.
func CalledWorkflows(def Definition) []string {
	var ids []string
	for _, s := range def.Steps {
		if s.Block != CallBlock {
			continue
		}
		if c, err := ParseCall(s.Config); err == nil && c.Workflow != "" && !slices.Contains(ids, c.Workflow) {
			ids = append(ids, c.Workflow)
		}
	}
	return ids
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
