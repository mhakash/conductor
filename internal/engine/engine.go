// Package engine executes workflow runs: it schedules steps by dependency,
// runs independent steps in parallel and records progress in the run store.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"conductor/internal/block"
	"conductor/internal/run"
	"conductor/internal/secrets"
	"conductor/internal/tmpl"
	"conductor/internal/workflow"
)

// DefaultStepTimeout applies to steps that don't set their own.
const DefaultStepTimeout = time.Hour

var (
	ErrNotActive = errors.New("run is not active")
	// ErrNotWaiting means a decision was sent for a step that isn't waiting
	// for one (any more).
	ErrNotWaiting = errors.New("step is not waiting for approval")
)

// errShutdown is the cancellation cause when Conductor itself is stopping.
var errShutdown = errors.New("Conductor was stopped while this run was in progress")

// errCancelled is the cancellation cause when a user cancels a run.
var errCancelled = errors.New("cancelled")

type Options struct {
	Runs   run.Repository
	Blocks *block.Registry
	Log    *slog.Logger
	// Events receives live progress; nil discards it.
	Events run.Publisher
	// Secrets provides values for {{ secrets.NAME }}; nil means no secrets.
	Secrets SecretSource
	// Workflows loads the workflows that steps run as sub-workflows; nil
	// means such steps fail.
	Workflows workflow.Getter
	// MaxParallel caps how many steps of one run execute at the same time.
	MaxParallel int
}

type Engine struct {
	runs        run.Repository
	events      run.Publisher
	secrets     SecretSource
	workflows   workflow.Getter
	blocks      *block.Registry
	log         *slog.Logger
	maxParallel int
	now         func() time.Time

	mu     sync.Mutex
	active map[string]activeRun
	// approvals delivers decisions to waiting approval steps, by approvalKey.
	approvals map[string]chan Decision
	// watchers hear about status changes of child runs, by child run id.
	watchers map[string]func(run.Status)
	wg       sync.WaitGroup
	closed   bool
}

type activeRun struct {
	cancel context.CancelCauseFunc
	// done is closed once the run has recorded its final state, or been
	// left waiting by a shutdown.
	done chan struct{}
}

// launch is what spawn needs to execute a run.
type launch struct {
	run     run.Run
	secrets map[string]string
	// masked are further values to mask, such as secrets a parent run
	// passed in as inputs.
	masked []string
	// resuming means the run was loaded from the store after a restart.
	resuming bool
}

// Decision is someone's answer to an approval step.
type Decision struct {
	Approve bool   `json:"approve"`
	Comment string `json:"comment"`
}

func New(o Options) *Engine {
	if o.MaxParallel <= 0 {
		o.MaxParallel = 8
	}
	if o.Events == nil {
		o.Events = nopPublisher{}
	}
	return &Engine{
		runs:        o.Runs,
		events:      o.Events,
		secrets:     o.Secrets,
		workflows:   o.Workflows,
		blocks:      o.Blocks,
		log:         o.Log,
		maxParallel: o.MaxParallel,
		now:         func() time.Time { return time.Now().UTC() },
		active:      map[string]activeRun{},
		approvals:   map[string]chan Decision{},
		watchers:    map[string]func(run.Status){},
	}
}

// SecretSource decrypts secrets by name; a missing name is a
// *secrets.MissingError.
type SecretSource interface {
	Values(ctx context.Context, names []string) (map[string]string, error)
}

// Start validates wf and inputs, records a new run and executes it in the
// background. It returns a *workflow.ValidationError if the workflow can't
// run as is.
func (e *Engine) Start(ctx context.Context, wf workflow.Workflow, inputs map[string]any) (run.Run, error) {
	if err := e.Check(ctx, wf); err != nil {
		return run.Run{}, err
	}
	resolved, err := workflow.ResolveInputs(wf.Definition.Inputs, inputs)
	if err != nil {
		return run.Run{}, err
	}
	secretValues, err := e.loadSecrets(ctx, wf.Definition)
	if err != nil {
		return run.Run{}, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return run.Run{}, errors.New("engine is shutting down")
	}

	r, err := e.runs.Create(ctx, run.Run{
		WorkflowID:   wf.ID,
		WorkflowName: wf.Name,
		Definition:   wf.Definition,
		Inputs:       resolved,
	})
	if err != nil {
		return run.Run{}, err
	}
	e.spawn(launch{run: r, secrets: secretValues})
	return r, nil
}

