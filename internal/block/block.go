// Package block describes the building blocks a workflow step can use: what
// they are called, what they need to be configured, and what they produce.
package block

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// FieldType tells the editor which input to render.
type FieldType string

const (
	FieldText     FieldType = "text"     // single line
	FieldTextarea FieldType = "textarea" // multi-line prose
	FieldCode     FieldType = "code"     // multi-line code; see Field.Language
	FieldNumber   FieldType = "number"
	FieldBoolean  FieldType = "boolean"
	FieldSelect   FieldType = "select" // one of Field.Options
	FieldKeyValue FieldType = "keyvalue"
)

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Field struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Help        string    `json:"help,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	Default     any       `json:"default,omitempty"`
	Options     []Option  `json:"options,omitempty"`
	Language    string    `json:"language,omitempty"`
}

type Output struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// Env is what a running step can use besides its config.
type Env struct {
	// Stdout and Stderr receive the step's output, line by line, into the run log.
	Stdout io.Writer
	Stderr io.Writer
	// Logf writes Conductor's own commentary ("GET https://… → 200").
	Logf func(format string, args ...any)
}

// RunFunc executes a step. It returns the step's outputs, which may be
// partially filled even when err is non-nil (e.g. a command's exit code).
// Implementations must stop promptly when ctx is cancelled.
type RunFunc func(ctx context.Context, config json.RawMessage, env Env) (map[string]any, error)

type Block struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	// Icon is a lucide icon name, rendered by the frontend.
	Icon    string   `json:"icon"`
	Fields  []Field  `json:"fields"`
	Outputs []Output `json:"outputs"`

	Run RunFunc `json:"-"`
}

// Missing returns the labels of required fields that have no value in config.
func (b Block) Missing(config json.RawMessage) ([]string, error) {
	var values map[string]any
	if len(config) > 0 {
		if err := json.Unmarshal(config, &values); err != nil {
			return nil, fmt.Errorf("config is not a JSON object: %w", err)
		}
	}
	var missing []string
	for _, f := range b.Fields {
		if f.Required && isEmpty(values[f.Key]) {
			missing = append(missing, f.Label)
		}
	}
	return missing, nil
}

func isEmpty(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case map[string]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	}
	return false
}

// Registry holds the available blocks in display order.
type Registry struct {
	blocks []Block
}

func NewRegistry(blocks ...Block) *Registry {
	return &Registry{blocks: blocks}
}

func (r *Registry) All() []Block { return slices.Clone(r.blocks) }

func (r *Registry) Get(typ string) (Block, bool) {
	i := slices.IndexFunc(r.blocks, func(b Block) bool { return b.Type == typ })
	if i < 0 {
		return Block{}, false
	}
	return r.blocks[i], true
}

func (r *Registry) Has(typ string) bool {
	_, ok := r.Get(typ)
	return ok
}
