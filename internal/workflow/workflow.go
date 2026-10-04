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
	// If is an expression (without {{ }}); the step is skipped unless it is true.
	If    string `json:"if,omitempty"`
	Retry *Retry `json:"retry,omitempty"`
	// ContinueOnError lets downstream steps run, and the run succeed, even
	// if this step fails.
	ContinueOnError bool     `json:"continueOnError,omitempty"`
	ForEach         *ForEach `json:"forEach,omitempty"`
}

// Retry runs a failed step again, up to Max more times.
type Retry struct {
	Max int `json:"max"`
	// Delay is the wait before the first retry (Go duration); default 10s.
	Delay string `json:"delay,omitempty"`
	// Backoff is "fixed" (default) or "exponential", which doubles the delay
	// after every attempt.
	Backoff string `json:"backoff,omitempty"`
}

const (
	BackoffFixed       = "fixed"
	BackoffExponential = "exponential"
	DefaultRetryDelay  = 10 * time.Second
	MaxRetries         = 20
	MaxRetryDelay      = time.Hour
)

// ForEach runs the step once per element of a list. The step's config can
// read the element as `item` and its position as `index`.
type ForEach struct {
	// Items is an expression (without {{ }}) that evaluates to a list.
	Items string `json:"items"`
	// Concurrency is how many items run at the same time; default 1.
	Concurrency int `json:"concurrency,omitempty"`
}

const (
	MaxForEachConcurrency = 32
	MaxForEachItems       = 1000
)

// DelayBefore returns the wait before retry number n (1-based).
func (r Retry) DelayBefore(n int) time.Duration {
	d := DefaultRetryDelay
	if r.Delay != "" {
		d, _ = time.ParseDuration(r.Delay) // validated on save
	}
	if r.Backoff == BackoffExponential {
		for i := 1; i < n && d < MaxRetryDelay; i++ {
			d *= 2
		}
	}
	return min(d, MaxRetryDelay)
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
