// Package workflow defines workflows and the rules that keep their graphs valid.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// SchemaVersion is the current version of the Definition format. Bump it and
// add a migration path when the shape changes incompatibly.
const SchemaVersion = 1

type Workflow struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Definition  Definition `json:"definition"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Definition is the graph of a workflow. It is stored as one JSON document so
// a save is atomic and the format can be exported as-is.
type Definition struct {
	SchemaVersion int     `json:"schemaVersion"`
	Inputs        []Input `json:"inputs"`
	Steps         []Step  `json:"steps"`
	Edges         []Edge  `json:"edges"`
}

// Step is a configured block. ID is unique within the workflow and is how
// expressions refer to the step's outputs, so it must be a valid identifier.
type Step struct {
	ID       string          `json:"id"`
	Block    string          `json:"block"`
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
	Position Position        `json:"position"`
	// Timeout is a Go duration ("30s", "10m"). Empty means the engine default.
	Timeout string `json:"timeout,omitempty"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Edge means To depends on From: To starts after From has finished.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Summary is a lightweight view for listings.
type Summary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	StepCount   int       `json:"stepCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

var (
	ErrNotFound = errors.New("workflow not found")
	// ErrConflict means the workflow changed since the caller last read it.
	ErrConflict = errors.New("workflow was changed elsewhere")
)

type Repository interface {
	List(ctx context.Context) ([]Summary, error)
	Get(ctx context.Context, id string) (Workflow, error)
	Create(ctx context.Context, w Workflow) (Workflow, error)
	// Update replaces a workflow. With ifUnchangedSince set, it fails with
	// ErrConflict unless the stored workflow still has that UpdatedAt.
	Update(ctx context.Context, w Workflow, ifUnchangedSince *time.Time) (Workflow, error)
	Delete(ctx context.Context, id string) error
}

// EmptyDefinition returns a definition with no steps, ready to be saved.
func EmptyDefinition() Definition {
	return Definition{SchemaVersion: SchemaVersion, Inputs: []Input{}, Steps: []Step{}, Edges: []Edge{}}
}
