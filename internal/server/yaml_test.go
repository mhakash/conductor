package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"conductor/internal/workflow"
)

func postYAML(t *testing.T, url, body string, out any) int {
	t.Helper()
	res, err := http.Post(url, "application/yaml", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return res.StatusCode
}

func TestYAMLImportExport(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api/workflows"

	var child workflow.Workflow
	if code := postYAML(t, base+"/import", `
conductor: 1
name: Notify
inputs:
  - name: msg
    type: text
steps:
  - id: say
    block: shell
    config:
      command: echo {{ inputs.msg }}
`, &child); code != http.StatusCreated {
		t.Fatalf("import child: status %d", code)
	}

	// The parent refers to the child by name; it is stored by id.
	var parent workflow.Workflow
	code := postYAML(t, base+"/import", `
conductor: 1
name: Release
steps:
  - id: build
    block: shell
    config: {command: make}
  - id: notify
    block: workflow
    needs: [build]
    config:
      workflow: Notify
      inputs: {msg: built}
`, &parent)
	if code != http.StatusCreated {
		t.Fatalf("import parent: status %d", code)
	}
	if !strings.Contains(string(parent.Definition.Steps[1].Config), child.ID) || len(parent.Definition.Edges) != 1 {
		t.Errorf("imported parent = %+v", parent.Definition)
	}

	res, err := http.Get(base + "/" + parent.ID + "/export")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if cd := res.Header.Get("Content-Disposition"); cd != `attachment; filename="release.yaml"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if text := string(body); !strings.Contains(text, "workflow: Notify") || !strings.Contains(text, "needs: [build]") {
		t.Errorf("export:\n%s", text)
	}

	// Problems come back like a failed save.
	var bad struct {
		Problems []workflow.Problem `json:"problems"`
	}
	tests := map[string]string{
		"conductor: 1\nname: X\nsteps: [{id: a, block: nope}]":                                  `unknown block "nope"`,
		"conductor: 1\nname: X\nsteps: [{id: a, block: workflow, config: {workflow: Missing}}]": "no workflow with that name",
		"conductor: 1\nname: X\nstesp: []":                                                      `unknown key "stesp"`,
	}
	for doc, want := range tests {
		code := postYAML(t, base+"/import", doc, &bad)
		if code != http.StatusUnprocessableEntity || len(bad.Problems) == 0 || !strings.Contains(bad.Problems[0].Message, want) {
			t.Errorf("%q: status %d, problems %+v", doc, code, bad.Problems)
		}
	}
}

func TestYAMLConvert(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api/workflows"
	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{{ID: "a", Block: "shell", Config: json.RawMessage(`{"command":"echo hi"}`)}}

	res, err := http.Post(base+"/to-yaml", "application/json",
		strings.NewReader(`{"name":"Draft","definition":`+mustJSON(t, def)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	text, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(text), "command: echo hi") {
		t.Fatalf("to-yaml:\n%s", text)
	}

	var back struct {
		Name       string              `json:"name"`
		Definition workflow.Definition `json:"definition"`
	}
	edited := strings.Replace(string(text), "echo hi", "echo bye", 1)
	if code := postYAML(t, base+"/from-yaml", edited, &back); code != http.StatusOK {
		t.Fatalf("from-yaml: status %d", code)
	}
	if back.Name != "Draft" || string(back.Definition.Steps[0].Config) != `{"command":"echo bye"}` {
		t.Errorf("from-yaml = %+v", back)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
