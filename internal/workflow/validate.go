package workflow

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"conductor/internal/tmpl"
)

// Problem is one validation failure. Step or Edge points at the offending
// element so the editor can highlight it.
type Problem struct {
	Message string `json:"message"`
	Step    string `json:"step,omitempty"`
	Edge    *Edge  `json:"edge,omitempty"`
}

// ValidationError collects every problem found, not just the first.
type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		msgs[i] = p.Message
	}
	return "invalid workflow: " + strings.Join(msgs, "; ")
}

var stepIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

const maxNameLen = 200

// Validate checks the workflow's structure: names, step IDs, known blocks,
// edge endpoints and the absence of cycles. Block configs are not checked
// here; drafts may be incomplete until they run.
func Validate(w Workflow, blockExists func(string) bool) error {
	var problems []Problem
	add := func(p Problem) { problems = append(problems, p) }

	name := strings.TrimSpace(w.Name)
	switch {
	case name == "":
		add(Problem{Message: "name is required"})
	case len(name) > maxNameLen:
		add(Problem{Message: fmt.Sprintf("name is longer than %d characters", maxNameLen)})
	}

	def := w.Definition
	if def.SchemaVersion != SchemaVersion {
		add(Problem{Message: fmt.Sprintf("unsupported schema version %d", def.SchemaVersion)})
	}
	problems = append(problems, validateInputs(def.Inputs)...)

	steps := make(map[string]bool, len(def.Steps))
	for _, s := range def.Steps {
		switch {
		case !stepIDPattern.MatchString(s.ID):
			add(Problem{Step: s.ID, Message: fmt.Sprintf("step id %q must start with a letter and use only a-z, 0-9 and _", s.ID)})
		case steps[s.ID]:
			add(Problem{Step: s.ID, Message: fmt.Sprintf("step id %q is used more than once", s.ID)})
		}
		steps[s.ID] = true

		if !blockExists(s.Block) {
			add(Problem{Step: s.ID, Message: fmt.Sprintf("step %q uses unknown block %q", s.ID, s.Block)})
		}
		if s.Timeout != "" {
			if d, err := time.ParseDuration(s.Timeout); err != nil || d <= 0 {
				add(Problem{Step: s.ID, Message: fmt.Sprintf("step %q has an invalid timeout %q; use a duration like 30s, 5m or 1h", s.ID, s.Timeout)})
			}
		}
	}

	seen := make(map[Edge]bool, len(def.Edges))
	for _, e := range def.Edges {
		edge := e
		switch {
		case !steps[e.From] || !steps[e.To]:
			add(Problem{Edge: &edge, Message: fmt.Sprintf("edge %s → %s references a missing step", e.From, e.To)})
		case e.From == e.To:
			add(Problem{Edge: &edge, Message: fmt.Sprintf("step %q cannot depend on itself", e.From)})
		case seen[e]:
			add(Problem{Edge: &edge, Message: fmt.Sprintf("edge %s → %s is duplicated", e.From, e.To)})
		}
		seen[e] = true
	}

	// Only look for cycles in an otherwise sound graph; dangling edges would
	// produce confusing extra messages.
	if len(problems) == 0 {
		if cycle := findCycle(def); cycle != nil {
			add(Problem{Step: cycle[0], Message: "steps form a cycle: " + strings.Join(cycle, " → ")})
		}
	}
	// Expression references depend on the graph, so check them last.
	if len(problems) == 0 {
		problems = append(problems, checkExpressions(def)...)
	}

	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

// findCycle returns the steps of one cycle (first step repeated at the end),
// or nil if the graph is acyclic.
func findCycle(def Definition) []string {
	next := make(map[string][]string, len(def.Steps))
	for _, e := range def.Edges {
		next[e.From] = append(next[e.From], e.To)
	}

	const (
		unvisited = iota
		inProgress
		done
	)
	state := make(map[string]int, len(def.Steps))
	var stack []string

	var visit func(id string) []string
	visit = func(id string) []string {
		state[id] = inProgress
		stack = append(stack, id)
		for _, n := range next[id] {
			switch state[n] {
			case inProgress:
				for i, s := range stack {
					if s == n {
						return append(append([]string{}, stack[i:]...), n)
					}
				}
			case unvisited:
				if c := visit(n); c != nil {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
		return nil
	}

	for _, s := range def.Steps {
		if state[s.ID] == unvisited {
			if c := visit(s.ID); c != nil {
				return c
			}
		}
	}
	return nil
}

// checkExpressions compiles every expression in step configs and checks that
// a step only reads outputs of steps that finish before it starts (its
// ancestors) and only inputs the workflow declares. Secrets are checked when
// a run starts, since they live outside the workflow.
func checkExpressions(def Definition) []Problem {
	inputs := make(map[string]bool, len(def.Inputs))
	for _, in := range def.Inputs {
		inputs[in.Name] = true
	}
	ancestors := ancestorSets(def)

	var problems []Problem
	for _, s := range def.Steps {
		refs, err := tmpl.ConfigRefs(s.Config)
		if err != nil {
			problems = append(problems, Problem{Step: s.ID, Message: fmt.Sprintf("step %q: %v", s.ID, err)})
			continue
		}
		reported := map[tmpl.Ref]bool{}
		for _, r := range refs {
			if reported[r] {
				continue
			}
			reported[r] = true
			switch {
			case r.Kind == "steps" && r.Name == s.ID:
				problems = append(problems, Problem{Step: s.ID, Message: fmt.Sprintf("step %q can't use its own outputs", s.ID)})
			case r.Kind == "steps" && !ancestors[s.ID][r.Name]:
				problems = append(problems, Problem{Step: s.ID, Message: fmt.Sprintf(
					"step %q uses outputs of %q but doesn't run after it; connect %s → %s", s.ID, r.Name, r.Name, s.ID)})
			case r.Kind == "inputs" && !inputs[r.Name]:
				problems = append(problems, Problem{Step: s.ID, Message: fmt.Sprintf("step %q uses input %q, which isn't defined", s.ID, r.Name)})
			}
		}
	}
	return problems
}

// ancestorSets maps each step to every step it transitively depends on.
func ancestorSets(def Definition) map[string]map[string]bool {
	parents := map[string][]string{}
	for _, e := range def.Edges {
		parents[e.To] = append(parents[e.To], e.From)
	}
	memo := map[string]map[string]bool{}
	var of func(id string) map[string]bool
	of = func(id string) map[string]bool {
		if a, ok := memo[id]; ok {
			return a
		}
		a := map[string]bool{}
		memo[id] = a
		for _, p := range parents[id] {
			a[p] = true
			for q := range of(p) {
				a[q] = true
			}
		}
		return a
	}
	for _, s := range def.Steps {
		of(s.ID)
	}
	return memo
}
