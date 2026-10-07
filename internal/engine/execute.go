package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"conductor/internal/run"
	"conductor/internal/secrets"
	"conductor/internal/tmpl"
	"conductor/internal/workflow"
)

type stepResult struct {
	id        string
	status    run.Status
	outputs   map[string]any
	err       string
	config    json.RawMessage // resolved and masked, as recorded
	startedAt *time.Time
	progress  run.Progress
	// waits marks a suspended step that waits for approval, itself or in a
	// run it started; such a step keeps the run alive across a restart.
	waits bool
	// suspended means Conductor is stopping and the step was left as the
	// store has it (waiting for approval, or queued) to resume after restart.
	suspended bool
}

// waitingUpdate says whether a step that runs a workflow is now waiting
// for an approval in that run.
type waitingUpdate struct {
	step    string
	waiting bool
}

// outcome is what downstream steps can see of a finished step.
type outcome struct {
	status  run.Status
	outputs map[string]any
}

// runner executes one run. Its maps are only touched by the goroutine
// running loop; steps run in their own goroutines and report on results.
type runner struct {
	e      *Engine
	ctx    context.Context // cancelled by Cancel or Shutdown
	store  context.Context // for bookkeeping writes, which must outlive ctx
	r      run.Run
	log    *slog.Logger
	masker *secrets.Masker
	// masked lists the values masker hides, for runs this one starts.
	masked []string
	sink   *logSink
	// secretEnv holds decrypted values for expressions; never recorded.
	secretEnv map[string]any
	// lineage lists the workflow ids of this run and the runs above it,
	// to stop a workflow from running itself.
	lineage []string

	steps      map[string]workflow.Step
	parents    map[string][]string
	dependents map[string][]string
	waitingOn  map[string]int
	done       map[string]outcome
	started    map[string]bool
	// stored is the state of each step when a run is resumed; nil otherwise.
	stored map[string]run.StepRun

	results   chan stepResult
	updates   chan waitingUpdate
	slots     chan struct{}
	inFlight  int             // started steps whose result hasn't arrived
	waiting   map[string]bool // those among them waiting for approval
	suspended int             // steps left waiting by a shutdown

	status    run.Status
	startedAt time.Time
	outputs   map[string]any // the workflow's outputs, recorded and masked
}

// execute runs r to completion. Steps start as soon as everything they
// depend on has finished, at most maxParallel doing work at a time. When a
// step fails, everything downstream of it is skipped; independent branches
// carry on. With resuming set, r was loaded from the store after a restart
// and carries on from where it was left.
func (e *Engine) execute(ctx context.Context, l launch) {
	r, resuming := l.run, l.resuming
	store := context.WithoutCancel(ctx)
	log := e.log.With("run", r.ID)

	rn := &runner{
		e: e, ctx: ctx, store: store, r: r, log: log,
		secretEnv:  make(map[string]any, len(l.secrets)),
		steps:      make(map[string]workflow.Step, len(r.Definition.Steps)),
		parents:    map[string][]string{},
		dependents: map[string][]string{},
		waitingOn:  make(map[string]int, len(r.Definition.Steps)),
		done:       make(map[string]outcome, len(r.Definition.Steps)),
		started:    map[string]bool{},
		results:    make(chan stepResult),
		updates:    make(chan waitingUpdate),
		slots:      make(chan struct{}, e.maxParallel),
		waiting:    map[string]bool{},
		status:     run.StatusRunning,
	}
	masked := slices.Clone(l.masked)
	for name, v := range l.secrets {
		rn.secretEnv[name] = v
		masked = append(masked, v)
	}
	rn.masked = masked
	rn.masker = secrets.NewMasker(masked)
	rn.lineage = e.lineage(store, r)
	for _, s := range r.Definition.Steps {
		rn.steps[s.ID] = s
	}
	for _, edge := range r.Definition.Edges {
		rn.waitingOn[edge.To]++
		rn.parents[edge.To] = append(rn.parents[edge.To], edge.From)
		rn.dependents[edge.From] = append(rn.dependents[edge.From], edge.To)
	}

	if resuming {
		rn.startedAt = *r.StartedAt
		rn.status = r.Status
		log.Info("run resumed", "workflow", r.WorkflowName)
	} else {
		rn.startedAt = e.now()
		if err := e.runs.StartRun(store, r.ID, rn.startedAt); err != nil {
			log.Error("start run", "err", err)
			e.events.End(r.ID)
			return
		}
		rn.publishRun(nil, "")
		log.Info("run started", "workflow", r.WorkflowName, "steps", len(r.Definition.Steps))
	}
	rn.sink = newLogSink(store, e.runs, e.events, r.ID, rn.masker, log)

	rn.begin(resuming)
	rn.loop()
	rn.finish()
}

