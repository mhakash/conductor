package workflow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type InputType string

const (
	InputText    InputType = "text"
	InputNumber  InputType = "number"
	InputBoolean InputType = "boolean"
)

// Input is a value asked for when a run starts, readable in expressions as
// inputs.<name>.
type Input struct {
	Name        string    `json:"name"`
	Label       string    `json:"label,omitempty"`
	Type        InputType `json:"type"`
	Default     any       `json:"default,omitempty"`
	Required    bool      `json:"required,omitempty"`
	Description string    `json:"description,omitempty"`
}

var inputNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)

func validateInputs(inputs []Input) []Problem {
	var problems []Problem
	seen := map[string]bool{}
	for _, in := range inputs {
		switch {
		case !inputNamePattern.MatchString(in.Name):
			problems = append(problems, Problem{Message: fmt.Sprintf("input name %q must start with a letter and use only letters, digits and _", in.Name)})
		case seen[in.Name]:
			problems = append(problems, Problem{Message: fmt.Sprintf("input %q is defined more than once", in.Name)})
		}
		seen[in.Name] = true

		switch in.Type {
		case InputText, InputNumber, InputBoolean:
			if in.Default != nil {
				if _, err := coerce(in.Type, in.Default); err != nil {
					problems = append(problems, Problem{Message: fmt.Sprintf("input %q: default %v", in.Name, err)})
				}
			}
		default:
			problems = append(problems, Problem{Message: fmt.Sprintf("input %q has unknown type %q", in.Name, in.Type)})
		}
	}
	return problems
}

// ResolveInputs applies defaults and type conversion to the values given
// for a run. Every declared input appears in the result.
func ResolveInputs(defs []Input, given map[string]any) (map[string]any, error) {
	var problems []Problem
	out := make(map[string]any, len(defs))
	declared := map[string]bool{}

	for _, d := range defs {
		declared[d.Name] = true
		v, ok := given[d.Name]
		if !ok || v == nil || v == "" {
			v = d.Default
		}
		if v == nil || v == "" {
			if d.Required {
				problems = append(problems, Problem{Message: fmt.Sprintf("input %q is required", d.Name)})
				continue
			}
			v = zero(d.Type)
		}
		c, err := coerce(d.Type, v)
		if err != nil {
			problems = append(problems, Problem{Message: fmt.Sprintf("input %q: %v", d.Name, err)})
			continue
		}
		out[d.Name] = c
	}
	for name := range given {
		if !declared[name] {
			problems = append(problems, Problem{Message: fmt.Sprintf("this workflow has no input called %q", name)})
		}
	}
	if len(problems) > 0 {
		return nil, &ValidationError{Problems: problems}
	}
	return out, nil
}

func zero(t InputType) any {
	switch t {
	case InputNumber:
		return 0.0
	case InputBoolean:
		return false
	}
	return ""
}

func coerce(t InputType, v any) (any, error) {
	switch t {
	case InputText:
		switch v := v.(type) {
		case string:
			return v, nil
		case float64, bool:
			return fmt.Sprint(v), nil
		}
	case InputNumber:
		switch v := v.(type) {
		case float64:
			return v, nil
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return f, nil
			}
			return nil, fmt.Errorf("%q is not a number", v)
		}
	case InputBoolean:
		switch v := v.(type) {
		case bool:
			return v, nil
		case string:
			if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
				return b, nil
			}
			return nil, fmt.Errorf("%q is not true or false", v)
		}
	}
	return nil, fmt.Errorf("must be a %s, got %v", t, v)
}
