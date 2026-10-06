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

// Blocks is what validation needs to know about the available blocks.
type Blocks interface {
	Has(typ string) bool
	// Literal lists the config keys of a block whose values are taken as
	// written rather than searched for expressions, such as a script's code.
	Literal(typ string) []string
}

// Validate checks the workflow's structure: names, step IDs, known blocks,
// edge endpoints and the absence of cycles. Block configs are not checked
// here; drafts may be incomplete until they run.
func Validate(w Workflow, blocks Blocks) error {
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

		if !blocks.Has(s.Block) {
			add(Problem{Step: s.ID, Message: fmt.Sprintf("step %q uses unknown block %q", s.ID, s.Block)})
		}
		if s.Timeout != "" {
			if d, err := time.ParseDuration(s.Timeout); err != nil || d <= 0 {
				add(Problem{Step: s.ID, Message: fmt.Sprintf("step %q has an invalid timeout %q; use a duration like 30s, 5m or 1h", s.ID, s.Timeout)})
			}
		}
		problems = append(problems, ValidateSettings(s)...)
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
		problems = append(problems, checkExpressions(def, blocks)...)
		problems = append(problems, checkOutputs(def, steps)...)
	}

	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

// ValidateSettings checks a step's retry and loop settings. Its condition
// and loop list are expressions, checked with the others.
func ValidateSettings(s Step) []Problem {
	var problems []Problem
	add := func(format string, args ...any) {
		problems = append(problems, Problem{Step: s.ID, Message: fmt.Sprintf("step %q: ", s.ID) + fmt.Sprintf(format, args...)})
	}
	if r := s.Retry; r != nil {
		if r.Max < 1 || r.Max > MaxRetries {
			add("retries must be between 1 and %d", MaxRetries)
		}
		if r.Delay != "" {
			if d, err := time.ParseDuration(r.Delay); err != nil || d <= 0 || d > MaxRetryDelay {
				add("retry delay %q must be a duration up to %s, like 10s or 2m", r.Delay, MaxRetryDelay)
			}
		}
		switch r.Backoff {
		case "", BackoffFixed, BackoffExponential:
		default:
			add("retry backoff must be %q or %q", BackoffFixed, BackoffExponential)
		}
	}
	if f := s.ForEach; f != nil {
		if strings.TrimSpace(f.Items) == "" {
			add("choose the list to run once per item over")
		}
		if f.Concurrency < 0 || f.Concurrency > MaxForEachConcurrency {
			add("items at a time must be between 1 and %d", MaxForEachConcurrency)
		}
	}
	return problems
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

// checkExpressions compiles every expression in step configs, conditions
// and loop lists, and checks that a step only reads outputs of steps that
// finish before it starts (its ancestors), only inputs the workflow declares,
// and `item` only when it runs once per item. Secrets are checked when a run
// starts, since they live outside the workflow.
func checkExpressions(def Definition, blocks Blocks) []Problem {
	inputs := make(map[string]bool, len(def.Inputs))
	for _, in := range def.Inputs {
		inputs[in.Name] = true
	}
	ancestors := ancestorSets(def)

	var problems []Problem
	for _, s := range def.Steps {
		fail := func(format string, args ...any) {
			problems = append(problems, Problem{Step: s.ID, Message: fmt.Sprintf("step %q", s.ID) + fmt.Sprintf(format, args...)})
		}
		refs, err := tmpl.ConfigRefs(s.Config, blocks.Literal(s.Block)...)
		if err != nil {
			fail(": %v", err)
			continue
		}
		// Expressions evaluated before the step's items exist can't use them.
		var outer []tmpl.Ref
		if s.If != "" {
			e, err := tmpl.CompileExpr(s.If)
			if err != nil {
				fail(" condition: %v", err)
				continue
			}
			outer = append(outer, e.Refs()...)
		}
		if s.ForEach != nil && strings.TrimSpace(s.ForEach.Items) != "" {
			e, err := tmpl.CompileExpr(s.ForEach.Items)
			if err != nil {
				fail(" list of items: %v", err)
				continue
			}
			outer = append(outer, e.Refs()...)
		}

		reported := map[tmpl.Ref]bool{}
		for i, r := range append(refs, outer...) {
			inConfig := i < len(refs)
			if reported[r] {
				continue
			}
			reported[r] = true
			switch {
			case r.Kind == "steps" && r.Name == s.ID:
				fail(" can't use its own outputs")
			case r.Kind == "steps" && !ancestors[s.ID][r.Name]:
				fail(" uses outputs of %q but doesn't run after it; connect %s → %s", r.Name, r.Name, s.ID)
			case r.Kind == "inputs" && !inputs[r.Name]:
				fail(" uses input %q, which isn't defined", r.Name)
			case r.Kind == "item" && !inConfig:
				fail(" uses %s outside its config; the item only exists while the step runs", r.Name)
			case r.Kind == "item" && s.ForEach == nil:
				fail(" uses %s, which only exists in steps that run once per item", r.Name)
			}
		}
	}
	return problems
}

// checkOutputs checks the workflow's outputs: unique names, and expressions
// that only read steps and inputs that exist. They are evaluated after every
// step has finished, so any step may be used.
func checkOutputs(def Definition, steps map[string]bool) []Problem {
	inputs := make(map[string]bool, len(def.Inputs))
	for _, in := range def.Inputs {
		inputs[in.Name] = true
	}
	var problems []Problem
	seen := map[string]bool{}
	for _, o := range def.Outputs {
		fail := func(format string, args ...any) {
			problems = append(problems, Problem{Message: fmt.Sprintf("output %q ", o.Name) + fmt.Sprintf(format, args...)})
		}
		switch {
		case !inputNamePattern.MatchString(o.Name):
			problems = append(problems, Problem{Message: fmt.Sprintf("output name %q must start with a letter and use only letters, digits and _", o.Name)})
			continue
		case seen[o.Name]:
			fail("is defined more than once")
			continue
		}
		seen[o.Name] = true
		if strings.TrimSpace(o.Value) == "" {
			fail("needs a value, like steps.build.outputs.stdout")
			continue
		}
		e, err := tmpl.CompileExpr(o.Value)
		if err != nil {
			fail("value: %v", err)
			continue
		}
		for _, r := range e.Refs() {
			switch {
			case r.Kind == "steps" && !steps[r.Name]:
				fail("uses step %q, which doesn't exist", r.Name)
			case r.Kind == "inputs" && !inputs[r.Name]:
				fail("uses input %q, which isn't defined", r.Name)
			case r.Kind == "item":
				fail("uses %s, which only exists in steps that run once per item", r.Name)
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
