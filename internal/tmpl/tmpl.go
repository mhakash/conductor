// Package tmpl implements Conductor's expressions: `{{ … }}` segments inside
// step config strings, evaluated with expr-lang (https://expr-lang.org).
//
//	"git tag -a {{ inputs.version }} -m {{ 'Release ' + inputs.version }}"
//	"{{ steps.pipeline.outputs.json.status }}"
//
// Expressions see `steps.<id>.outputs` and `steps.<id>.status`, `inputs`,
// `secrets`, `run` and `workflow`; steps that run once per item also see
// `item` and `index`. A rendered value that is a string is inserted as is; anything
// else (numbers, objects) is inserted as JSON.
package tmpl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"
)

const (
	open  = "{{"
	close = "}}"
)

// Ref is a reference to something an expression reads: Kind is "steps",
// "inputs" or "secrets" with Name the step id, input or secret name, or
// "item" (Name "item" or "index") for the current element of a loop.
type Ref struct {
	Kind string
	Name string
}

type part struct {
	text    string // literal text, when program is nil
	source  string
	program *vm.Program
	refs    []Ref
}

// Template is a compiled string with zero or more expressions.
type Template struct {
	parts []part
}

// Has reports whether s contains an expression at all.
func Has(s string) bool { return strings.Contains(s, open) }

// Compile parses s. Errors name the offending expression.
func Compile(s string) (*Template, error) {
	var t Template
	rest := s
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			if rest != "" {
				t.parts = append(t.parts, part{text: rest})
			}
			return &t, nil
		}
		if i > 0 {
			t.parts = append(t.parts, part{text: rest[:i]})
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, close)
		if j < 0 {
			return nil, fmt.Errorf("unclosed %q in %q", open, s)
		}
		src := strings.TrimSpace(rest[:j])
		rest = rest[j+len(close):]
		if src == "" {
			return nil, errors.New("empty expression {{ }}")
		}

		e, err := CompileExpr(src)
		if err != nil {
			return nil, fmt.Errorf("in {{ %s }}: %w", src, err)
		}
		t.parts = append(t.parts, part{source: src, program: e.program, refs: e.refs})
	}
}

// Expr is a single expression written without {{ }}, such as a step's
// condition or the list a step loops over.
type Expr struct {
	source  string
	program *vm.Program
	refs    []Ref
}

func CompileExpr(src string) (*Expr, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, errors.New("expression is empty")
	}
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, errors.New(firstLine(err.Error()))
	}
	program, err := expr.Compile(src)
	if err != nil {
		return nil, errors.New(firstLine(err.Error()))
	}
	return &Expr{source: src, program: program, refs: refsOf(tree.Node)}, nil
}

func (e *Expr) Eval(env map[string]any) (any, error) {
	v, err := expr.Run(e.program, env)
	if err != nil {
		return nil, errors.New(firstLine(err.Error()))
	}
	return v, nil
}

func (e *Expr) Refs() []Ref { return e.refs }

// Render evaluates every expression against env and joins the result.
func (t *Template) Render(env map[string]any) (string, error) {
	var b strings.Builder
	for _, p := range t.parts {
		if p.program == nil {
			b.WriteString(p.text)
			continue
		}
		v, err := expr.Run(p.program, env)
		if err != nil {
			return "", fmt.Errorf("in {{ %s }}: %s", p.source, firstLine(err.Error()))
		}
		s, err := stringify(v)
		if err != nil {
			return "", fmt.Errorf("in {{ %s }}: %w", p.source, err)
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

// Refs lists what the template's expressions read, in order of appearance.
func (t *Template) Refs() []Ref {
	var out []Ref
	for _, p := range t.parts {
		out = append(out, p.refs...)
	}
	return out
}

func stringify(v any) (string, error) {
	switch v := v.(type) {
	case nil:
		// Missing map keys evaluate to nil; treating that as an error turns
		// typos like `outputs.stdot` into a clear failure instead of "".
		return "", errors.New("has no value; check the name")
	case string:
		return v, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("value can't be written as text: %w", err)
	}
	return string(data), nil
}

// refsOf finds member accesses rooted at steps/inputs/secrets, such as
// steps.tag or inputs["version"], and uses of item and index.
func refsOf(node ast.Node) []Ref {
	v := &refVisitor{}
	ast.Walk(&node, v)
	return v.refs
}

type refVisitor struct{ refs []Ref }

func (v *refVisitor) Visit(node *ast.Node) {
	if id, ok := (*node).(*ast.IdentifierNode); ok && (id.Value == "item" || id.Value == "index") {
		v.refs = append(v.refs, Ref{Kind: "item", Name: id.Value})
		return
	}
	m, ok := (*node).(*ast.MemberNode)
	if !ok {
		return
	}
	root, ok := m.Node.(*ast.IdentifierNode)
	if !ok {
		return
	}
	switch root.Value {
	case "steps", "inputs", "secrets":
	default:
		return
	}
	if prop, ok := m.Property.(*ast.StringNode); ok {
		v.refs = append(v.refs, Ref{Kind: root.Value, Name: prop.Value})
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
