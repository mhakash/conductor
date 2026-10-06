package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"conductor/internal/preset"
	"conductor/internal/workflow"
)

func TestPresetsAPI(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api/presets"

	body := map[string]any{
		"name": " GitLab API ", "block": "http",
		"step": map[string]any{"config": map[string]any{"url": "https://gitlab.example.com/api/v4", "headers": map[string]string{"PRIVATE-TOKEN": "{{ secrets.GITLAB }}"}}, "retry": map[string]any{"max": 2}},
	}
	var created preset.Preset
	if code := do(t, "POST", base, body, &created); code != http.StatusCreated {
		t.Fatalf("create: status %d", code)
	}
	if created.Name != "GitLab API" || created.Step.Retry.Max != 2 {
		t.Errorf("created %+v", created)
	}
	if code := do(t, "POST", base, body, nil); code != http.StatusConflict {
		t.Errorf("duplicate: status %d, want 409", code)
	}

	var bad struct {
		Problems []workflow.Problem `json:"problems"`
	}
	invalid := map[string]any{"name": "Broken", "block": "nope", "step": map[string]any{"config": map[string]any{"url": "{{ item }}"}, "timeout": "soon"}}
	if code := do(t, "POST", base, invalid, &bad); code != http.StatusUnprocessableEntity || len(bad.Problems) != 3 {
		t.Errorf("invalid: status %d, problems %+v", code, bad.Problems)
	}

	body["description"] = "Base URL and token"
	var updated preset.Preset
	if code := do(t, "PUT", base+"/"+created.ID, body, &updated); code != http.StatusOK || updated.Description != "Base URL and token" {
		t.Errorf("update: status %d, %+v", code, updated)
	}
	var list []preset.Preset
	do(t, "GET", base, nil, &list)
	if len(list) != 1 {
		t.Errorf("list = %+v", list)
	}
	if code := do(t, "DELETE", base+"/"+created.ID, nil, nil); code != http.StatusNoContent {
		t.Errorf("delete: status %d", code)
	}
	if code := do(t, "DELETE", base+"/"+created.ID, nil, nil); code != http.StatusNotFound {
		t.Errorf("delete again: status %d", code)
	}
}

func TestSubWorkflowsAPI(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api/workflows"

	childDef := workflow.EmptyDefinition()
	childDef.Inputs = []workflow.Input{{Name: "msg", Type: workflow.InputText}}
	childDef.Steps = []workflow.Step{{ID: "say", Block: "shell", Config: json.RawMessage(`{"command":"echo {{ inputs.msg }}"}`)}}
	childDef.Outputs = []workflow.Output{{Name: "said", Value: "steps.say.outputs.stdout"}}
	var child workflow.Workflow
	if code := do(t, "POST", base, map[string]any{"name": "Say", "definition": childDef}, &child); code != http.StatusCreated {
		t.Fatalf("create child: status %d", code)
	}

	call := func(inputs string) workflow.Definition {
		def := workflow.EmptyDefinition()
		def.Steps = []workflow.Step{{ID: "say", Block: workflow.CallBlock,
			Config: json.RawMessage(`{"workflow":"` + child.ID + `","inputs":` + inputs + `}`)}}
		def.Outputs = []workflow.Output{{Name: "said", Value: "steps.say.outputs.said"}}
		return def
	}
	var res struct {
		Problems []workflow.Problem `json:"problems"`
	}
	code := do(t, "POST", base, map[string]any{"name": "Parent", "definition": call(`{"mesage":"hi"}`)}, &res)
	if code != http.StatusUnprocessableEntity || len(res.Problems) != 1 || !strings.Contains(res.Problems[0].Message, `input "mesage"`) {
		t.Errorf("unknown input: status %d, %+v", code, res.Problems)
	}
	var parent workflow.Workflow
	if code := do(t, "POST", base, map[string]any{"name": "Parent", "definition": call(`{"msg":"hi"}`)}, &parent); code != http.StatusCreated {
		t.Fatalf("create parent: status %d", code)
	}

	// The child can't start running its parent.
	childDef.Steps = append(childDef.Steps, workflow.Step{ID: "back", Block: workflow.CallBlock,
		Config: json.RawMessage(`{"workflow":"` + parent.ID + `"}`)})
	code = do(t, "PUT", base+"/"+child.ID, map[string]any{"name": "Say", "definition": childDef}, &res)
	if code != http.StatusUnprocessableEntity || !strings.Contains(res.Problems[0].Message, "Say → Parent → Say") {
		t.Errorf("cycle: status %d, %+v", code, res.Problems)
	}

	var errBody struct{ Error string }
	if code := do(t, "DELETE", base+"/"+child.ID, nil, &errBody); code != http.StatusConflict || !strings.Contains(errBody.Error, "Parent runs this workflow") {
		t.Errorf("delete child: status %d %q", code, errBody.Error)
	}
	if code := do(t, "DELETE", base+"/"+parent.ID, nil, nil); code != http.StatusNoContent {
		t.Errorf("delete parent: status %d", code)
	}
}
