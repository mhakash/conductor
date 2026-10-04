package tmpl

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

var env = map[string]any{
	"inputs": map[string]any{"version": "v1.4.0", "dryRun": true},
	"steps": map[string]any{
		"pipeline": map[string]any{"outputs": map[string]any{
			"status": 200,
			"json":   map[string]any{"status": "success", "jobs": []any{"build", "test"}},
		}},
	},
	"secrets": map[string]any{"GITLAB_TOKEN": "glpat-123"},
}

func TestRender(t *testing.T) {
	tests := []struct{ in, want string }{
		{"no expressions", "no expressions"},
		{"git tag {{ inputs.version }}", "git tag v1.4.0"},
		{"{{ 'Release ' + inputs.version }}!", "Release v1.4.0!"},
		{"status={{steps.pipeline.outputs.json.status}}", "status=success"},
		{"{{ steps.pipeline.outputs.status }}", "200"},
		{"{{ steps.pipeline.outputs.json.jobs }}", `["build","test"]`},
		{"{{ inputs.dryRun ? '--dry-run' : '' }}", "--dry-run"},
		{"{{ upper(inputs.version) }} {{ len(steps.pipeline.outputs.json.jobs) }}", "V1.4.0 2"},
		{`token {{ secrets["GITLAB_TOKEN"] }}`, "token glpat-123"},
	}
	for _, tt := range tests {
		tpl, err := Compile(tt.in)
		if err != nil {
			t.Fatalf("Compile(%q): %v", tt.in, err)
		}
		got, err := tpl.Render(env)
		if err != nil {
			t.Fatalf("Render(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("Render(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestErrors(t *testing.T) {
	for _, in := range []string{"{{ inputs.version", "{{ }}", "{{ inputs. }}"} {
		if _, err := Compile(in); err == nil {
			t.Errorf("Compile(%q) succeeded", in)
		}
	}

	tpl, _ := Compile("{{ inputs.verison }}")
	if _, err := tpl.Render(env); err == nil || !strings.Contains(err.Error(), "inputs.verison") {
		t.Errorf("typo did not fail clearly: %v", err)
	}
}

func TestRefs(t *testing.T) {
	tpl, err := Compile(`{{ steps.tag.outputs.stdout }} {{ inputs["version"] }} {{ secrets.TOKEN }} {{ run.id }}`)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprint(tpl.Refs())
	want := "[{steps tag} {inputs version} {secrets TOKEN}]"
	if got != want {
		t.Errorf("Refs = %s, want %s", got, want)
	}
}

func TestRenderConfig(t *testing.T) {
	raw := json.RawMessage(`{
		"url": "https://gitlab/api/{{ inputs.version }}",
		"headers": {"PRIVATE-TOKEN": "{{ secrets.GITLAB_TOKEN }}"},
		"retries": 3,
		"tags": ["{{ inputs.version }}", "latest"]
	}`)
	out, err := RenderConfig(raw, env)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if got["url"] != "https://gitlab/api/v1.4.0" ||
		got["headers"].(map[string]any)["PRIVATE-TOKEN"] != "glpat-123" ||
		got["retries"] != float64(3) ||
		got["tags"].([]any)[0] != "v1.4.0" {
		t.Errorf("rendered = %s", out)
	}

	_, err = RenderConfig(json.RawMessage(`{"headers":{"X":"{{ nope.missing }}"}}`), env)
	if err == nil || !strings.HasPrefix(err.Error(), "headers.X:") {
		t.Errorf("error should name the field: %v", err)
	}
}

func TestConfigRefs(t *testing.T) {
	refs, err := ConfigRefs(json.RawMessage(`{"command":"echo {{ steps.a.outputs.stdout }}","env":{"T":"{{ secrets.T }}"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(refs) != "[{steps a} {secrets T}]" {
		t.Errorf("refs = %v", refs)
	}
}

func TestExpr(t *testing.T) {
	e, err := CompileExpr(`steps.pipeline.outputs.json.status == "success" && !inputs.dryRun`)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := e.Eval(env); err != nil || v != false {
		t.Errorf("Eval = %v, %v; want false", v, err)
	}
	if got := fmt.Sprint(e.Refs()); got != "[{steps pipeline} {inputs dryRun}]" {
		t.Errorf("Refs = %s", got)
	}

	loop, _ := CompileExpr(`"KEY-" + item.id + " #" + string(index)`)
	if got := fmt.Sprint(loop.Refs()); got != "[{item item} {item index}]" {
		t.Errorf("loop Refs = %s", got)
	}

	for _, bad := range []string{"", "  ", "a ==", "{{ a }}"} {
		if _, err := CompileExpr(bad); err == nil {
			t.Errorf("CompileExpr(%q) succeeded", bad)
		}
	}
}