// begin replays the state of a resumed run, then starts every step that is
// ready.
func (rn *runner) begin(resuming bool) {
	if resuming {
		rn.stored = make(map[string]run.StepRun, len(rn.r.Steps))
		for _, st := range rn.r.Steps {
			rn.stored[st.StepID] = st
			switch {
			case st.Status.Done():
				var outputs map[string]any
				_ = json.Unmarshal(st.Outputs, &outputs)
				rn.done[st.StepID] = outcome{status: st.Status, outputs: outputs}
			case st.Status == run.StatusRunning && rn.steps[st.StepID].Block == workflow.CallBlock:
				// It runs again and picks up the runs it had started.
			case st.Status == run.StatusRunning:
				// The process died mid-step; whatever it was doing is gone.
				rn.record(stepResult{id: st.StepID, status: run.StatusCancelled,
					err: "Conductor stopped while this step was running", config: st.Config, startedAt: st.StartedAt})
				rn.done[st.StepID] = outcome{status: run.StatusCancelled}
			}
		}
		// Apply finished steps parents-first, as if they had just finished.
		for _, id := range topoOrder(rn.r.Definition) {
			if _, ok := rn.done[id]; ok {
				rn.settle(id)
			}
		}
	}
	for _, s := range rn.r.Definition.Steps {
		if rn.waitingOn[s.ID] == 0 && !rn.isDone(s.ID) && !rn.started[s.ID] {
			rn.ready(s.ID)
		}
	}
	rn.syncStatus()
}

func (rn *runner) loop() {
	for rn.inFlight > 0 {
		select {
		case u := <-rn.updates:
			if u.waiting {
				rn.waiting[u.step] = true
			} else {
				delete(rn.waiting, u.step)
			}
		case res := <-rn.results:
			rn.inFlight--
			delete(rn.waiting, res.id)
			if res.suspended {
				if res.waits {
					rn.suspended++
				}
				continue
			}
			rn.complete(res)
		}
		rn.syncStatus()
	}
}

// finish records the run's outcome, unless it is waiting for approval while
// Conductor stops; then it is left for Recover to resume.
func (rn *runner) finish() {
	if rn.suspended > 0 {
		rn.sink.Close()
		rn.log.Info("run suspended until Conductor restarts", "waiting", rn.suspended)
		return
	}

	// Steps left queued by a shutdown.
	reason := cancelReason(rn.ctx)
	for _, s := range rn.r.Definition.Steps {
		if !rn.isDone(s.ID) {
			rn.record(stepResult{id: s.ID, status: run.StatusCancelled, err: reason})
			rn.done[s.ID] = outcome{status: run.StatusCancelled}
		}
	}

	status, msg := run.StatusSucceeded, ""
	failed, cancelled := false, false
	for id, o := range rn.done {
		switch {
		case o.status == run.StatusFailed && !rn.steps[id].ContinueOnError:
			failed = true
		case o.status == run.StatusCancelled:
			cancelled = true
		}
	}
	switch {
	case rn.ctx.Err() != nil:
		status, msg = run.StatusCancelled, reason
	case failed:
		status = run.StatusFailed
	case cancelled:
		status = run.StatusCancelled
	}
	if status == run.StatusSucceeded && len(rn.r.Definition.Outputs) > 0 {
		outputs, err := evalOutputs(rn.r.Definition.Outputs, rn.exprEnv())
		if err != nil {
			status, msg = run.StatusFailed, rn.masker.Mask(err.Error())
		} else {
			_ = json.Unmarshal(maskJSON(rn.masker, outputs), &rn.outputs)
		}
	}

	// Flush logs first so the stream never ends with lines still unsent.
	rn.sink.Close()
	finishedAt := rn.e.now()
	if err := rn.e.runs.FinishRun(rn.store, rn.r.ID, status, rn.outputs, msg, finishedAt); err != nil {
		rn.log.Error("finish run", "err", err)
	}
	rn.status = status
	rn.publishRun(&finishedAt, msg)
	rn.e.events.End(rn.r.ID)
	rn.log.Info("run finished", "status", status)
}

