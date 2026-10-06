package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"conductor/internal/run"
	"conductor/internal/workflow"
)

// save stores wf so steps can run it, and returns it with its id.
func (f fixture) save(t *testing.T, wf workflow.Workflow) workflow.Workflow {
	t.Helper()
	saved, err := f.workflows.Create(context.Background(), wf)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

// greeter takes a name and outputs a greeting.
func greeter() workflow.Workflow {
	wf := build([]stepSpec{{id: "say", block: "emit", config: `{"value":"hello {{ inputs.name }}"}`}})
	wf.Name = "Greet"
	wf.Definition.Inputs = []workflow.Input{{Name: "name", Type: workflow.InputText, Required: true}}
	wf.Definition.Outputs = []workflow.Output{{Name: "greeting", Value: "steps.say.outputs.value"}}
	return wf
}

func callStep(id, workflowID, inputs string) stepSpec {
	return stepSpec{id: id, block: workflow.CallBlock, config: `{"workflow":"` + workflowID + `","inputs":` + inputs + `}`}
}

func TestSubWorkflow(t *testing.T) {
	f := newFixture(t, 8)
	child := f.save(t, greeter())
	parent := f.save(t, build([]stepSpec{
		{id: "who", block: "emit", config: `{"value":"Ada"}`},
		callStep("greet", child.ID, `{"name":"{{ steps.who.outputs.value }}"}`),
		{id: "shout", block: "emit", config: `{"value":"{{ upper(steps.greet.outputs.greeting) }}"}`},
	}, "who>greet", "greet>shout"))

	r, _ := f.runToEnd(t, parent)
	if r.Status != run.StatusSucceeded {
		t.Fatalf("run %s: %v", r.Status, statuses(r))
	}
	if out := string(step(r, "shout").Outputs); out != `{"value":"HELLO ADA"}` {
		t.Errorf("shout outputs = %s", out)
	}
	if len(r.Children) != 1 {
		t.Fatalf("children = %+v", r.Children)
	}
	c := f.waitDone(t, r.Children[0].ID)
	if c.Parent == nil || c.Parent.RunID != r.ID || c.Parent.StepID != "greet" || c.Parent.WorkflowName != "Test" {
		t.Errorf("child parent = %+v", c.Parent)
	}
	if c.Outputs["greeting"] != "hello Ada" || c.Inputs["name"] != "Ada" {
		t.Errorf("child inputs %v, outputs %v", c.Inputs, c.Outputs)
	}

	top, _ := f.runs.List(context.Background(), run.ListFilter{TopLevel: true})
	if len(top) != 1 || top[0].ID != r.ID {
		t.Errorf("top-level runs = %+v", top)
	}
	logs, _ := f.runs.Logs(context.Background(), r.ID, "greet", 0, 0)
	if text := logText(logs); !strings.Contains(text, "Started Greet (run "+c.ID+")") || !strings.Contains(text, "Greet succeeded") {
		t.Errorf("greet logs:\n%s", text)
	}
}

func TestSubWorkflowFailure(t *testing.T) {
	f := newFixture(t, 8)
	child := build([]stepSpec{{id: "push", block: "fail"}})
	child.Name = "Release"
	child = f.save(t, child)
	r, _ := f.runToEnd(t, build([]stepSpec{
		callStep("release", child.ID, `{}`),
		{id: "after", block: "emit"},
	}, "release>after"))

	if st := step(r, "release"); st.Status != run.StatusFailed || st.Error != `Release failed at step "push": boom` {
		t.Errorf("release = %s %q", st.Status, st.Error)
	}
	if step(r, "after").Status != run.StatusSkipped || r.Status != run.StatusFailed {
		t.Errorf("run %s, steps %v", r.Status, statuses(r))
	}
}

func TestSubWorkflowInputsAreChecked(t *testing.T) {
	f := newFixture(t, 8)
	child := f.save(t, greeter())
	r, _ := f.runToEnd(t, build([]stepSpec{callStep("greet", child.ID, `{}`)}))
	if st := step(r, "greet"); st.Status != run.StatusFailed || st.Error != `input "name" is required` {
		t.Errorf("greet = %s %q", st.Status, st.Error)
	}
	if len(r.Children) != 0 {
		t.Errorf("a child run was started: %+v", r.Children)
	}
}

func TestSubWorkflowPerItem(t *testing.T) {
	f := newFixture(t, 8)
	child := f.save(t, greeter())
	r, _ := f.runToEnd(t, build([]stepSpec{{
		id: "greet", block: workflow.CallBlock,
		config: `{"workflow":"` + child.ID + `","inputs":{"name":"{{ item }}"}}`,
		edit: func(s *workflow.Step) {
			s.ForEach = &workflow.ForEach{Items: `["Ada", "Grace"]`, Concurrency: 2}
		},
	}}))
	if r.Status != run.StatusSucceeded {
		t.Fatalf("run %s: %v", r.Status, step(r, "greet").Error)
	}
	want := `{"results":[{"greeting":"hello Ada"},{"greeting":"hello Grace"}]}`
	if out := string(step(r, "greet").Outputs); out != want {
		t.Errorf("outputs = %s", out)
	}
	items := map[int]bool{}
	for _, c := range r.Children {
		items[*c.Parent.Item] = true
	}
	if len(r.Children) != 2 || !items[0] || !items[1] {
		t.Errorf("children = %+v", r.Children)
	}
}

func TestCancellingParentCancelsChild(t *testing.T) {
	f := newFixture(t, 8)
	child := f.save(t, build([]stepSpec{{id: "wait", block: "sleep", config: `{"ms":10000}`}}))
	r, err := f.engine.Start(context.Background(), build([]stepSpec{callStep("call", child.ID, `{}`)}), nil)
	if err != nil {
		t.Fatal(err)
	}
	running := f.waitFor(t, r.ID, func(r run.Run) bool { return len(r.Children) == 1 && r.Children[0].Status == run.StatusRunning })
	must(t, f.engine.Cancel(r.ID))

	if got := f.waitDone(t, r.ID); got.Status != run.StatusCancelled || step(got, "call").Status != run.StatusCancelled {
		t.Errorf("parent %s, steps %v", got.Status, statuses(got))
	}
	c := f.waitDone(t, running.Children[0].ID)
	if c.Status != run.StatusCancelled || c.Error != errParentStopped.Error() {
		t.Errorf("child %s %q", c.Status, c.Error)
	}
}

func TestSubWorkflowTimeoutStopsChild(t *testing.T) {
	f := newFixture(t, 8)
	child := f.save(t, build([]stepSpec{{id: "wait", block: "sleep", config: `{"ms":10000}`}}))
	call := callStep("call", child.ID, `{}`)
	call.timeout = "100ms"
	r, _ := f.runToEnd(t, build([]stepSpec{call}))
	if st := step(r, "call"); st.Status != run.StatusFailed || st.Error != "timed out after 100ms" {
		t.Errorf("call = %s %q", st.Status, st.Error)
	}
	if c := f.waitDone(t, r.Children[0].ID); c.Status != run.StatusCancelled {
		t.Errorf("child = %s", c.Status)
	}
}

func TestSubWorkflowSecretsStayMasked(t *testing.T) {
	f := newFixture(t, 8)
	child := build([]stepSpec{{id: "use", block: "emit", config: `{"value":"token={{ inputs.token }}"}`}})
	child.Definition.Inputs = []workflow.Input{{Name: "token", Type: workflow.InputText}}
	child.Definition.Outputs = []workflow.Output{{Name: "echo", Value: "steps.use.outputs.value"}}
	child = f.save(t, child)

	r, _ := f.runToEnd(t, build([]stepSpec{callStep("call", child.ID, `{"token":"{{ secrets.TOKEN }}"}`)}))
	if r.Status != run.StatusSucceeded {
		t.Fatalf("run %s: %q", r.Status, step(r, "call").Error)
	}
	c := f.waitDone(t, r.Children[0].ID)
	logs, _ := f.runs.Logs(context.Background(), c.ID, "", 0, 0)
	for name, text := range map[string]string{
		"child inputs":  c.Inputs["token"].(string),
		"child outputs": string(step(c, "use").Outputs),
		"child logs":    logText(logs),
		"parent step":   string(step(r, "call").Outputs),
	} {
		if strings.Contains(text, "s3cr3t-token") {
			t.Errorf("%s show the secret: %s", name, text)
		}
	}
}

func TestOutputsThatFailFailTheRun(t *testing.T) {
	f := newFixture(t, 8)
	wf := build([]stepSpec{{id: "a", block: "emit", config: `{"value":"x"}`}})
	wf.Definition.Outputs = []workflow.Output{{Name: "v", Value: "steps.a.outputs.vale"}}
	r, _ := f.runToEnd(t, wf)
	if r.Status != run.StatusFailed || !strings.Contains(r.Error, "output v: has no value") {
		t.Errorf("run %s %q", r.Status, r.Error)
	}
}

func TestCheckRejectsSelfCall(t *testing.T) {
	f := newFixture(t, 8)
	wf := f.save(t, build([]stepSpec{{id: "a", block: "emit"}}))
	wf.Definition.Steps = append(wf.Definition.Steps, workflow.Step{ID: "again", Block: workflow.CallBlock,
		Config: json.RawMessage(`{"workflow":"` + wf.ID + `"}`)})
	if err := f.engine.Check(context.Background(), wf); err == nil || !strings.Contains(err.Error(), "can't run the workflow it belongs to") {
		t.Errorf("Check = %v", err)
	}
}

// A child waiting for approval keeps its parent waiting too, through a
// restart: both resume and the parent picks up the child's result.
func TestSubWorkflowApprovalSurvivesRestart(t *testing.T) {
	f := newFixture(t, 8)
	child := approvalFlow()
	child.Name = "Release"
	child.Definition.Outputs = []workflow.Output{{Name: "pushed", Value: "steps.push.outputs.value"}}
	child = f.save(t, child)
	parent := build([]stepSpec{
		callStep("release", child.ID, `{}`),
		{id: "announce", block: "emit", config: `{"value":"{{ steps.release.outputs.pushed }}"}`},
	}, "release>announce")

	r, err := f.engine.Start(context.Background(), parent, nil)
	if err != nil {
		t.Fatal(err)
	}
	waiting := f.waitFor(t, r.ID, func(r run.Run) bool { return r.Status == run.StatusWaiting })
	childID := waiting.Children[0].ID
	if waiting.Children[0].Status != run.StatusWaiting || step(waiting, "release").Status != run.StatusRunning {
		t.Fatalf("child %s, steps %v", waiting.Children[0].Status, statuses(waiting))
	}
	must(t, f.engine.Shutdown(context.Background()))

	restarted := f.newEngine(t, 8)
	must(t, restarted.Recover(context.Background()))
	decide(t, restarted, childID, Decision{Approve: true, Comment: "ok"})

	got := f.waitDone(t, r.ID)
	if got.Status != run.StatusSucceeded {
		t.Fatalf("parent %s, steps %v (%q)", got.Status, statuses(got), step(got, "release").Error)
	}
	if out := string(step(got, "announce").Outputs); out != `{"value":"pushed v1.4.0 (ok)"}` {
		t.Errorf("announce outputs = %s", out)
	}
	if len(got.Children) != 1 {
		t.Errorf("the child ran again: %+v", got.Children)
	}
}

// After a crash, a child left waiting whose parent step is gone is failed
// instead of waiting forever.
func TestRecoverAbandonsOrphanedChild(t *testing.T) {
	f := newFixture(t, 8)
	ctx := context.Background()
	parentWF := build([]stepSpec{{id: "call", block: workflow.CallBlock}})
	p, _ := f.runs.Create(ctx, run.Run{WorkflowID: "p", WorkflowName: "Parent", Definition: parentWF.Definition})
	must(t, f.runs.FinishRun(ctx, p.ID, run.StatusFailed, nil, "", f.engine.now()))

	child := approvalFlow()
	c, _ := f.runs.Create(ctx, run.Run{WorkflowID: "c", WorkflowName: "Child", Definition: child.Definition,
		Parent: &run.Parent{RunID: p.ID, StepID: "call"}})
	must(t, f.runs.StartRun(ctx, c.ID, f.engine.now()))
	must(t, f.runs.StartStep(ctx, c.ID, "approve", run.StatusWaiting, nil, f.engine.now()))

	must(t, f.engine.Recover(ctx))
	if got, _ := f.runs.Get(ctx, c.ID); got.Status != run.StatusFailed || !strings.Contains(got.Error, "has finished") {
		t.Errorf("child = %s %q", got.Status, got.Error)
	}
}