// spawn executes a run in the background. Call with e.mu held.
func (e *Engine) spawn(l launch) {
	// The run outlives the HTTP request (or parent step) that started it.
	runCtx, cancel := context.WithCancelCause(context.Background())
	done := make(chan struct{})
	id := l.run.ID
	e.active[id] = activeRun{cancel: cancel, done: done}
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			delete(e.active, id)
			e.mu.Unlock()
			cancel(nil)
			close(done)
		}()
		e.execute(runCtx, l)
	}()
}

// Recover deals with runs a previous process left unfinished: those waiting
// for approval carry on where they were, the rest are marked failed. Call it
// once at startup, before starting new runs.
func (e *Engine) Recover(ctx context.Context) error {
	n, err := e.runs.FailUnfinished(ctx, "Conductor stopped while this run was in progress", e.now())
	if err != nil {
		return err
	}
	if n > 0 {
		e.log.Warn("marked interrupted runs as failed", "count", n)
	}
	ids, err := e.runs.Unfinished(ctx)
	if err != nil {
		return err
	}
	var launches []launch
	// Oldest first, so a parent is dealt with before the runs it started.
	for _, id := range ids {
		r, err := e.runs.Get(ctx, id)
		if err != nil {
			return err
		}
		if orphaned, err := e.orphaned(ctx, r); err != nil {
			return err
		} else if orphaned {
			e.abandon(ctx, r, "The run that started this one has finished")
			continue
		}
		secretValues, err := e.loadSecrets(ctx, r.Definition)
		if err != nil {
			e.abandon(ctx, r, "Couldn't resume after a restart: "+err.Error())
			continue
		}
		launches = append(launches, launch{run: r, secrets: secretValues, resuming: true})
	}
	// Spawn them all at once: a resumed parent looks for its children among
	// the active runs.
	e.mu.Lock()
	for _, l := range launches {
		e.spawn(l)
	}
	e.mu.Unlock()
	if len(launches) > 0 {
		e.log.Info("resumed runs waiting for approval", "count", len(launches))
	}
	return nil
}

// orphaned reports whether r was started by a step that is no longer
// running, so nothing would ever pick up its result.
func (e *Engine) orphaned(ctx context.Context, r run.Run) (bool, error) {
	if r.Parent == nil {
		return false, nil
	}
	parent, err := e.runs.Get(ctx, r.Parent.RunID)
	if errors.Is(err, run.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if parent.Status.Done() {
		return true, nil
	}
	for _, st := range parent.Steps {
		if st.StepID == r.Parent.StepID {
			return st.Status.Done(), nil
		}
	}
	return true, nil
}

// abandon fails a run that can't be resumed, with its unfinished steps.
func (e *Engine) abandon(ctx context.Context, r run.Run, msg string) {
	at := e.now()
	for _, st := range r.Steps {
		if !st.Status.Done() {
			_ = e.runs.FinishStep(ctx, r.ID, st.StepID, run.StatusFailed, nil, msg, at)
		}
	}
	if err := e.runs.FinishRun(ctx, r.ID, run.StatusFailed, nil, msg, at); err != nil {
		e.log.Error("abandon run", "run", r.ID, "err", err)
	}
}

// Decide approves or rejects an approval step that is waiting.
func (e *Engine) Decide(runID, stepID string, d Decision) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.active[runID]; !ok {
		return ErrNotActive
	}
	key := approvalKey(runID, stepID)
	decisions, ok := e.approvals[key]
	if !ok {
		return ErrNotWaiting
	}
	delete(e.approvals, key) // a second click gets ErrNotWaiting
	decisions <- d           // buffered; the step reads it
	return nil
}

func (e *Engine) addApproval(runID, stepID string) <-chan Decision {
	ch := make(chan Decision, 1)
	e.mu.Lock()
	e.approvals[approvalKey(runID, stepID)] = ch
	e.mu.Unlock()
	return ch
}

func (e *Engine) removeApproval(runID, stepID string) {
	e.mu.Lock()
	delete(e.approvals, approvalKey(runID, stepID))
	e.mu.Unlock()
}

func approvalKey(runID, stepID string) string { return runID + "/" + stepID }

