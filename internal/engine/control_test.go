package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"conductor/internal/run"
	"conductor/internal/workflow"
)

func when(cond string) func(*workflow.Step) {
	return func(s *workflow.Step) { s.If = cond }
}

func TestConditionSkipsBranchButNotJoin(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{
		{id: "check", block: "emit", config: `{"value":"red"}`},
		{id: "deploy", block: "emit", edit: when(`steps.check.outputs.value == "green"`)},
		{id: "announce", block: "emit"},
		{id: "alert", block: "emit", config: `{"value":"{{ steps.check.status }}"}`, edit: when(`steps.check.outputs.value != "green"`)},
		{id: "report", block: "emit", config: `{"value":"deploy was {{ steps.deploy.status }}"}`},
	}, "check>deploy", "deploy>announce", "check>alert", "announce>report", "alert>report"))

	want := map[string]run.Status{
		"check": run.StatusSucceeded, "deploy": run.StatusSkipped, "announce": run.StatusSkipped,
		"alert": run.StatusSucceeded, "report": run.StatusSucceeded,
	}
	for id, st := range want {
		if got := step(r, id).Status; got != st {
			t.Errorf("%s = %s, want %s (%s)", id, got, st, step(r, id).Error)
		}
	}
	if r.Status != run.StatusSucceeded {
		t.Errorf("run = %s, want succeeded", r.Status)
	}
	if e := step(r, "deploy").Error; !strings.Contains(e, "condition is false") {
		t.Errorf("deploy reason = %q", e)
	}
	if e := step(r, "announce").Error; e != `skipped because "deploy" was skipped` {
		t.Errorf("announce reason = %q", e)
	}
	if out := string(step(r, "report").Outputs); out != `{"value":"deploy was skipped"}` {
		t.Errorf("report outputs = %s", out)
	}
	if out := string(step(r, "alert").Outputs); out != `{"value":"succeeded"}` {
		t.Errorf("alert outputs = %s", out)
	}
}

func TestConditionMustBeBoolean(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{
		{id: "a", block: "emit", edit: when(`"yes"`)},
	}))
	if st := step(r, "a"); st.Status != run.StatusFailed || !strings.Contains(st.Error, "must be true or false") {
		t.Errorf("a = %s %q", st.Status, st.Error)
	}
}

func TestContinueOnError(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{
		{id: "lint", block: "fail", edit: func(s *workflow.Step) { s.ContinueOnError = true }},
		{id: "notify", block: "emit", config: `{"value":"lint {{ steps.lint.status }}"}`},
	}, "lint>notify"))
	if r.Status != run.StatusSucceeded {
		t.Errorf("run = %s, want succeeded", r.Status)
	}
	if got := statuses(r); got["lint"] != run.StatusFailed || got["notify"] != run.StatusSucceeded {
		t.Errorf("statuses = %v", got)
	}
	if out := string(step(r, "notify").Outputs); out != `{"value":"lint failed"}` {
		t.Errorf("notify outputs = %s", out)
	}
}

func retry(max int) func(*workflow.Step) {
	return func(s *workflow.Step) { s.Retry = &workflow.Retry{Max: max, Delay: "10ms"} }
}

func TestRetry(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{
		{id: "eventually", block: "flaky", config: `{"fails":2}`, edit: retry(3)},
		{id: "never", block: "flaky", config: `{"fails":5}`, edit: retry(1)},
	}))
	if st := step(r, "eventually"); st.Status != run.StatusSucceeded || st.Attempt != 3 {
		t.Errorf("eventually = %s after %d attempts", st.Status, st.Attempt)
	}
	if st := step(r, "never"); st.Status != run.StatusFailed || st.Attempt != 2 || st.Error != "flake 2" {
		t.Errorf("never = %s %q after %d attempts", st.Status, st.Error, st.Attempt)
	}
	logs, _ := f.runs.Logs(context.Background(), r.ID, "eventually", 0, 0)
	if text := logText(logs); !strings.Contains(text, "Attempt 1 of 4 failed: flake 1") || !strings.Contains(text, "Retrying in 10ms") {
		t.Errorf("logs = %s", text)
	}
}