// ready is called once everything a step depends on has finished.
func (rn *runner) ready(id string) {
	s := rn.steps[id]
	rn.started[id] = true

	if st, ok := rn.stored[id]; ok && st.Status == run.StatusWaiting {
		rn.await(s, st.Config, *st.StartedAt)
		return
	}
	if parents := rn.parents[id]; len(parents) > 0 && rn.allSkipped(parents) {
		reason := "skipped because everything it depends on was skipped"
		if len(parents) == 1 {
			reason = fmt.Sprintf("skipped because %q was skipped", parents[0])
		}
		rn.complete(stepResult{id: id, status: run.StatusSkipped, err: reason})
		return
	}

	env := rn.exprEnv()
	if s.If != "" {
		ok, err := evalCondition(s.If, env)
		switch {
		case err != nil:
			rn.complete(stepResult{id: id, status: run.StatusFailed, err: "condition: " + rn.masker.Mask(err.Error())})
			return
		case !ok:
			rn.complete(stepResult{id: id, status: run.StatusSkipped, err: "condition is false: " + s.If})
			return
		}
	}

	if waits(s.Block) {
		rn.startWaiting(s, env)
		return
	}
	resuming := rn.stored[id].Status == run.StatusRunning
	rn.inFlight++
	go func() { rn.results <- rn.runStep(s, env, resuming) }()
}

// complete records a step's result and moves on to what depends on it.
func (rn *runner) complete(res stepResult) {
	rn.record(res)
	if res.outputs == nil {
		res.outputs = map[string]any{}
	}
	rn.done[res.id] = outcome{status: res.status, outputs: res.outputs}
	rn.settle(res.id)
}

// settle applies a finished step to its dependents: they become ready, or
// are skipped or cancelled along with everything below them.
func (rn *runner) settle(id string) {
	o := rn.done[id]
	switch {
	case o.status == run.StatusCancelled:
		rn.closeDownstream(id, run.StatusCancelled, "")
	case o.status == run.StatusFailed && !rn.steps[id].ContinueOnError:
		rn.closeDownstream(id, run.StatusSkipped, fmt.Sprintf("skipped because %q failed", id))
	default:
		for _, next := range rn.dependents[id] {
			rn.waitingOn[next]--
			if rn.waitingOn[next] == 0 && !rn.isDone(next) && !rn.started[next] {
				rn.ready(next)
			}
		}
	}
}

// closeDownstream marks every unfinished step downstream of id with status.
func (rn *runner) closeDownstream(id string, status run.Status, reason string) {
	for _, next := range rn.dependents[id] {
		if rn.isDone(next) {
			continue
		}
		rn.record(stepResult{id: next, status: status, err: reason})
		rn.done[next] = outcome{status: status}
		rn.closeDownstream(next, status, reason)
	}
}

// record stores a step's final state and publishes it.
func (rn *runner) record(res stepResult) {
	recorded := maskJSON(rn.masker, res.outputs)
	errMsg := rn.masker.Mask(res.err)
	at := rn.e.now()
	if err := rn.e.runs.FinishStep(rn.store, rn.r.ID, res.id, res.status, recorded, errMsg, at); err != nil {
		rn.log.Error("finish step", "step", res.id, "err", err)
	}
	config := res.config
	if config == nil {
		config = json.RawMessage("{}")
	}
	rn.e.events.Publish(rn.r.ID, run.Event{Type: run.EventStep, Step: &run.StepRun{
		StepID: res.id, Status: res.status, Error: errMsg, Outputs: recorded,
		Config: config, StartedAt: res.startedAt, FinishedAt: &at, Progress: res.progress,
	}})
}

// syncStatus shows the run as waiting while approvals are all it has left.
func (rn *runner) syncStatus() {
	want := run.StatusRunning
	if len(rn.waiting) > 0 && len(rn.waiting) == rn.inFlight {
		want = run.StatusWaiting
	}
	if want == rn.status {
		return
	}
	rn.status = want
	if err := rn.e.runs.SetRunStatus(rn.store, rn.r.ID, want); err != nil {
		rn.log.Error("set run status", "err", err)
	}
	rn.publishRun(nil, "")
}

