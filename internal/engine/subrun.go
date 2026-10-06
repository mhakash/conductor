package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"conductor/internal/block"
	"conductor/internal/run"
	"conductor/internal/workflow"
)

var (
	// errSuspended means a child run was left waiting for approval because
	// Conductor is stopping; the step carries on after the restart.
	errSuspended = errors.New("left waiting until Conductor restarts")
	// errChildCancelled fails a step whose child run was cancelled; the step
	// counts as cancelled rather than failed, so it isn't retried.
	errChildCancelled = errors.New("cancelled")
	// errParentStopped is the cause given to a child run when the step that
	// started it is cancelled or times out.
	errParentStopped = errors.New("stopped by the step that started this run")
)

// statusSuspended is what try reports for errSuspended. It is never stored.
const statusSuspended run.Status = "suspended"

// calls runs the workflow of one step as child runs (one per item for a
// step that loops), and tracks whether they are all waiting for approval.
type calls struct {
	rn   *runner
	step workflow.Step

	mu sync.Mutex
	// active maps each unfinished child run to whether it is waiting.
	active  map[string]bool
	waiting bool
	// previous holds the latest child run of each item from before a
	// restart, by item (-1 for a step that doesn't loop).
	previous map[int]run.Summary
}

func (rn *runner) newCalls(s workflow.Step, resuming bool) *calls {
	c := &calls{rn: rn, step: s, active: map[string]bool{}}
	if !resuming {
		return c
	}
	children, err := rn.e.runs.Children(rn.store, rn.r.ID)
	if err != nil {
		rn.log.Error("list child runs", "step", s.ID, "err", err)
	}
	c.previous = map[int]run.Summary{}
	for _, child := range children {
		if child.Parent.StepID == s.ID {
			c.previous[itemKey(child.Parent.Item)] = child
		}
	}
	return c
}

func itemKey(item *int) int {
	if item == nil {
		return -1
	}
	return *item
}

// runFunc returns the RunFunc for one item of the step (nil if it doesn't
// loop). Each call of it is one attempt, and starts one child run.
func (c *calls) runFunc(item *int) block.RunFunc {
	return func(ctx context.Context, config json.RawMessage, env block.Env) (map[string]any, error) {
		return c.run(ctx, item, config, env)
	}
}

func (c *calls) run(ctx context.Context, item *int, config json.RawMessage, env block.Env) (map[string]any, error) {
	rn := c.rn
	var id string
	if prev, ok := c.takePrevious(item); ok {
		// A run started before the restart: use its result, or wait for it
		// if it is still going. Starting it again could repeat its work.
		id = prev.ID
		rn.e.watch(id, c.update)
		if !prev.Status.Done() {
			env.Logf("Carrying on with %s (run %s) after the restart", prev.WorkflowName, prev.ID)
			c.update(id, prev.Status)
		}
	} else {
		child, err := c.start(ctx, item, config)
		if err != nil {
			return nil, err
		}
		id = child.ID
		env.Logf("Started %s (run %s)", child.WorkflowName, child.ID)
	}
	defer rn.e.unwatch(id)

	if done := rn.e.doneChan(id); done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			// On shutdown the child is stopped by Shutdown itself, and may be
			// left waiting for approval.
			if !errors.Is(context.Cause(rn.ctx), errShutdown) {
				_ = rn.e.cancelRun(id, errParentStopped)
			}
			<-done
		}
	}

	child, err := rn.e.runs.Get(rn.store, id)
	if err != nil {
		return nil, err
	}
	if child.Status.Done() {
		c.update(id, child.Status) // in case its last change came before we watched
	}
	switch child.Status {
	case run.StatusSucceeded:
		env.Logf("%s succeeded", child.WorkflowName)
		return child.Outputs, nil
	case run.StatusCancelled:
		return nil, fmt.Errorf("%s was %w", child.WorkflowName, errChildCancelled)
	case run.StatusFailed:
		return nil, errors.New(failureOf(child))
	default:
		return nil, errSuspended
	}
}

func (c *calls) takePrevious(item *int) (run.Summary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev, ok := c.previous[itemKey(item)]
	delete(c.previous, itemKey(item))
	return prev, ok
}

// update records a child run's status and tells the runner when the step
// starts or stops waiting: when every child still going is waiting.
func (c *calls) update(id string, status run.Status) {
	c.mu.Lock()
	defer c.mu.Unlock() // held while sending, so updates arrive in order
	if status.Done() {
		delete(c.active, id)
	} else {
		c.active[id] = status == run.StatusWaiting
	}
	waiting := len(c.active) > 0
	for _, w := range c.active {
		waiting = waiting && w
	}
	if waiting != c.waiting {
		c.waiting = waiting
		c.rn.updates <- waitingUpdate{step: c.step.ID, waiting: waiting}
	}
}