func TestRetryWaitIsCancellable(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.engine.Start(context.Background(), build([]stepSpec{{
		id: "poll", block: "flaky", config: `{"fails":9}`,
		edit: func(s *workflow.Step) { s.Retry = &workflow.Retry{Max: 5, Delay: "1h"} },
	}}), nil)
	f.waitFor(t, r.ID, func(r run.Run) bool { return step(r, "poll").Status == run.StatusRunning })
	time.Sleep(20 * time.Millisecond)
	_ = f.engine.Cancel(r.ID)
	if got := f.waitDone(t, r.ID); got.Status != run.StatusCancelled {
		t.Errorf("run = %s", got.Status)
	}
}

func forEach(items string, concurrency int) func(*workflow.Step) {
	return func(s *workflow.Step) { s.ForEach = &workflow.ForEach{Items: items, Concurrency: concurrency} }
}

func TestForEach(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{
		{id: "list", block: "emit", config: `{"value":"KEY-1,KEY-2,KEY-3"}`},
		{id: "update", block: "emit", config: `{"value":"{{ index }}:{{ item }}"}`,
			edit: forEach(`split(steps.list.outputs.value, ",")`, 1)},
		{id: "summary", block: "emit", config: `{"value":"{{ steps.update.outputs.results[2].value }}"}`},
	}, "list>update", "update>summary"))

	if r.Status != run.StatusSucceeded {
		t.Fatalf("run = %s: %v", r.Status, step(r, "update").Error)
	}
	up := step(r, "update")
	if up.Items != 3 || up.ItemsDone != 3 {
		t.Errorf("progress = %+v", up.Progress)
	}
	if string(up.Config) != `[{"value":"0:KEY-1"},{"value":"1:KEY-2"},{"value":"2:KEY-3"}]` {
		t.Errorf("config = %s", up.Config)
	}
	if string(step(r, "summary").Outputs) != `{"value":"2:KEY-3"}` {
		t.Errorf("summary = %s", step(r, "summary").Outputs)
	}

	logs, _ := f.runs.Logs(context.Background(), r.ID, "update", 0, 0)
	var items []int
	for _, l := range logs {
		if l.Stream == run.StreamStdout {
			items = append(items, *l.Item)
		}
	}
	if len(items) != 3 || items[0] != 0 || items[2] != 2 {
		t.Errorf("stdout lines by item = %v", items)
	}
}

func TestForEachConcurrencyAndFailFast(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{
		{id: "naps", block: "sleep", config: `{"ms":40}`, edit: forEach(`[1, 2, 3, 4]`, 2)},
	}))
	if r.Status != run.StatusSucceeded || f.conc.max.Load() != 2 {
		t.Errorf("run = %s, max concurrent items = %d, want 2", r.Status, f.conc.max.Load())
	}

	r, _ = f.runToEnd(t, build([]stepSpec{
		{id: "each", block: "emit", config: `{"value":"{{ item }}"}`, edit: forEach(`["ok", "bad", "never"]`, 1)},
	}))
	st := step(r, "each")
	if st.Status != run.StatusFailed || st.Error != "item 2 of 3: bad value" || st.ItemsDone != 1 {
		t.Errorf("each = %s %q, %d done", st.Status, st.Error, st.ItemsDone)
	}
	var outputs struct{ Results []any }
	_ = json.Unmarshal(st.Outputs, &outputs)
	if len(outputs.Results) != 3 || outputs.Results[2] != nil {
		t.Errorf("results = %v; the third item should never have run", outputs.Results)
	}
}

func TestForEachNeedsAList(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{
		{id: "each", block: "emit", edit: forEach(`"KEY-1"`, 1)},
		{id: "none", block: "emit", edit: forEach(`[]`, 1)},
	}))
	if st := step(r, "each"); st.Status != run.StatusFailed || !strings.Contains(st.Error, "must be a list, but is KEY-1") {
		t.Errorf("each = %s %q", st.Status, st.Error)
	}
	if st := step(r, "none"); st.Status != run.StatusSucceeded || string(st.Outputs) != `{"results":[]}` {
		t.Errorf("none = %s %s", st.Status, st.Outputs)
	}
}

