package wfyaml

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"conductor/internal/workflow"
)

func order(block string) []string {
	if block == "shell" {
		return []string{"command", "workdir", "env"}
	}
	return nil
}

func sample() workflow.Workflow {
	def := workflow.EmptyDefinition()
	def.Inputs = []workflow.Input{{Name: "version", Type: workflow.InputText, Default: "1.0.0", Required: true}, {Name: "count", Type: workflow.InputNumber, Default: 3.0}}
	def.Outputs = []workflow.Output{{Name: "tag", Value: "steps.tag.outputs.stdout", Description: "The tag"}}
	def.Steps = []workflow.Step{
		{ID: "tag", Name: "Create tag", Block: "shell", Config: json.RawMessage(`{"workdir":"","env":{"V":"{{ inputs.version }}"},"command":"git tag v$V\ngit push --tags"}`)},
		{ID: "notify", Block: "http", Config: json.RawMessage(`{"url":"https://example.com","method":"POST"}`),
			If: "steps.tag.status == 'succeeded'", Timeout: "30s", ContinueOnError: true,
			Retry: &workflow.Retry{Max: 2, Delay: "5s", Backoff: "exponential"}},
		{ID: "each", Block: "shell", Config: json.RawMessage(`{"command":"echo {{ item }}"}`),
			ForEach: &workflow.ForEach{Items: "[1, 2]", Concurrency: 2}},
		{ID: "approve", Block: "approval", Config: json.RawMessage(`{}`)},
	}
	def.Edges = []workflow.Edge{{From: "tag", To: "notify"}, {From: "tag", To: "each"}, {From: "notify", To: "approve"}, {From: "each", To: "approve"}}
	return workflow.Workflow{Name: "Release", Description: "Tags and tells people", Definition: def}
}

func TestRoundTrip(t *testing.T) {
	want := sample()
	data, err := Encode(want, order)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, s := range []string{
		"conductor: 1\nname: Release\n",
		"needs: [notify, each]",
		"command: |-\n        git tag v$V\n        git push --tags\n      env:", // multi-line literal, form order
	} {
		if !strings.Contains(text, s) {
			t.Errorf("YAML lacks %q:\n%s", s, text)
		}
	}
	if strings.Contains(text, "workdir") || strings.Contains(text, "position") {
		t.Errorf("empty values or positions written:\n%s", text)
	}

	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name || got.Description != want.Description ||
		!reflect.DeepEqual(got.Definition.Inputs, want.Definition.Inputs) ||
		!reflect.DeepEqual(got.Definition.Outputs, want.Definition.Outputs) ||
		!reflect.DeepEqual(got.Definition.Edges, want.Definition.Edges) {
		t.Errorf("round trip changed the workflow:\n got %+v\nwant %+v", got, want)
	}
	for i, s := range got.Definition.Steps {
		w := want.Definition.Steps[i]
		var gc, wc map[string]any
		_ = json.Unmarshal(s.Config, &gc)
		_ = json.Unmarshal(w.Config, &wc)
		delete(wc, "workdir")
		s.Config, w.Config, s.Position = nil, nil, workflow.Position{}
		if !reflect.DeepEqual(s, w) || !reflect.DeepEqual(gc, wc) {
			t.Errorf("step %d:\n got %+v %v\nwant %+v %v", i, s, gc, w, wc)
		}
	}
	// Columns by dependency depth.
	pos := map[string]workflow.Position{}
	for _, s := range got.Definition.Steps {
		pos[s.ID] = s.Position
	}
	if pos["tag"].X != 0 || pos["notify"].X != 320 || pos["each"] != (workflow.Position{X: 320, Y: 140}) || pos["approve"].X != 640 {
		t.Errorf("positions = %v", pos)
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := map[string]string{
		"steps: []":                       `needs "conductor: 1" at the top`,
		"conductor: 1\nname: x\nstep: []": `unknown key "step"`,
		"conductor: 1\nname: x\nsteps: [{id: a, block: shell, config: [1]}]": "config must be a mapping",
		"conductor: 1\nname: [": "line 2",
	}
	for doc, want := range tests {
		if _, err := Decode([]byte(doc)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error = %v, want %q", doc, err, want)
		}
	}
}
