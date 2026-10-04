package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"conductor/internal/block"
	"conductor/internal/run"
	"conductor/internal/secrets"
	"conductor/internal/store"
	"conductor/internal/workflow"
)

// concurrency tracks how many "sleep" steps run at once.
type concurrency struct{ now, max atomic.Int32 }

func (c *concurrency) enter() {
	n := c.now.Add(1)
	for {
		m := c.max.Load()
		if n <= m || c.max.CompareAndSwap(m, n) {
			return
		}
	}
}

func testBlocks(c *concurrency) *block.Registry {
	sleep := block.Block{Type: "sleep", Run: func(ctx context.Context, cfg json.RawMessage, env block.Env) (map[string]any, error) {
		var v struct{ Ms int }
		_ = json.Unmarshal(cfg, &v)
		c.enter()
		defer c.now.Add(-1)
		select {
		case <-time.After(time.Duration(v.Ms) * time.Millisecond):
			return map[string]any{"slept": v.Ms}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	fail := block.Block{Type: "fail", Run: func(context.Context, json.RawMessage, block.Env) (map[string]any, error) {
		return map[string]any{"partial": true}, errors.New("boom")
	}}
	panicky := block.Block{Type: "panic", Run: func(context.Context, json.RawMessage, block.Env) (map[string]any, error) {
		panic("kaboom")
	}}
	echo := block.Block{Type: "echo", Run: func(_ context.Context, _ json.RawMessage, env block.Env) (map[string]any, error) {
		fmt.Fprint(env.Stdout, "line one\nline two\npartial")
		fmt.Fprintln(env.Stderr, "a warning")
		return nil, nil
	}}
	needs := block.Block{
		Type:   "needs",
		Fields: []block.Field{{Key: "url", Label: "URL", Required: true}},
		Run:    echo.Run,
	}
	// emit returns its config's "value" as an output and logs it.
	emit := block.Block{Type: "emit", Run: func(_ context.Context, cfg json.RawMessage, env block.Env) (map[string]any, error) {
		var v struct{ Value string }
		_ = json.Unmarshal(cfg, &v)
		fmt.Fprintln(env.Stdout, v.Value)
		return map[string]any{"value": v.Value}, nil
	}}
	return block.NewRegistry(sleep, fail, panicky, echo, needs, emit)
}

type fixture struct {
	engine *Engine
	runs   *store.Runs
	conc   *concurrency
}

func newFixture(t *testing.T, maxParallel int) fixture {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"), log)
	if err != nil {
		t.Fatal(err)
	}
	runs := store.NewRuns(db)
	conc := &concurrency{}
	e := New(Options{Runs: runs, Blocks: testBlocks(conc), Log: log, MaxParallel: maxParallel,
		Secrets: fakeSecrets{"TOKEN": "s3cr3t-token"}})
	t.Cleanup(func() {
		_ = e.Shutdown(context.Background())
		db.Close()
	})
	return fixture{engine: e, runs: runs, conc: conc}
}

type stepSpec struct {
	id, block, config, timeout string
}

func build(steps []stepSpec, edges ...string) workflow.Workflow {
	def := workflow.EmptyDefinition()
	for _, s := range steps {
		cfg := s.config
		if cfg == "" {
			cfg = "{}"
		}
		def.Steps = append(def.Steps, workflow.Step{ID: s.id, Block: s.block, Config: json.RawMessage(cfg), Timeout: s.timeout})
	}
	for _, e := range edges {
		from, to, _ := strings.Cut(e, ">")
		def.Edges = append(def.Edges, workflow.Edge{From: from, To: to})
	}
	return workflow.Workflow{ID: "wf", Name: "Test", Definition: def}
}

func (f fixture) runToEnd(t *testing.T, wf workflow.Workflow) (run.Run, time.Duration) {
	t.Helper()
	start := time.Now()
	r, err := f.engine.Start(context.Background(), wf, nil)
	if err != nil {
		t.Fatal(err)
	}
	return f.waitDone(t, r.ID), time.Since(start)
}

func (f fixture) waitDone(t *testing.T, id string) run.Run {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		r, err := f.runs.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status.Done() {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run did not finish")
	return run.Run{}
}

func statuses(r run.Run) map[string]run.Status {
	out := map[string]run.Status{}
	for _, s := range r.Steps {
		out[s.StepID] = s.Status
	}
	return out
}

func step(r run.Run, id string) run.StepRun {
	for _, s := range r.Steps {
		if s.StepID == id {
			return s
		}
	}
	return run.StepRun{}
}

func TestParallelBranches(t *testing.T) {
	f := newFixture(t, 8)
	wf := build([]stepSpec{
		{id: "tag", block: "sleep", config: `{"ms":20}`},
		{id: "confluence", block: "sleep", config: `{"ms":300}`},
		{id: "jira", block: "sleep", config: `{"ms":300}`},
		{id: "email", block: "sleep", config: `{"ms":20}`},
	}, "tag>confluence", "tag>jira", "confluence>email", "jira>email")

	r, took := f.runToEnd(t, wf)

	if r.Status != run.StatusSucceeded {
		t.Fatalf("run %s: %s", r.Status, r.Error)
	}
	if took > 550*time.Millisecond {
		t.Errorf("took %s; branches did not run in parallel", took)
	}
	if got := f.conc.max.Load(); got != 2 {
		t.Errorf("max concurrency = %d, want 2", got)
	}
	email, jira := step(r, "email"), step(r, "jira")
	if email.StartedAt.Before(*jira.FinishedAt) {
		t.Error("email started before its dependency jira finished")
	}
	if string(step(r, "tag").Outputs) != `{"slept":20}` {
		t.Errorf("outputs = %s", step(r, "tag").Outputs)
	}
}

func TestMaxParallelLimitsConcurrency(t *testing.T) {
	f := newFixture(t, 1)
	wf := build([]stepSpec{
		{id: "a", block: "sleep", config: `{"ms":100}`},
		{id: "b", block: "sleep", config: `{"ms":100}`},
		{id: "c", block: "sleep", config: `{"ms":100}`},
	})
	r, took := f.runToEnd(t, wf)
	if r.Status != run.StatusSucceeded {
		t.Fatalf("run %s", r.Status)
	}
	if f.conc.max.Load() != 1 || took < 300*time.Millisecond {
		t.Errorf("max concurrency %d in %s, want 1 and ≥300ms", f.conc.max.Load(), took)
	}
}

func TestFailureSkipsDownstreamOnly(t *testing.T) {
	f := newFixture(t, 8)
	wf := build([]stepSpec{
		{id: "push", block: "fail"},
		{id: "pipeline", block: "sleep"},
		{id: "notify", block: "sleep"},
		{id: "lint", block: "sleep", config: `{"ms":50}`},
	}, "push>pipeline", "pipeline>notify")

	r, _ := f.runToEnd(t, wf)

	want := map[string]run.Status{
		"push": run.StatusFailed, "pipeline": run.StatusSkipped,
		"notify": run.StatusSkipped, "lint": run.StatusSucceeded,
	}
	if got := statuses(r); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("statuses = %v, want %v", got, want)
	}
	if r.Status != run.StatusFailed {
		t.Errorf("run status = %s", r.Status)
	}
	push := step(r, "push")
	if push.Error != "boom" || string(push.Outputs) != `{"partial":true}` {
		t.Errorf("push = %+v", push)
	}
	if !strings.Contains(step(r, "notify").Error, `"push" failed`) {
		t.Errorf("skip reason = %q", step(r, "notify").Error)
	}
}

func TestCancel(t *testing.T) {
	f := newFixture(t, 8)
	wf := build([]stepSpec{
		{id: "slow", block: "sleep", config: `{"ms":10000}`},
		{id: "after", block: "sleep"},
	}, "slow>after")

	r, err := f.engine.Start(context.Background(), wf, nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := f.engine.Cancel(r.ID); err != nil {
		t.Fatal(err)
	}
	done := f.waitDone(t, r.ID)

	if done.Status != run.StatusCancelled || done.Error != "" {
		t.Errorf("run = %s %q", done.Status, done.Error)
	}
	if got := statuses(done); got["slow"] != run.StatusCancelled || got["after"] != run.StatusCancelled {
		t.Errorf("statuses = %v", got)
	}
	if err := f.engine.Cancel(r.ID); !errors.Is(err, ErrNotActive) {
		t.Errorf("second cancel: %v", err)
	}
}

func TestStepTimeout(t *testing.T) {
	f := newFixture(t, 8)
	wf := build([]stepSpec{{id: "slow", block: "sleep", config: `{"ms":5000}`, timeout: "100ms"}})
	r, took := f.runToEnd(t, wf)
	if r.Status != run.StatusFailed || step(r, "slow").Error != "timed out after 100ms" {
		t.Errorf("run %s, step error %q", r.Status, step(r, "slow").Error)
	}
	if took > time.Second {
		t.Errorf("timeout took %s", took)
	}
}

func TestPanicFailsStep(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{{id: "p", block: "panic"}}))
	if r.Status != run.StatusFailed || !strings.Contains(step(r, "p").Error, "kaboom") {
		t.Errorf("run %s, step error %q", r.Status, step(r, "p").Error)
	}
}

func TestLogs(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{{id: "say", block: "echo"}}))

	lines, err := f.runs.Logs(context.Background(), r.ID, "say", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range lines {
		got = append(got, string(l.Stream)+":"+l.Text)
	}
	want := []string{"stdout:line one", "stdout:line two", "stderr:a warning", "stdout:partial"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("logs = %v, want %v", got, want)
	}
}

func TestCheckReportsMissingConfig(t *testing.T) {
	f := newFixture(t, 8)
	_, err := f.engine.Start(context.Background(), build([]stepSpec{{id: "call", block: "needs"}}), nil)
	var verr *workflow.ValidationError
	if !errors.As(err, &verr) || verr.Problems[0].Step != "call" || !strings.Contains(verr.Problems[0].Message, "URL") {
		t.Fatalf("err = %v", err)
	}

	_, err = f.engine.Start(context.Background(), build(nil), nil)
	if !errors.As(err, &verr) || !strings.Contains(verr.Problems[0].Message, "at least one step") {
		t.Fatalf("empty workflow: err = %v", err)
	}
}

func TestShutdownCancelsRuns(t *testing.T) {
	f := newFixture(t, 8)
	r, err := f.engine.Start(context.Background(), build([]stepSpec{{id: "slow", block: "sleep", config: `{"ms":10000}`}}), nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := f.engine.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := f.runs.Get(context.Background(), r.ID)
	if got.Status != run.StatusCancelled || !strings.Contains(got.Error, "stopped") {
		t.Errorf("run = %s %q", got.Status, got.Error)
	}
	if _, err := f.engine.Start(context.Background(), build([]stepSpec{{id: "a", block: "echo"}}), nil); err == nil {
		t.Error("Start after Shutdown succeeded")
	}
}

type recorder struct {
	mu     sync.Mutex
	events []run.Event
	ended  bool
}

func (r *recorder) Publish(_ string, e run.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) End(string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ended = true
}

func TestPublishesEvents(t *testing.T) {
	f := newFixture(t, 8)
	rec := &recorder{}
	f.engine.events = rec

	r, _ := f.runToEnd(t, build([]stepSpec{
		{id: "say", block: "echo"},
		{id: "boom", block: "fail"},
		{id: "after", block: "echo"},
	}, "boom>after"))
	_ = f.engine.Shutdown(context.Background()) // wait for the run goroutine to return

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if !rec.ended {
		t.Fatal("End was not called")
	}

	var trail []string
	logLines := 0
	for _, e := range rec.events {
		switch e.Type {
		case run.EventRun:
			trail = append(trail, "run:"+string(e.State.Status))
		case run.EventStep:
			trail = append(trail, e.Step.StepID+":"+string(e.Step.Status))
		case run.EventLogs:
			for _, l := range e.Lines {
				if l.Seq == 0 {
					t.Errorf("log line without seq: %+v", l)
				}
			}
			logLines += len(e.Lines)
		}
	}
	joined := strings.Join(trail, " ")
	for _, want := range []string{"run:running", "say:running", "say:succeeded", "boom:failed", "after:skipped", "run:failed"} {
		if !strings.Contains(joined, want) {
			t.Errorf("events %q missing %q", joined, want)
		}
	}
	if !strings.HasPrefix(joined, "run:running") || !strings.HasSuffix(joined, "run:"+string(r.Status)) {
		t.Errorf("run events out of order: %q", joined)
	}
	stored, _ := f.runs.Logs(context.Background(), r.ID, "", 0, 0)
	if logLines != len(stored) {
		t.Errorf("published %d log lines, stored %d", logLines, len(stored))
	}
	last := rec.events[len(rec.events)-1]
	if last.Type != run.EventRun {
		t.Errorf("last event = %s, want the final run state after all logs", last.Type)
	}
}

type fakeSecrets map[string]string

func (f fakeSecrets) Values(_ context.Context, names []string) (map[string]string, error) {
	out := map[string]string{}
	var missing []string
	for _, n := range names {
		v, ok := f[n]
		if !ok {
			missing = append(missing, n)
			continue
		}
		out[n] = v
	}
	if len(missing) > 0 {
		return nil, &secrets.MissingError{Names: missing}
	}
	return out, nil
}

func TestDataFlow(t *testing.T) {
	f := newFixture(t, 8)
	wf := build([]stepSpec{
		{id: "tag", block: "emit", config: `{"value":"{{ inputs.version }}"}`},
		{id: "notes", block: "emit", config: `{"value":"release {{ steps.tag.outputs.value }} by {{ inputs.who }} using {{ secrets.TOKEN }}"}`},
	}, "tag>notes")
	wf.Definition.Inputs = []workflow.Input{
		{Name: "version", Type: workflow.InputText, Required: true},
		{Name: "who", Type: workflow.InputText, Default: "conductor"},
	}

	if _, err := f.engine.Start(context.Background(), wf, nil); err == nil || !strings.Contains(err.Error(), `"version" is required`) {
		t.Fatalf("missing required input: err = %v", err)
	}

	r, err := f.engine.Start(context.Background(), wf, map[string]any{"version": "v1.4.0"})
	if err != nil {
		t.Fatal(err)
	}
	done := f.waitDone(t, r.ID)
	if done.Status != run.StatusSucceeded {
		t.Fatalf("run %s: %+v", done.Status, done.Steps)
	}
	if done.Inputs["version"] != "v1.4.0" || done.Inputs["who"] != "conductor" {
		t.Errorf("recorded inputs = %v", done.Inputs)
	}

	notes := step(done, "notes")
	want := "release v1.4.0 by conductor using " + secrets.Mask
	if !strings.Contains(string(notes.Outputs), want) || !strings.Contains(string(notes.Config), want) {
		t.Errorf("outputs = %s\nconfig = %s\nwant both to contain %q", notes.Outputs, notes.Config, want)
	}
	lines, _ := f.runs.Logs(context.Background(), r.ID, "notes", 0, 0)
	for _, l := range lines {
		if strings.Contains(l.Text, "s3cr3t") {
			t.Errorf("secret leaked into logs: %q", l.Text)
		}
	}
}

func TestMissingSecretIsReported(t *testing.T) {
	f := newFixture(t, 8)
	_, err := f.engine.Start(context.Background(), build([]stepSpec{
		{id: "call", block: "emit", config: `{"value":"{{ secrets.JIRA_TOKEN }}"}`},
	}), nil)
	var verr *workflow.ValidationError
	if !errors.As(err, &verr) || verr.Problems[0].Step != "call" || !strings.Contains(verr.Problems[0].Message, "JIRA_TOKEN") {
		t.Fatalf("err = %v", err)
	}
}

func TestRenderErrorFailsStep(t *testing.T) {
	f := newFixture(t, 8)
	r, _ := f.runToEnd(t, build([]stepSpec{
		{id: "a", block: "emit", config: `{"value":"x"}`},
		{id: "b", block: "emit", config: `{"value":"{{ steps.a.outputs.vaule }}"}`},
	}, "a>b"))
	b := step(r, "b")
	if b.Status != run.StatusFailed || !strings.HasPrefix(b.Error, "config: value:") {
		t.Errorf("b = %s %q", b.Status, b.Error)
	}
}
