package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"conductor/internal/run"
	"conductor/internal/workflow"
)

func openRuns(t *testing.T) *Runs {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewRuns(db)
}

func TestRunLifecycle(t *testing.T) {
	ctx := context.Background()
	s := openRuns(t)

	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{{ID: "tag", Block: "shell"}, {ID: "push", Block: "shell"}}

	r, err := s.Create(ctx, run.Run{WorkflowID: "wf1", WorkflowName: "Build prep", Definition: def,
		Inputs: map[string]any{"version": "v1.4.0"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != run.StatusQueued || len(r.Steps) != 2 {
		t.Fatalf("create returned %+v", r)
	}

	now := time.Now().UTC()
	must(t, s.StartRun(ctx, r.ID, now))
	must(t, s.StartStep(ctx, r.ID, "tag", run.StatusRunning, json.RawMessage(`{"command":"git tag v1.4.0"}`), now))
	must(t, s.UpdateStep(ctx, r.ID, "tag", run.Progress{Attempt: 2, Items: 3, ItemsDone: 1}))
	item := 1
	appended, err := s.AppendLogs(ctx, r.ID, []run.LogLine{
		{StepID: "tag", Stream: run.StreamStdout, Text: "tagged", Time: now},
		{StepID: "tag", Stream: run.StreamStderr, Text: "warning", Time: now, Item: &item},
	})
	must(t, err)
	if appended[0].Seq == 0 || appended[1].Seq <= appended[0].Seq {
		t.Errorf("appended seqs = %d, %d", appended[0].Seq, appended[1].Seq)
	}
	must(t, s.FinishStep(ctx, r.ID, "tag", run.StatusSucceeded, json.RawMessage(`{"exitCode":0}`), "", now))
	must(t, s.FinishRun(ctx, r.ID, run.StatusSucceeded, map[string]any{"version": "1.4.0"}, "", now))

	got, err := s.Get(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != run.StatusSucceeded || got.StartedAt == nil || got.FinishedAt == nil {
		t.Errorf("run = %+v", got)
	}
	if got.Steps[0].Status != run.StatusSucceeded || string(got.Steps[0].Outputs) != `{"exitCode":0}` {
		t.Errorf("step = %+v", got.Steps[0])
	}
	if got.Inputs["version"] != "v1.4.0" || string(got.Steps[0].Config) != `{"command":"git tag v1.4.0"}` {
		t.Errorf("inputs/config not stored: %v %s", got.Inputs, got.Steps[0].Config)
	}
	if got.Steps[0].Progress != (run.Progress{Attempt: 2, Items: 3, ItemsDone: 1}) {
		t.Errorf("progress = %+v", got.Steps[0].Progress)
	}
	if got.Steps[1].Status != run.StatusQueued || got.Steps[1].StartedAt != nil {
		t.Errorf("untouched step = %+v", got.Steps[1])
	}

	logs, err := s.Logs(ctx, r.ID, "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].Text != "tagged" {
		t.Fatalf("logs = %+v", logs)
	}
	after, _ := s.Logs(ctx, r.ID, "", logs[0].Seq, 0)
	if len(after) != 1 || after[0].Text != "warning" || after[0].Item == nil || *after[0].Item != 1 || logs[0].Item != nil {
		t.Errorf("logs after cursor = %+v", after)
	}

	list, _ := s.List(ctx, run.ListFilter{WorkflowID: "wf1"})
	if len(list) != 1 || list[0].StepCount != 2 {
		t.Errorf("list = %+v", list)
	}
	if page, _ := s.List(ctx, run.ListFilter{WorkflowID: "wf1", Offset: 1}); len(page) != 0 {
		t.Errorf("list past the end = %+v", page)
	}
	scratch, err := s.Create(ctx, run.Run{WorkflowName: "Scratch", Definition: def})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s.List(ctx, run.ListFilter{Scratch: true}); len(got) != 1 || got[0].ID != scratch.ID {
		t.Errorf("scratch list = %+v", got)
	}
	if _, err := s.Get(ctx, "missing"); !errors.Is(err, run.ErrNotFound) {
		t.Errorf("get missing: %v", err)
	}
}

func TestFailUnfinished(t *testing.T) {
	ctx := context.Background()
	s := openRuns(t)

	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{{ID: "a", Block: "shell"}, {ID: "b", Block: "shell"}}
	crashed, _ := s.Create(ctx, run.Run{WorkflowID: "wf", Definition: def})
	must(t, s.StartRun(ctx, crashed.ID, time.Now()))
	must(t, s.StartStep(ctx, crashed.ID, "a", run.StatusRunning, nil, time.Now()))

	// A run waiting for approval survives, to be resumed.
	waiting, _ := s.Create(ctx, run.Run{WorkflowID: "wf", Definition: def})
	must(t, s.StartRun(ctx, waiting.ID, time.Now()))
	must(t, s.StartStep(ctx, waiting.ID, "a", run.StatusWaiting, nil, time.Now()))
	must(t, s.SetRunStatus(ctx, waiting.ID, run.StatusWaiting))

	n, err := s.FailUnfinished(ctx, "Conductor stopped", time.Now())
	if err != nil || n != 1 {
		t.Fatalf("FailUnfinished = %d, %v", n, err)
	}
	got, _ := s.Get(ctx, crashed.ID)
	if got.Status != run.StatusFailed || got.Steps[0].Status != run.StatusFailed ||
		got.Steps[1].Status != run.StatusFailed || got.Error != "Conductor stopped" {
		t.Errorf("crashed run = %+v", got)
	}
	got, _ = s.Get(ctx, waiting.ID)
	if got.Status != run.StatusWaiting || got.Steps[0].Status != run.StatusWaiting || got.Steps[1].Status != run.StatusQueued {
		t.Errorf("waiting run = %+v", got)
	}

	ids, err := s.Unfinished(ctx)
	if err != nil || len(ids) != 1 || ids[0] != waiting.ID {
		t.Errorf("Unfinished = %v, %v", ids, err)
	}
	list, _ := s.List(ctx, run.ListFilter{Status: run.StatusWaiting})
	if len(list) != 1 || list[0].ID != waiting.ID {
		t.Errorf("list waiting = %+v", list)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