// loadSecrets decrypts every secret the workflow's expressions reference.
// Missing ones are reported against the steps that use them.
func (e *Engine) loadSecrets(ctx context.Context, def workflow.Definition) (map[string]string, error) {
	users := map[string][]string{} // secret name → step ids
	var names []string
	for _, s := range def.Steps {
		for _, r := range e.stepRefs(s) {
			if r.Kind == "secrets" {
				if users[r.Name] == nil {
					names = append(names, r.Name)
				}
				users[r.Name] = append(users[r.Name], s.ID)
			}
		}
	}
	if len(names) == 0 {
		return map[string]string{}, nil
	}
	if e.secrets == nil {
		return nil, errors.New("secrets are not available")
	}

	values, err := e.secrets.Values(ctx, names)
	var missing *secrets.MissingError
	if errors.As(err, &missing) {
		var problems []workflow.Problem
		for _, name := range missing.Names {
			for _, step := range users[name] {
				problems = append(problems, workflow.Problem{
					Step:    step,
					Message: fmt.Sprintf("step %q uses secret %s, which doesn't exist; add it under Secrets", step, name),
				})
			}
		}
		return nil, &workflow.ValidationError{Problems: problems}
	}
	return values, err
}

// stepRefs lists what a step's config, condition and list of items read.
// The step must have passed validation.
func (e *Engine) stepRefs(s workflow.Step) []tmpl.Ref {
	refs, _ := tmpl.ConfigRefs(s.Config, e.blocks.Literal(s.Block)...)
	for _, src := range []string{s.If, forEachItems(s)} {
		if src == "" {
			continue
		}
		if e, err := tmpl.CompileExpr(src); err == nil {
			refs = append(refs, e.Refs()...)
		}
	}
	return refs
}

func forEachItems(s workflow.Step) string {
	if s.ForEach == nil {
		return ""
	}
	return s.ForEach.Items
}

// Cancel stops a running run. Running steps are interrupted; steps that
// haven't started are marked cancelled.
func (e *Engine) Cancel(id string) error {
	return e.cancelRun(id, errCancelled)
}

func (e *Engine) cancelRun(id string, cause error) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	a, ok := e.active[id]
	if !ok {
		return ErrNotActive
	}
	a.cancel(cause)
	return nil
}

// doneChan returns a channel closed when run id stops executing, or nil if
// it isn't executing.
func (e *Engine) doneChan(id string) <-chan struct{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	if a, ok := e.active[id]; ok {
		return a.done
	}
	return nil
}

// Shutdown cancels all active runs and waits for them to record their final
// state, or for ctx to expire. Runs waiting for approval are left waiting,
// for Recover to resume on the next start.
func (e *Engine) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	e.closed = true
	for _, a := range e.active {
		a.cancel(errShutdown)
	}
	e.mu.Unlock()

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Check reports everything that would stop wf from running: structural
// problems, unknown blocks, missing required config and workflows it runs
// that don't exist or would run it in turn.
func (e *Engine) Check(ctx context.Context, wf workflow.Workflow) error {
	if err := workflow.Validate(wf, e.blocks); err != nil {
		return err
	}
	if e.workflows != nil {
		if err := workflow.ValidateCalls(ctx, wf, e.workflows); err != nil {
			return err
		}
	}
	var problems []workflow.Problem
	if len(wf.Definition.Steps) == 0 {
		problems = append(problems, workflow.Problem{Message: "add at least one step before running"})
	}
	for _, s := range wf.Definition.Steps {
		b, _ := e.blocks.Get(s.Block)
		missing, err := b.Missing(s.Config)
		switch {
		case err != nil:
			problems = append(problems, workflow.Problem{Step: s.ID, Message: fmt.Sprintf("step %q: %v", s.ID, err)})
		case len(missing) > 0:
			problems = append(problems, workflow.Problem{
				Step:    s.ID,
				Message: fmt.Sprintf("step %q needs %s", s.ID, strings.Join(missing, ", ")),
			})
		}
		switch {
		case s.Block == block.ApprovalType && (s.Retry != nil || s.ForEach != nil):
			problems = append(problems, workflow.Problem{Step: s.ID, Message: fmt.Sprintf("step %q: approval steps can't retry or run once per item", s.ID)})
		case s.Block != block.ApprovalType && s.Block != workflow.CallBlock && b.Run == nil:
			problems = append(problems, workflow.Problem{Step: s.ID, Message: fmt.Sprintf("block %q can't run yet", s.Block)})
		}
	}
	if len(problems) > 0 {
		return &workflow.ValidationError{Problems: problems}
	}
	return nil
}

type nopPublisher struct{}

func (nopPublisher) Publish(string, run.Event) {}
func (nopPublisher) End(string)                {}
