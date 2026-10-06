// Package wfyaml reads and writes workflows as YAML: the format for viewing,
// exporting and importing them (ADR 0009). It is the workflow definition in
// a form meant for people: dependencies are listed on each step as needs,
// and canvas positions are left out.
package wfyaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"conductor/internal/workflow"
)

// Version is the format version written to, and required in, documents.
const Version = 1

type document struct {
	Conductor   int      `yaml:"conductor"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Inputs      []input  `yaml:"inputs,omitempty"`
	Outputs     []output `yaml:"outputs,omitempty"`
	Steps       []step   `yaml:"steps"`
}

type input struct {
	Name        string             `yaml:"name"`
	Label       string             `yaml:"label,omitempty"`
	Type        workflow.InputType `yaml:"type"`
	Default     any                `yaml:"default,omitempty"`
	Required    bool               `yaml:"required,omitempty"`
	Description string             `yaml:"description,omitempty"`
}

type output struct {
	Name        string `yaml:"name"`
	Value       string `yaml:"value"`
	Description string `yaml:"description,omitempty"`
}

type step struct {
	ID              string    `yaml:"id"`
	Name            string    `yaml:"name,omitempty"`
	Block           string    `yaml:"block"`
	Needs           []string  `yaml:"needs,omitempty,flow"`
	If              string    `yaml:"if,omitempty"`
	ForEach         *forEach  `yaml:"forEach,omitempty"`
	Retry           *retry    `yaml:"retry,omitempty"`
	ContinueOnError bool      `yaml:"continueOnError,omitempty"`
	Timeout         string    `yaml:"timeout,omitempty"`
	Config          yaml.Node `yaml:"config,omitempty"`
}

type forEach struct {
	Items       string `yaml:"items"`
	Concurrency int    `yaml:"concurrency,omitempty"`
}

type retry struct {
	Max     int    `yaml:"max"`
	Delay   string `yaml:"delay,omitempty"`
	Backoff string `yaml:"backoff,omitempty"`
}

// Encode writes w as YAML. fieldOrder gives a block's config keys in the
// order its form shows them; other keys follow alphabetically. Empty config
// values are left out.
func Encode(w workflow.Workflow, fieldOrder func(block string) []string) ([]byte, error) {
	def := w.Definition
	doc := document{Conductor: Version, Name: w.Name, Description: w.Description}
	for _, in := range def.Inputs {
		doc.Inputs = append(doc.Inputs, input(in))
	}
	for _, o := range def.Outputs {
		doc.Outputs = append(doc.Outputs, output(o))
	}
	needs := map[string][]string{}
	for _, e := range def.Edges {
		needs[e.To] = append(needs[e.To], e.From)
	}
	for _, s := range def.Steps {
		st := step{
			ID: s.ID, Name: s.Name, Block: s.Block, Needs: needs[s.ID],
			If: s.If, ContinueOnError: s.ContinueOnError, Timeout: s.Timeout,
		}
		if s.ForEach != nil {
			st.ForEach = &forEach{Items: s.ForEach.Items, Concurrency: s.ForEach.Concurrency}
		}
		if s.Retry != nil {
			st.Retry = &retry{Max: s.Retry.Max, Delay: s.Retry.Delay, Backoff: s.Retry.Backoff}
		}
		config, err := configNode(s.Config, fieldOrder(s.Block))
		if err != nil {
			return nil, fmt.Errorf("step %q: %w", s.ID, err)
		}
		st.Config = config
		doc.Steps = append(doc.Steps, st)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), enc.Close()
}

// configNode turns a step's JSON config into a YAML mapping with its keys in
// form order, leaving out empty values. An empty config is a zero Node,
// which omitempty drops.
func configNode(raw json.RawMessage, order []string) (yaml.Node, error) {
	var values map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &values); err != nil {
			return yaml.Node{}, fmt.Errorf("config is not a JSON object: %w", err)
		}
	}
	keys := make([]string, 0, len(values))
	for k, v := range values {
		if !isEmpty(v) {
			keys = append(keys, k)
		}
	}
	rank := func(k string) int {
		if i := slices.Index(order, k); i >= 0 {
			return i
		}
		return len(order)
	}
	slices.SortFunc(keys, func(a, b string) int {
		if d := rank(a) - rank(b); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	if len(keys) == 0 {
		return yaml.Node{}, nil
	}

	node := yaml.Node{Kind: yaml.MappingNode}
	for _, k := range keys {
		var v yaml.Node
		if err := v.Encode(values[k]); err != nil {
			return yaml.Node{}, err
		}
		if s, ok := values[k].(string); ok && strings.Contains(s, "\n") {
			v.Style = yaml.LiteralStyle // keep scripts readable
		}
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, &v)
	}
	return node, nil
}

func isEmpty(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case map[string]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	}
	return false
}

// Decode reads a workflow from YAML. Unknown keys are errors, so typos
// don't vanish silently. Steps are laid out in columns by dependency depth.
// The result still needs workflow.Validate.
func Decode(data []byte) (workflow.Workflow, error) {
	var doc document
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return workflow.Workflow{}, readable(err)
	}
	if doc.Conductor != Version {
		return workflow.Workflow{}, fmt.Errorf("this isn't a Conductor workflow: it needs \"conductor: %d\" at the top", Version)
	}

	def := workflow.EmptyDefinition()
	for _, in := range doc.Inputs {
		in.Default = jsonValue(in.Default)
		def.Inputs = append(def.Inputs, workflow.Input(in))
	}
	for _, o := range doc.Outputs {
		def.Outputs = append(def.Outputs, workflow.Output(o))
	}
	for _, st := range doc.Steps {
		config := map[string]any{}
		if !st.Config.IsZero() {
			if err := st.Config.Decode(&config); err != nil {
				return workflow.Workflow{}, fmt.Errorf("step %q: config must be a mapping of field names to values (line %d)", st.ID, st.Config.Line)
			}
		}
		raw, err := json.Marshal(config)
		if err != nil {
			return workflow.Workflow{}, fmt.Errorf("step %q: config: %w", st.ID, err)
		}
		s := workflow.Step{
			ID: st.ID, Name: st.Name, Block: st.Block, Config: raw,
			If: st.If, ContinueOnError: st.ContinueOnError, Timeout: st.Timeout,
		}
		if st.ForEach != nil {
			s.ForEach = &workflow.ForEach{Items: st.ForEach.Items, Concurrency: st.ForEach.Concurrency}
		}
		if st.Retry != nil {
			s.Retry = &workflow.Retry{Max: st.Retry.Max, Delay: st.Retry.Delay, Backoff: st.Retry.Backoff}
		}
		def.Steps = append(def.Steps, s)
		for _, from := range st.Needs {
			def.Edges = append(def.Edges, workflow.Edge{From: from, To: st.ID})
		}
	}
	Layout(&def)
	return workflow.Workflow{Name: strings.TrimSpace(doc.Name), Description: strings.TrimSpace(doc.Description), Definition: def}, nil
}

// jsonValue converts a decoded YAML value to what JSON decoding would give
// (float64 numbers), which is what the rest of Conductor expects.
func jsonValue(v any) any {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	_ = json.Unmarshal(data, &out)
	return out
}

var unknownField = regexp.MustCompile(`field (\S+) not found in type \S+`)

// readable rewords the library's errors for people: no "yaml: " prefix or
// Go type names.
func readable(err error) error {
	msgs := []string{strings.TrimPrefix(err.Error(), "yaml: ")}
	var te *yaml.TypeError
	if errors.As(err, &te) {
		msgs = te.Errors
	}
	for i, m := range msgs {
		msgs[i] = unknownField.ReplaceAllString(m, `unknown key "$1"`)
	}
	return errors.New(strings.Join(msgs, "; "))
}

// Column and row spacing of Layout, matching the editor's step cards.
const (
	columnWidth = 320
	rowHeight   = 140
)

// Layout places steps in columns by the length of the longest chain of
// dependencies leading to them, in document order within a column. Cycles
// (reported by validation) are tolerated.
func Layout(def *workflow.Definition) {
	parents := map[string][]string{}
	for _, e := range def.Edges {
		parents[e.To] = append(parents[e.To], e.From)
	}
	depth := map[string]int{}
	visiting := map[string]bool{}
	var depthOf func(id string) int
	depthOf = func(id string) int {
		if d, ok := depth[id]; ok {
			return d
		}
		if visiting[id] {
			return 0
		}
		visiting[id] = true
		d := 0
		for _, p := range parents[id] {
			d = max(d, depthOf(p)+1)
		}
		depth[id] = d
		return d
	}
	rows := map[int]int{}
	for i, s := range def.Steps {
		col := depthOf(s.ID)
		def.Steps[i].Position = workflow.Position{X: float64(col * columnWidth), Y: float64(rows[col] * rowHeight)}
		rows[col]++
	}
}
