// Package preset defines saved step configurations: a block with its config
// and settings filled in, offered in the editor's library under its own name.
// Adding a preset copies it into the workflow; later edits to the preset
// don't change steps made from it.
package preset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"conductor/internal/tmpl"
	"conductor/internal/workflow"
)

type Preset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Block       string    `json:"block"`
	Step        Step      `json:"step"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Step is what a preset fills in on a new step. Conditions and loops are
// left out: they refer to other steps of a particular workflow.
type Step struct {
	Config          json.RawMessage `json:"config"`
	Timeout         string          `json:"timeout,omitempty"`
	Retry           *workflow.Retry `json:"retry,omitempty"`
	ContinueOnError bool            `json:"continueOnError,omitempty"`
}

var (
	ErrNotFound = errors.New("preset not found")
	// ErrNameTaken means another preset already has the name (ignoring case).
	ErrNameTaken = errors.New("a preset with this name already exists")
)

type Repository interface {
	List(ctx context.Context) ([]Preset, error)
	Get(ctx context.Context, id string) (Preset, error)
	Create(ctx context.Context, p Preset) (Preset, error)
	Update(ctx context.Context, p Preset) (Preset, error)
	Delete(ctx context.Context, id string) error
}

// Validate checks a preset as it would be checked as a step: a known block,
// valid settings and expressions that compile.
func Validate(p Preset, blockExists func(string) bool) error {
	var problems []workflow.Problem
	add := func(format string, args ...any) {
		problems = append(problems, workflow.Problem{Message: fmt.Sprintf(format, args...)})
	}
	switch name := strings.TrimSpace(p.Name); {
	case name == "":
		add("name is required")
	case len(name) > 200:
		add("name is longer than 200 characters")
	}
	if !blockExists(p.Block) {
		add("unknown block %q", p.Block)
	}
	if p.Step.Timeout != "" {
		if d, err := time.ParseDuration(p.Step.Timeout); err != nil || d <= 0 {
			add("invalid timeout %q; use a duration like 30s, 5m or 1h", p.Step.Timeout)
		}
	}
	// Reuse the step rules, without the step name they start with.
	for _, pr := range workflow.ValidateSettings(workflow.Step{ID: "preset", Retry: p.Step.Retry}) {
		problems = append(problems, workflow.Problem{Message: strings.TrimPrefix(pr.Message, `step "preset": `)})
	}
	refs, err := tmpl.ConfigRefs(p.Step.Config)
	if err != nil {
		add("config: %v", err)
	}
	for _, r := range refs {
		if r.Kind == "item" {
			add("config uses %s, which only exists in steps that run once per item", r.Name)
			break
		}
	}
	if len(problems) > 0 {
		return &workflow.ValidationError{Problems: problems}
	}
	return nil
}