func (rn *runner) publishRun(finishedAt *time.Time, msg string) {
	rn.e.events.Publish(rn.r.ID, run.Event{Type: run.EventRun, State: &run.State{
		Status: rn.status, Outputs: rn.outputs, Error: msg, StartedAt: &rn.startedAt, FinishedAt: finishedAt,
	}})
	if p := rn.r.Parent; p != nil {
		rn.e.childChanged(p.RunID, run.Summary{
			ID: rn.r.ID, WorkflowID: rn.r.WorkflowID, WorkflowName: rn.r.WorkflowName,
			Status: rn.status, Error: msg, StepCount: len(rn.r.Definition.Steps),
			CreatedAt: rn.r.CreatedAt, StartedAt: &rn.startedAt, FinishedAt: finishedAt, Parent: p,
		})
	}
}

func (rn *runner) isDone(id string) bool {
	_, ok := rn.done[id]
	return ok
}

func (rn *runner) allSkipped(ids []string) bool {
	for _, id := range ids {
		if rn.done[id].status != run.StatusSkipped {
			return false
		}
	}
	return true
}

// exprEnv builds what expressions can read. Call it from the loop goroutine
// only; the result is a snapshot that step goroutines may keep.
func (rn *runner) exprEnv() map[string]any {
	steps := make(map[string]any, len(rn.done))
	for id, o := range rn.done {
		outputs := o.outputs
		if outputs == nil {
			outputs = map[string]any{}
		}
		steps[id] = map[string]any{"outputs": outputs, "status": string(o.status)}
	}
	return map[string]any{
		"steps":    steps,
		"inputs":   rn.r.Inputs,
		"secrets":  rn.secretEnv,
		"run":      map[string]any{"id": rn.r.ID},
		"workflow": map[string]any{"id": rn.r.WorkflowID, "name": rn.r.WorkflowName},
	}
}

// evalOutputs evaluates a workflow's outputs once all its steps are done.
func evalOutputs(outputs []workflow.Output, env map[string]any) (map[string]any, error) {
	values := make(map[string]any, len(outputs))
	for _, o := range outputs {
		e, err := tmpl.CompileExpr(o.Value)
		if err == nil {
			values[o.Name], err = e.Eval(env)
		}
		if err == nil && values[o.Name] == nil {
			err = errors.New("has no value; check the name")
		}
		if err != nil {
			return nil, fmt.Errorf("output %s: %w", o.Name, err)
		}
	}
	return values, nil
}

func evalCondition(src string, env map[string]any) (bool, error) {
	e, err := tmpl.CompileExpr(src)
	if err != nil {
		return false, err
	}
	v, err := e.Eval(env)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("must be true or false, but is %s", describe(v))
	}
	return b, nil
}

// topoOrder lists step ids so that every step comes after its parents.
func topoOrder(def workflow.Definition) []string {
	indegree := make(map[string]int, len(def.Steps))
	next := map[string][]string{}
	for _, e := range def.Edges {
		indegree[e.To]++
		next[e.From] = append(next[e.From], e.To)
	}
	var queue, order []string
	for _, s := range def.Steps {
		if indegree[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, n := range next[id] {
			if indegree[n]--; indegree[n] == 0 {
				queue = append(queue, n)
			}
		}
	}
	return order
}

// cancelReason explains a cancellation, or returns "" when the user asked
// for it and the status already says everything.
func cancelReason(ctx context.Context) string {
	cause := context.Cause(ctx)
	if cause == nil || errors.Is(cause, errCancelled) || errors.Is(cause, context.Canceled) {
		return ""
	}
	return cause.Error()
}

// describe renders a value briefly for log lines and messages.
func describe(v any) string {
	if v == nil {
		return "nothing"
	}
	var s string
	if str, ok := v.(string); ok {
		s = str
	} else {
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%T", v)
		}
		s = string(data)
	}
	if s = strings.TrimSpace(s); len(s) > 80 {
		s = s[:77] + "…"
	}
	return s
}

// maskJSON encodes v and masks secrets in it. Masking replaces text inside
// JSON strings; if a secret ever matched JSON syntax itself, the result
// would be invalid, so it is replaced wholesale rather than stored broken.
func maskJSON(m *secrets.Masker, v any) json.RawMessage {
	var data []byte
	switch v := v.(type) {
	case json.RawMessage:
		data = v
	default:
		var err error
		if data, err = json.Marshal(v); err != nil {
			return json.RawMessage("{}")
		}
	}
	if len(data) == 0 || string(data) == "null" {
		return json.RawMessage("{}")
	}
	out := m.Mask(string(data))
	if !json.Valid([]byte(out)) {
		return json.RawMessage(`{"_hidden":"contains a secret"}`)
	}
	return json.RawMessage(out)
}
