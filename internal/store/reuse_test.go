package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"conductor/internal/preset"
	"conductor/internal/run"
	"conductor/internal/workflow"
)

func openDB(t *testing.T) (*Runs, *Workflows, *Presets) {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewRuns(db), NewWorkflows(db), NewPresets(db)
}

func TestChildRuns(t *testing.T) {
	ctx := context.Background()
	runs, _, _ := openDB(t)
	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{{ID: "call", Block: workflow.CallBlock}}

	parent, err := runs.Create(ctx, run.Run{WorkflowID: "p", WorkflowName: "Parent", Definition: def})
	if err != nil {
		t.Fatal(err)
	}
	item := 2
	child, err := runs.Create(ctx, run.Run{WorkflowID: "c", WorkflowName: "Child", Definition: workflow.EmptyDefinition(),
		Parent: &run.Parent{RunID: parent.ID, StepID: "call", Item: &item}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := runs.Get(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Children) != 1 || got.Children[0].ID != child.ID {
		t.Fatalf("children = %+v", got.Children)
	}
	p := got.Children[0].Parent
	if p == nil || p.RunID != parent.ID || p.WorkflowName != "Parent" || p.StepID != "call" || *p.Item != 2 {
		t.Errorf("child's parent = %+v", p)
	}
	if all, _ := runs.List(ctx, run.ListFilter{}); len(all) != 2 {
		t.Errorf("all runs = %d, want 2", len(all))
	}
	if top, _ := runs.List(ctx, run.ListFilter{TopLevel: true}); len(top) != 1 || top[0].ID != parent.ID {
		t.Errorf("top-level runs = %+v", top)
	}

	// A child waiting for approval keeps its parent from being failed.
	now := time.Now().UTC()
	must(t, runs.StartRun(ctx, parent.ID, now))
	must(t, runs.StartStep(ctx, parent.ID, "call", run.StatusRunning, nil, now))
	must(t, runs.StartRun(ctx, child.ID, now))
	_, err = runs.Create(ctx, run.Run{WorkflowID: "x", WorkflowName: "Other", Definition: workflow.EmptyDefinition()})
	if err != nil {
		t.Fatal(err)
	}
	childDef := workflow.EmptyDefinition()
	childDef.Steps = []workflow.Step{{ID: "approve", Block: "approval"}}
	waiting, _ := runs.Create(ctx, run.Run{WorkflowID: "c", WorkflowName: "Child", Definition: childDef,
		Parent: &run.Parent{RunID: parent.ID, StepID: "call"}})
	must(t, runs.StartStep(ctx, waiting.ID, "approve", run.StatusWaiting, nil, now))

	n, err := runs.FailUnfinished(ctx, "stopped", now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 { // the other run and the child that wasn't waiting
		t.Errorf("failed %d runs, want 2", n)
	}
	if got, _ := runs.Get(ctx, parent.ID); got.Status.Done() || got.Steps[0].Status != run.StatusRunning {
		t.Errorf("parent = %s, step %s", got.Status, got.Steps[0].Status)
	}

	must(t, runs.FinishRun(ctx, waiting.ID, run.StatusSucceeded, map[string]any{"v": 1.0}, "", now))
	if got, _ := runs.Get(ctx, waiting.ID); got.Outputs["v"] != 1.0 {
		t.Errorf("outputs = %v", got.Outputs)
	}
}

func TestCallers(t *testing.T) {
	ctx := context.Background()
	_, wfs, _ := openDB(t)
	child, _ := wfs.Create(ctx, workflow.Workflow{Name: "Child", Definition: workflow.EmptyDefinition()})
	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{{ID: "call", Block: workflow.CallBlock, Config: json.RawMessage(`{"workflow":"` + child.ID + `"}`)}}
	def.Outputs = []workflow.Output{{Name: "v", Value: "steps.call.outputs.v"}}
	parent, _ := wfs.Create(ctx, workflow.Workflow{Name: "Parent", Definition: def})
	_, _ = wfs.Create(ctx, workflow.Workflow{Name: "Unrelated", Definition: workflow.EmptyDefinition()})

	callers, err := wfs.Callers(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(callers) != 1 || callers[0].ID != parent.ID {
		t.Errorf("callers = %+v", callers)
	}
	list, _ := wfs.List(ctx)
	for _, w := range list {
		if w.ID == parent.ID && (len(w.Outputs) != 1 || w.Outputs[0].Name != "v" || w.Inputs == nil) {
			t.Errorf("summary = %+v", w)
		}
	}
}

func TestPresets(t *testing.T) {
	ctx := context.Background()
	_, _, presets := openDB(t)
	p, err := presets.Create(ctx, preset.Preset{Name: "GitLab API", Block: "http", Step: preset.Step{
		Config: json.RawMessage(`{"url":"https://gitlab.example.com/api/v4"}`), Timeout: "30s",
		Retry: &workflow.Retry{Max: 2},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := presets.Create(ctx, preset.Preset{Name: "gitlab api", Block: "http"}); !errors.Is(err, preset.ErrNameTaken) {
		t.Errorf("duplicate name: %v", err)
	}

	p.Description = "Calls GitLab"
	p.Step.Config = json.RawMessage(`{"url":"https://gitlab.example.com/api/v4/projects"}`)
	if _, err := presets.Update(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err := presets.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "Calls GitLab" || got.Step.Timeout != "30s" || got.Step.Retry.Max != 2 ||
		string(got.Step.Config) != `{"url":"https://gitlab.example.com/api/v4/projects"}` {
		t.Errorf("got %+v", got)
	}

	must(t, presets.Delete(ctx, p.ID))
	if _, err := presets.Get(ctx, p.ID); !errors.Is(err, preset.ErrNotFound) {
		t.Errorf("after delete: %v", err)
	}
	if list, _ := presets.List(ctx); len(list) != 0 {
		t.Errorf("list = %+v", list)
	}
}