// start checks and starts the workflow a step runs, as a child run.
func (c *calls) start(ctx context.Context, item *int, config json.RawMessage) (run.Run, error) {
	rn := c.rn
	call, err := workflow.ParseCall(config)
	switch {
	case err != nil:
		return run.Run{}, err
	case call.Workflow == "":
		return run.Run{}, errors.New("choose the workflow to run")
	case rn.e.workflows == nil:
		return run.Run{}, errors.New("workflows can't be run as steps here")
	case len(rn.lineage) >= workflow.MaxCallDepth:
		return run.Run{}, fmt.Errorf("workflows can run one another at most %d levels deep", workflow.MaxCallDepth)
	}
	wf, err := rn.e.workflows(ctx, call.Workflow)
	if errors.Is(err, workflow.ErrNotFound) {
		return run.Run{}, errors.New("the workflow this step runs no longer exists")
	}
	if err != nil {
		return run.Run{}, err
	}
	if slices.Contains(rn.lineage, wf.ID) {
		return run.Run{}, fmt.Errorf("%s is already running further up; a workflow can't run itself", wf.Name)
	}

	parent := &run.Parent{RunID: rn.r.ID, WorkflowName: rn.r.WorkflowName, StepID: c.step.ID, Item: item}
	return rn.e.startChild(ctx, wf, call.Inputs, parent, rn, c.update)
}

// startChild starts wf as a child run of parent. Its inputs may hold the
// parent's secrets, so they are stored masked and the child masks them too.
// watch hears about the child's status from the start.
func (e *Engine) startChild(ctx context.Context, wf workflow.Workflow, inputs map[string]any, parent *run.Parent, from *runner, watch func(string, run.Status)) (run.Run, error) {
	if err := e.Check(ctx, wf); err != nil {
		return run.Run{}, fmt.Errorf("%s can't run: %s", wf.Name, explain(err))
	}
	resolved, err := workflow.ResolveInputs(wf.Definition.Inputs, inputs)
	if err != nil {
		return run.Run{}, errors.New(explain(err))
	}
	secretValues, err := e.loadSecrets(ctx, wf.Definition)
	if err != nil {
		return run.Run{}, fmt.Errorf("%s can't run: %s", wf.Name, explain(err))
	}
	var stored map[string]any
	_ = json.Unmarshal(maskJSON(from.masker, resolved), &stored)

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return run.Run{}, errors.New("Conductor is shutting down")
	}
	r, err := e.runs.Create(ctx, run.Run{
		WorkflowID:   wf.ID,
		WorkflowName: wf.Name,
		Definition:   wf.Definition,
		Inputs:       stored,
		Parent:       parent,
	})
	if err != nil {
		return run.Run{}, err
	}
	e.watchers[r.ID] = func(st run.Status) { watch(r.ID, st) }
	r.Inputs = resolved
	e.spawn(launch{run: r, secrets: secretValues, masked: from.masked})
	return r, nil
}

// explain renders an error as one line, listing a validation error's
// problems without its generic prefix.
func explain(err error) string {
	var verr *workflow.ValidationError
	if !errors.As(err, &verr) {
		return err.Error()
	}
	msgs := make([]string, len(verr.Problems))
	for i, p := range verr.Problems {
		msgs[i] = p.Message
	}
	return strings.Join(msgs, "; ")
}

// failureOf explains why a child run failed: its error, or the first step
// that failed.
func failureOf(r run.Run) string {
	if r.Error != "" {
		return fmt.Sprintf("%s failed: %s", r.WorkflowName, r.Error)
	}
	for _, st := range r.Steps {
		if st.Status == run.StatusFailed {
			return fmt.Sprintf("%s failed at step %q: %s", r.WorkflowName, st.StepID, st.Error)
		}
	}
	return r.WorkflowName + " failed"
}

func (e *Engine) watch(id string, fn func(string, run.Status)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.watchers[id] = func(st run.Status) { fn(id, st) }
}

func (e *Engine) unwatch(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.watchers, id)
}

// childChanged passes a child run's new state to the run that started it:
// its stream, and the step waiting for it.
func (e *Engine) childChanged(parentID string, child run.Summary) {
	e.events.Publish(parentID, run.Event{Type: run.EventChild, Child: &child})
	e.mu.Lock()
	watcher := e.watchers[child.ID]
	e.mu.Unlock()
	if watcher != nil {
		watcher(child.Status)
	}
}

// lineage lists the workflow ids of r and the runs above it, r's first.
func (e *Engine) lineage(ctx context.Context, r run.Run) []string {
	ids := []string{r.WorkflowID}
	for p := r.Parent; p != nil && len(ids) <= workflow.MaxCallDepth; {
		parent, err := e.runs.Get(ctx, p.RunID)
		if err != nil {
			break
		}
		ids = append(ids, parent.WorkflowID)
		p = parent.Parent
	}
	return ids
}
