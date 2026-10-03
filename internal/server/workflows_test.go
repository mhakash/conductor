package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"conductor/internal/workflow"
)

func do(t *testing.T, method, url string, body any, out any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s: %v", method, url, err)
		}
	}
	return res.StatusCode
}

func TestWorkflowLifecycle(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api/workflows"

	var created workflow.Workflow
	if code := do(t, "POST", base, map[string]any{"name": "  Build prep "}, &created); code != http.StatusCreated {
		t.Fatalf("create: status %d", code)
	}
	if created.Name != "Build prep" || created.Definition.SchemaVersion != workflow.SchemaVersion {
		t.Fatalf("create returned %+v", created)
	}

	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{
		{ID: "tag", Block: "shell", Name: "Create tag"},
		{ID: "push", Block: "shell", Name: "Push"},
	}
	def.Edges = []workflow.Edge{{From: "tag", To: "push"}}

	var updated workflow.Workflow
	code := do(t, "PUT", base+"/"+created.ID, map[string]any{"name": "Build prep", "definition": def}, &updated)
	if code != http.StatusOK {
		t.Fatalf("update: status %d", code)
	}
	if string(updated.Definition.Steps[0].Config) != "{}" {
		t.Errorf("missing config not normalised to {}: %s", updated.Definition.Steps[0].Config)
	}

	var list []workflow.Summary
	do(t, "GET", base, nil, &list)
	if len(list) != 1 || list[0].StepCount != 2 {
		t.Errorf("list = %+v", list)
	}

	if code := do(t, "DELETE", base+"/"+created.ID, nil, nil); code != http.StatusNoContent {
		t.Errorf("delete: status %d", code)
	}
	if code := do(t, "GET", base+"/"+created.ID, nil, nil); code != http.StatusNotFound {
		t.Errorf("get after delete: status %d", code)
	}
}

func TestUpdateRejectsCycle(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api/workflows"

	var created workflow.Workflow
	do(t, "POST", base, map[string]any{"name": "Loop"}, &created)

	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{{ID: "a", Block: "shell"}, {ID: "b", Block: "shell"}}
	def.Edges = []workflow.Edge{{From: "a", To: "b"}, {From: "b", To: "a"}}

	var body struct {
		Problems []workflow.Problem `json:"problems"`
	}
	code := do(t, "PUT", base+"/"+created.ID, map[string]any{"name": "Loop", "definition": def}, &body)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", code)
	}
	if len(body.Problems) != 1 || body.Problems[0].Step != "a" {
		t.Errorf("problems = %+v", body.Problems)
	}
}

func TestListBlocks(t *testing.T) {
	srv := newTestServer(t)
	var blocks []struct{ Type string }
	do(t, "GET", srv.URL+"/api/blocks", nil, &blocks)
	if len(blocks) < 2 || blocks[0].Type != "shell" {
		t.Errorf("blocks = %+v", blocks)
	}
}