func approvalFlow() workflow.Workflow {
	return build([]stepSpec{
		{id: "tag", block: "emit", config: `{"value":"v1.4.0"}`},
		{id: "approve", block: "approval", config: `{"message":"Push {{ steps.tag.outputs.value }}?"}`},
		{id: "push", block: "emit", config: `{"value":"pushed {{ steps.tag.outputs.value }} ({{ steps.approve.outputs.comment }})"}`},
	}, "tag>approve", "approve>push")
}

func waitForApproval(t *testing.T, f fixture, id string) run.Run {
	t.Helper()
	return f.waitFor(t, id, func(r run.Run) bool { return r.Status == run.StatusWaiting })
}

func TestApprovalApproved(t *testing.T) {
	f := newFixture(t, 8)
	r, err := f.engine.Start(context.Background(), approvalFlow(), nil)
	if err != nil {
		t.Fatal(err)
	}
	waiting := waitForApproval(t, f, r.ID)
	if st := step(waiting, "approve"); st.Status != run.StatusWaiting || string(st.Config) != `{"message":"Push v1.4.0?"}` {
		t.Errorf("approve = %s %s", st.Status, st.Config)
	}

	if err := f.engine.Decide(r.ID, "push", Decision{Approve: true}); !errors.Is(err, ErrNotWaiting) {
		t.Errorf("deciding a step that isn't waiting: %v", err)
	}
	if err := f.engine.Decide(r.ID, "approve", Decision{Approve: true, Comment: "ship it"}); err != nil {
		t.Fatal(err)
	}
	if err := f.engine.Decide(r.ID, "approve", Decision{Approve: false}); err == nil {
		t.Error("a second decision was accepted")
	}

	got := f.waitDone(t, r.ID)
	if got.Status != run.StatusSucceeded {
		t.Errorf("run = %s", got.Status)
	}
	if out := string(step(got, "push").Outputs); out != `{"value":"pushed v1.4.0 (ship it)"}` {
		t.Errorf("push outputs = %s", out)
	}
}

func TestApprovalRejected(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.engine.Start(context.Background(), approvalFlow(), nil)
	waitForApproval(t, f, r.ID)
	if err := f.engine.Decide(r.ID, "approve", Decision{Comment: "not today"}); err != nil {
		t.Fatal(err)
	}
	got := f.waitDone(t, r.ID)
	if got.Status != run.StatusFailed || step(got, "approve").Error != "Rejected: not today" ||
		step(got, "push").Status != run.StatusSkipped {
		t.Errorf("run = %s, steps %v, approve error %q", got.Status, statuses(got), step(got, "approve").Error)
	}
}

func TestApprovalTimeoutAndCancel(t *testing.T) {
	f := newFixture(t, 8)
	wf := approvalFlow()
	wf.Definition.Steps[1].Timeout = "50ms"
	r, _ := f.runToEnd(t, wf)
	if st := step(r, "approve"); st.Status != run.StatusFailed || st.Error != "no decision within 50ms" {
		t.Errorf("approve = %s %q", st.Status, st.Error)
	}

	r, _ = f.engine.Start(context.Background(), approvalFlow(), nil)
	waitForApproval(t, f, r.ID)
	_ = f.engine.Cancel(r.ID)
	if got := f.waitDone(t, r.ID); got.Status != run.StatusCancelled || step(got, "approve").Status != run.StatusCancelled {
		t.Errorf("run = %s, steps %v", got.Status, statuses(got))
	}
}

