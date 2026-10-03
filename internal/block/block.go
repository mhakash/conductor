// Package block describes the building blocks a workflow step can use: what
// they are called, what they need to be configured, and what they produce.
package block

import "slices"

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

type Block struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	// Icon is a lucide icon name, rendered by the frontend.
	Icon    string   `json:"icon"`
	Fields  []Field  `json:"fields"`
	Outputs []Output `json:"outputs"`
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
