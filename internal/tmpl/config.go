package tmpl

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// RenderConfig evaluates the expressions in every string of a JSON config
// (nested objects and arrays included; object keys are left alone).
func RenderConfig(raw json.RawMessage, env map[string]any) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("config is not valid JSON: %w", err)
	}
	out, err := walk(v, "", func(path, s string) (string, error) {
		t, err := Compile(s)
		if err != nil {
			return "", fieldErr(path, err)
		}
		r, err := t.Render(env)
		if err != nil {
			return "", fieldErr(path, err)
		}
		return r, nil
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

// ConfigRefs compiles every expression in a JSON config and returns what
// they reference. It fails on the first expression that doesn't compile.
func ConfigRefs(raw json.RawMessage) ([]Ref, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("config is not valid JSON: %w", err)
	}
	var refs []Ref
	_, err := walk(v, "", func(path, s string) (string, error) {
		t, err := Compile(s)
		if err != nil {
			return "", fieldErr(path, err)
		}
		refs = append(refs, t.Refs()...)
		return s, nil
	})
	return refs, err
}

// walk applies fn to every string containing an expression and returns the
// rebuilt value. path is a dotted field path for error messages.
func walk(v any, path string, fn func(path, s string) (string, error)) (any, error) {
	switch v := v.(type) {
	case string:
		if !Has(v) {
			return v, nil
		}
		return fn(path, v)
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys) // deterministic error reporting
		out := make(map[string]any, len(v))
		for _, k := range keys {
			r, err := walk(v[k], join(path, k), fn)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			r, err := walk(item, fmt.Sprintf("%s[%d]", path, i), fn)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	}
	return v, nil
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func fieldErr(path string, err error) error {
	if path == "" {
		return err
	}
	return fmt.Errorf("%s: %w", strings.TrimPrefix(path, "."), err)
}
