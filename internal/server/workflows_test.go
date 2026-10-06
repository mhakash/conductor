package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"conductor/internal/run"
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

	// Saving on top of a version someone else replaced is refused.
	stale := map[string]any{"name": "Stale", "definition": def, "updatedAt": created.UpdatedAt}
	if code := do(t, "PUT", base+"/"+created.ID, stale, nil); code != http.StatusConflict {
		t.Errorf("stale save: status %d, want 409", code)
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

func TestRunWorkflow(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api"

	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{
		{ID: "hello", Block: "shell", Config: json.RawMessage(`{"command":"echo hello","shell":"sh"}`)},
		{ID: "call", Block: "http"},
	}
	var wf workflow.Workflow
	do(t, "POST", base+"/workflows", map[string]any{"name": "Run me", "definition": def}, &wf)

	// The http step has no URL yet, so the run is refused with a problem on it.
	var refused struct {
		Problems []workflow.Problem `json:"problems"`
	}
	if code := do(t, "POST", base+"/workflows/"+wf.ID+"/runs", nil, &refused); code != http.StatusUnprocessableEntity {
		t.Fatalf("start: status %d, want 422", code)
	}
	if len(refused.Problems) != 1 || refused.Problems[0].Step != "call" {
		t.Fatalf("problems = %+v", refused.Problems)
	}

	def.Steps = def.Steps[:1]
	do(t, "PUT", base+"/workflows/"+wf.ID, map[string]any{"name": "Run me", "definition": def}, nil)

	var started run.Run
	if code := do(t, "POST", base+"/workflows/"+wf.ID+"/runs", nil, &started); code != http.StatusCreated {
		t.Fatalf("start: status %d", code)
	}

	var got run.Run
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		do(t, "GET", base+"/runs/"+started.ID, nil, &got)
		if got.Status.Done() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got.Status != run.StatusSucceeded {
		t.Fatalf("run status = %s (%s)", got.Status, got.Error)
	}

	var lines []run.LogLine
	do(t, "GET", base+"/runs/"+started.ID+"/logs?step=hello", nil, &lines)
	found := false
	for _, l := range lines {
		found = found || (l.Stream == run.StreamStdout && l.Text == "hello")
	}
	if !found {
		t.Errorf("logs = %+v, want stdout hello", lines)
	}

	var list []run.Summary
	do(t, "GET", base+"/runs?workflowId="+wf.ID, nil, &list)
	if len(list) != 1 {
		t.Errorf("list = %+v", list)
	}

	if code := do(t, "POST", base+"/runs/"+started.ID+"/cancel", nil, nil); code != http.StatusConflict {
		t.Errorf("cancel finished run: status %d, want 409", code)
	}
}

func TestApprovalDecision(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api"

	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{
		{ID: "approve", Block: "approval", Config: json.RawMessage(`{"message":"Go?"}`)},
		{ID: "after", Block: "shell", Config: json.RawMessage(`{"command":"echo done","shell":"sh"}`)},
	}
	def.Edges = []workflow.Edge{{From: "approve", To: "after"}}
	var wf workflow.Workflow
	do(t, "POST", base+"/workflows", map[string]any{"name": "Gate", "definition": def}, &wf)
	var started run.Run
	do(t, "POST", base+"/workflows/"+wf.ID+"/runs", nil, &started)

	var waiting []run.Summary
	deadline := time.Now().Add(10 * time.Second)
	for len(waiting) == 0 && time.Now().Before(deadline) {
		do(t, "GET", base+"/runs?status=waiting", nil, &waiting)
		time.Sleep(20 * time.Millisecond)
	}
	if len(waiting) != 1 || waiting[0].ID != started.ID {
		t.Fatalf("waiting runs = %+v", waiting)
	}

	decision := base + "/runs/" + started.ID + "/steps/%s/decision"
	if code := do(t, "POST", fmt.Sprintf(decision, "after"), map[string]any{"approve": true}, nil); code != http.StatusConflict {
		t.Errorf("deciding a step that isn't waiting: status %d, want 409", code)
	}
	if code := do(t, "POST", fmt.Sprintf(decision, "approve"), map[string]any{"approve": true, "comment": "go"}, nil); code != http.StatusAccepted {
		t.Fatalf("approve: status %d", code)
	}

	var got run.Run
	for time.Now().Before(deadline) {
		do(t, "GET", base+"/runs/"+started.ID, nil, &got)
		if got.Status.Done() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got.Status != run.StatusSucceeded || string(got.Steps[0].Outputs) != `{"approved":true,"comment":"go"}` {
		t.Errorf("run = %s, approval outputs %s", got.Status, got.Steps[0].Outputs)
	}
}

func TestSecretsAndInputsEndToEnd(t *testing.T) {
	srv := newTestServer(t)
	base := srv.URL + "/api"

	if code := do(t, "PUT", base+"/secrets/lower-case", map[string]string{"value": "x"}, nil); code != http.StatusUnprocessableEntity {
		t.Errorf("bad name: status %d", code)
	}
	if code := do(t, "PUT", base+"/secrets/API_TOKEN", map[string]string{"value": "tok-9f8e7d"}, nil); code != http.StatusOK {
		t.Fatalf("put secret: status %d", code)
	}
	var list []map[string]any
	do(t, "GET", base+"/secrets", nil, &list)
	if len(list) != 1 || list[0]["name"] != "API_TOKEN" || list[0]["value"] != nil {
		t.Errorf("list = %v (values must never be returned)", list)
	}

	def := workflow.EmptyDefinition()
	def.Inputs = []workflow.Input{{Name: "version", Type: workflow.InputText, Required: true}}
	def.Steps = []workflow.Step{{ID: "say", Block: "shell", Config: json.RawMessage(
		`{"command":"echo $TOKEN {{ inputs.version }}","shell":"sh","env":{"TOKEN":"{{ secrets.API_TOKEN }}"}}`)}}
	var wf workflow.Workflow
	do(t, "POST", base+"/workflows", map[string]any{"name": "Secret", "definition": def}, &wf)

	var started run.Run
	if code := do(t, "POST", base+"/workflows/"+wf.ID+"/runs", map[string]any{"inputs": map[string]any{"version": "v2"}}, &started); code != http.StatusCreated {
		t.Fatalf("start: status %d", code)
	}
	var got run.Run
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		do(t, "GET", base+"/runs/"+started.ID, nil, &got)
		if got.Status.Done() {
			break
		}
	}
	if got.Status != run.StatusSucceeded {
		t.Fatalf("run = %s %+v", got.Status, got.Steps)
	}
	out := string(got.Steps[0].Outputs) + string(got.Steps[0].Config)
	if strings.Contains(out, "tok-9f8e7d") || !strings.Contains(string(got.Steps[0].Outputs), "v2") {
		t.Errorf("outputs/config = %s", out)
	}
}

func TestScratchRun(t *testing.T) {
	srv := newTestServer(t)
	def := workflow.EmptyDefinition()
	def.Inputs = []workflow.Input{{Name: "who", Type: workflow.InputText}}
	def.Steps = []workflow.Step{{ID: "hi", Block: "shell", Config: json.RawMessage(`{"command":"echo hi {{ inputs.who }}"}`)}}

	var started run.Run
	body := map[string]any{"workflow": map[string]any{"definition": def}, "inputs": map[string]any{"who": "there"}}
	if code := do(t, "POST", srv.URL+"/api/runs", body, &started); code != http.StatusCreated {
		t.Fatalf("start: status %d", code)
	}
	if started.WorkflowID != "" || started.WorkflowName != "Scratch" {
		t.Errorf("started %+v", started)
	}
	deadline := time.Now().Add(10 * time.Second)
	var got run.Run
	for time.Now().Before(deadline) && !got.Status.Done() {
		time.Sleep(20 * time.Millisecond)
		do(t, "GET", srv.URL+"/api/runs/"+started.ID, nil, &got)
	}
	if got.Status != run.StatusSucceeded || !strings.Contains(string(got.Steps[0].Outputs), `"stdout":"hi there"`) {
		t.Errorf("run %s, outputs %s", got.Status, got.Steps[0].Outputs)
	}
	var list []workflow.Summary
	do(t, "GET", srv.URL+"/api/workflows", nil, &list)
	if len(list) != 0 {
		t.Errorf("a scratch run saved a workflow: %+v", list)
	}

	// A draft of a saved workflow runs as that workflow, unsaved.
	var saved workflow.Workflow
	do(t, "POST", srv.URL+"/api/workflows", map[string]any{"name": "Saved"}, &saved)
	draft := map[string]any{"workflowId": saved.ID, "workflow": map[string]any{"name": "Saved", "definition": def}, "inputs": map[string]any{"who": "x"}}
	if code := do(t, "POST", srv.URL+"/api/runs", draft, &started); code != http.StatusCreated || started.WorkflowID != saved.ID {
		t.Errorf("draft run: status %d, workflow %q", code, started.WorkflowID)
	}
	var after workflow.Workflow
	do(t, "GET", srv.URL+"/api/workflows/"+saved.ID, nil, &after)
	if len(after.Definition.Steps) != 0 {
		t.Errorf("running a draft saved it: %+v", after.Definition.Steps)
	}

	var res struct {
		Problems []workflow.Problem `json:"problems"`
	}
	bad := map[string]any{"workflow": map[string]any{"definition": map[string]any{"schemaVersion": 1, "steps": []any{map[string]any{"id": "x", "block": "nope"}}}}}
	if code := do(t, "POST", srv.URL+"/api/runs", bad, &res); code != http.StatusUnprocessableEntity || len(res.Problems) == 0 {
		t.Errorf("invalid: status %d, %+v", code, res.Problems)
	}
}
