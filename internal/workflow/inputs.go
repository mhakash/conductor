package workflow

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type InputType string

const (
	InputText    InputType = "text"
	InputNumber  InputType = "number"
	InputBoolean InputType = "boolean"
	// InputLongText is text written over several lines.
	InputLongText InputType = "longtext"
	// InputSelect is one of Input.Options.
	InputSelect InputType = "select"
)

// Input is a value asked for when a run starts, readable in expressions as
// inputs.<name>. Input steps ask for the same kind of values mid-run.
type Input struct {
	Name        string    `json:"name"`
	Label       string    `json:"label,omitempty"`
	Type        InputType `json:"type"`
	Options     []string  `json:"options,omitempty"`
	Default     any       `json:"default,omitempty"`
	Required    bool      `json:"required,omitempty"`
	Description string    `json:"description,omitempty"`
}

var inputNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)

// validateInputs checks input definitions; noun names them in messages
// ("input", or "field" for an input step's). With exprDefaults, a default
// may be an expression, checked once it is rendered.
func validateInputs(inputs []Input, noun string, exprDefaults bool) []Problem {
	var problems []Problem
	add := func(format string, args ...any) {
		problems = append(problems, Problem{Message: noun + " " + fmt.Sprintf(format, args...)})
	}
	seen := map[string]bool{}
	for _, in := range inputs {
		switch {
		case !inputNamePattern.MatchString(in.Name):
			add("name %q must start with a letter and use only letters, digits and _", in.Name)
		case seen[in.Name]:
			add("%q is defined more than once", in.Name)
		}
		seen[in.Name] = true

		switch in.Type {
		case InputText, InputNumber, InputBoolean, InputLongText, InputSelect:
		default:
			add("%q has unknown type %q", in.Name, in.Type)
			continue
		}
		if in.Type == InputSelect {
			if len(in.Options) == 0 {
				add("%q needs at least one option", in.Name)
			}
			for i, o := range in.Options {
				switch {
				case strings.TrimSpace(o) == "":
					add("%q has an empty option", in.Name)
				case slices.Contains(in.Options[:i], o):
					add("%q has option %q more than once", in.Name, o)
				}
			}
		} else if len(in.Options) > 0 {
			add("%q: only select %ss have options", in.Name, noun)
		}
		if s, ok := in.Default.(string); ok && exprDefaults && strings.Contains(s, "{{") {
			continue
		}
		if in.Default != nil {
			if _, err := coerce(in, in.Default); err != nil {
				add("%q: default %v", in.Name, err)
			}
		}
	}
	return problems
}

// ResolveInputs applies defaults and type conversion to the values given
// for a run. Every declared input appears in the result.
func ResolveInputs(defs []Input, given map[string]any) (map[string]any, error) {
	return resolve(defs, given, "input")
}

// ResolveFields does the same for the values given to an input step.
func ResolveFields(defs []Input, given map[string]any) (map[string]any, error) {
	return resolve(defs, given, "field")
}

func resolve(defs []Input, given map[string]any, noun string) (map[string]any, error) {
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
				problems = append(problems, Problem{Message: fmt.Sprintf("%s %q is required", noun, d.Name)})
				continue
			}
			if d.Type == InputSelect {
				out[d.Name] = "" // nothing chosen
				continue
			}
			v = zero(d.Type)
		}
		c, err := coerce(d, v)
		if err != nil {
			problems = append(problems, Problem{Message: fmt.Sprintf("%s %q: %v", noun, d.Name, err)})
			continue
		}
		out[d.Name] = c
	}
	for name := range given {
		if !declared[name] {
			problems = append(problems, Problem{Message: fmt.Sprintf("there's no %s called %q", noun, name)})
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

func coerce(in Input, v any) (any, error) {
	t := in.Type
	switch t {
	case InputSelect:
		if s, ok := v.(string); ok && slices.Contains(in.Options, s) {
			return s, nil
		}
		return nil, fmt.Errorf("%v is not one of the options", v)
	case InputText, InputLongText:
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