func TestApprovalSurvivesRestart(t *testing.T) {
	f := newFixture(t, 8)
	wf := approvalFlow()
	// A slow parallel branch is interrupted by the restart.
	wf.Definition.Steps = append(wf.Definition.Steps, workflow.Step{ID: "slow", Block: "sleep", Config: json.RawMessage(`{"ms":10000}`)})

	r, err := f.engine.Start(context.Background(), wf, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.waitFor(t, r.ID, func(r run.Run) bool {
		return step(r, "approve").Status == run.StatusWaiting && step(r, "slow").Status == run.StatusRunning
	})
	if err := f.engine.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	stopped, _ := f.runs.Get(context.Background(), r.ID)
	if stopped.Status.Done() || step(stopped, "approve").Status != run.StatusWaiting ||
		step(stopped, "push").Status != run.StatusQueued || step(stopped, "slow").Status != run.StatusCancelled {
		t.Fatalf("after shutdown: run %s, steps %v", stopped.Status, statuses(stopped))
	}

	restarted := f.newEngine(t, 8)
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	decide(t, restarted, r.ID, Decision{Approve: true, Comment: "after restart"})
	got := f.waitDone(t, r.ID)
	if out := string(step(got, "push").Outputs); out != `{"value":"pushed v1.4.0 (after restart)"}` {
		t.Errorf("push outputs = %s", out)
	}
	// Everything that could finish did, but one branch was interrupted.
	if got.Status != run.StatusCancelled || step(got, "push").Status != run.StatusSucceeded {
		t.Errorf("run = %s, steps %v", got.Status, statuses(got))
	}
}

func TestRecoverAfterCrash(t *testing.T) {
	f := newFixture(t, 8)
	ctx := context.Background()
	// Leave a run as a crash would: one approval waiting, one step mid-run.
	wf := approvalFlow()
	wf.Definition.Steps = append(wf.Definition.Steps, workflow.Step{ID: "slow", Block: "sleep", Config: json.RawMessage(`{}`)},
		workflow.Step{ID: "after_slow", Block: "emit"})
	wf.Definition.Edges = append(wf.Definition.Edges, workflow.Edge{From: "slow", To: "after_slow"})
	r, _ := f.runs.Create(ctx, run.Run{WorkflowID: wf.ID, WorkflowName: wf.Name, Definition: wf.Definition})
	now := time.Now().UTC()
	must(t, f.runs.StartRun(ctx, r.ID, now))
	must(t, f.runs.StartStep(ctx, r.ID, "tag", run.StatusRunning, nil, now))
	must(t, f.runs.FinishStep(ctx, r.ID, "tag", run.StatusSucceeded, json.RawMessage(`{"value":"v2"}`), "", now))
	must(t, f.runs.StartStep(ctx, r.ID, "approve", run.StatusWaiting, json.RawMessage(`{"message":"Push v2?"}`), now))
	must(t, f.runs.StartStep(ctx, r.ID, "slow", run.StatusRunning, nil, now))

	if err := f.engine.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	got := f.waitFor(t, r.ID, func(r run.Run) bool { return step(r, "after_slow").Status.Done() })
	if st := step(got, "slow"); st.Status != run.StatusCancelled || !strings.Contains(st.Error, "Conductor stopped") {
		t.Errorf("slow = %s %q", st.Status, st.Error)
	}
	if step(got, "after_slow").Status != run.StatusCancelled || step(got, "approve").Status != run.StatusWaiting {
		t.Errorf("steps = %v", statuses(got))
	}
	decide(t, f.engine, r.ID, Decision{Approve: true})
	if out := string(step(f.waitDone(t, r.ID), "push").Outputs); !strings.Contains(out, "pushed v2") {
		t.Errorf("push outputs = %s", out)
	}
}

func TestCheckRejectsLoopingApproval(t *testing.T) {
	f := newFixture(t, 8)
	err := f.engine.Check(context.Background(), build([]stepSpec{{id: "ok", block: "approval", edit: retry(2)}}))
	if err == nil || !strings.Contains(err.Error(), "approval steps can't retry") {
		t.Errorf("Check = %v", err)
	}
}

// decide approves or rejects step "approve" once the run is waiting for it;
// a resumed run takes a moment to start waiting again.
func decide(t *testing.T, e *Engine, runID string, d Decision) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := e.Decide(runID, "approve", d)
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Decide: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func logText(lines []run.LogLine) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.Text + "\n")
	}
	return b.String()
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
