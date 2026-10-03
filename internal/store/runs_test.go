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

	r, err := s.Create(ctx, run.Run{WorkflowID: "wf1", WorkflowName: "Build prep", Definition: def})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != run.StatusQueued || len(r.Steps) != 2 {
		t.Fatalf("create returned %+v", r)
	}

	now := time.Now().UTC()
	must(t, s.StartRun(ctx, r.ID, now))
	must(t, s.StartStep(ctx, r.ID, "tag", now))
	must(t, s.AppendLogs(ctx, r.ID, []run.LogLine{
		{StepID: "tag", Stream: run.StreamStdout, Text: "tagged", Time: now},
		{StepID: "tag", Stream: run.StreamStderr, Text: "warning", Time: now},
	}))
	must(t, s.FinishStep(ctx, r.ID, "tag", run.StatusSucceeded, json.RawMessage(`{"exitCode":0}`), "", now))
	must(t, s.FinishRun(ctx, r.ID, run.StatusSucceeded, "", now))

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
	if len(after) != 1 || after[0].Text != "warning" {
		t.Errorf("logs after cursor = %+v", after)
	}

	list, _ := s.List(ctx, run.ListFilter{WorkflowID: "wf1"})
	if len(list) != 1 || list[0].StepCount != 2 {
		t.Errorf("list = %+v", list)
	}
	if _, err := s.Get(ctx, "missing"); !errors.Is(err, run.ErrNotFound) {
		t.Errorf("get missing: %v", err)
	}
}

func TestFailUnfinished(t *testing.T) {
	ctx := context.Background()
	s := openRuns(t)

	def := workflow.EmptyDefinition()
	def.Steps = []workflow.Step{{ID: "a", Block: "shell"}}
	r, _ := s.Create(ctx, run.Run{WorkflowID: "wf", Definition: def})
	must(t, s.StartRun(ctx, r.ID, time.Now()))

	n, err := s.FailUnfinished(ctx, "Conductor stopped", time.Now())
	if err != nil || n != 1 {
		t.Fatalf("FailUnfinished = %d, %v", n, err)
	}
	got, _ := s.Get(ctx, r.ID)
	if got.Status != run.StatusFailed || got.Steps[0].Status != run.StatusFailed || got.Error != "Conductor stopped" {
		t.Errorf("run = %+v", got)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
