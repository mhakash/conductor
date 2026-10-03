package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"conductor/internal/block"
	"conductor/internal/run"
	"conductor/internal/workflow"
)

type stepResult struct {
	id      string
	status  run.Status
	outputs map[string]any
	err     string
}

// execute runs r to completion. Steps start as soon as all their
// dependencies have succeeded, at most maxParallel at a time. When a step
// fails, everything downstream of it is skipped; independent branches carry on.
func (e *Engine) execute(ctx context.Context, r run.Run) {
	// Bookkeeping writes must succeed even after ctx is cancelled.
	store := context.WithoutCancel(ctx)
	log := e.log.With("run", r.ID)

	if err := e.runs.StartRun(store, r.ID, e.now()); err != nil {
		log.Error("start run", "err", err)
		return
	}
	log.Info("run started", "workflow", r.WorkflowName, "steps", len(r.Definition.Steps))

	sink := newLogSink(store, e.runs, r.ID, log)
	defer sink.Close()

	steps := make(map[string]workflow.Step, len(r.Definition.Steps))
	waitingOn := make(map[string]int, len(r.Definition.Steps))
	dependents := make(map[string][]string)
	for _, s := range r.Definition.Steps {
		steps[s.ID] = s
		waitingOn[s.ID] = 0
	}
	for _, edge := range r.Definition.Edges {
		waitingOn[edge.To]++
		dependents[edge.From] = append(dependents[edge.From], edge.To)
	}

	results := make(chan stepResult)
	slots := make(chan struct{}, e.maxParallel)
	finished := make(map[string]bool, len(steps))
	pending := len(steps)

	launch := func(s workflow.Step) {
		go func() {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				results <- stepResult{id: s.ID, status: run.StatusCancelled, err: cancelReason(ctx)}
				return
			}
			results <- e.runStep(ctx, store, r.ID, s, sink)
		}()
	}

	// close marks every not-yet-finished step downstream of id with status.
	var closeDownstream func(id string, status run.Status, reason string)
	closeDownstream = func(id string, status run.Status, reason string) {
		for _, next := range dependents[id] {
			if finished[next] {
				continue
			}
			finished[next] = true
			pending--
			if err := e.runs.FinishStep(store, r.ID, next, status, nil, reason, e.now()); err != nil {
				log.Error("finish step", "step", next, "err", err)
			}
			closeDownstream(next, status, reason)
		}
	}

	for _, s := range r.Definition.Steps {
		if waitingOn[s.ID] == 0 {
			launch(s)
		}
	}

	failed := false
	for pending > 0 {
		res := <-results
		finished[res.id] = true
		pending--

		outputs, err := json.Marshal(res.outputs)
		if err != nil {
			outputs = nil
		}
		if err := e.runs.FinishStep(store, r.ID, res.id, res.status, outputs, res.err, e.now()); err != nil {
			log.Error("finish step", "step", res.id, "err", err)
		}

		switch res.status {
		case run.StatusSucceeded:
			for _, next := range dependents[res.id] {
				waitingOn[next]--
				if waitingOn[next] == 0 && !finished[next] {
					launch(steps[next])
				}
			}
		case run.StatusCancelled:
			closeDownstream(res.id, run.StatusCancelled, "")
		default:
			failed = true
			closeDownstream(res.id, run.StatusSkipped, fmt.Sprintf("skipped because %q failed", res.id))
		}
	}

	status, msg := run.StatusSucceeded, ""
	switch {
	case ctx.Err() != nil:
		status, msg = run.StatusCancelled, cancelReason(ctx)
	case failed:
		status = run.StatusFailed
	}
	if err := e.runs.FinishRun(store, r.ID, status, msg, e.now()); err != nil {
		log.Error("finish run", "err", err)
	}
	log.Info("run finished", "status", status)
}

func (e *Engine) runStep(ctx, store context.Context, runID string, s workflow.Step, sink *logSink) stepResult {
	b, _ := e.blocks.Get(s.Block)
	timeout := DefaultStepTimeout
	if s.Timeout != "" {
		// Validated before the run started.
		timeout, _ = time.ParseDuration(s.Timeout)
	}
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := e.runs.StartStep(store, runID, s.ID, e.now()); err != nil {
		e.log.Error("start step", "run", runID, "step", s.ID, "err", err)
	}

	env, flush := sink.envFor(s.ID)
	outputs, err := safeRun(stepCtx, b.Run, s.Config, env)
	flush()

	res := stepResult{id: s.ID, status: run.StatusSucceeded, outputs: outputs}
	switch {
	case err == nil:
	case ctx.Err() != nil:
		res.status, res.err = run.StatusCancelled, cancelReason(ctx)
	case errors.Is(stepCtx.Err(), context.DeadlineExceeded):
		res.status, res.err = run.StatusFailed, fmt.Sprintf("timed out after %s", timeout)
	default:
		res.status, res.err = run.StatusFailed, err.Error()
	}
	switch {
	case res.err != "":
		env.Logf("%s", res.err)
	case res.status == run.StatusCancelled:
		env.Logf("cancelled")
	}
	return res
}

// safeRun turns a panic inside a block into a step failure instead of
// taking the whole process down.
func safeRun(ctx context.Context, fn block.RunFunc, config json.RawMessage, env block.Env) (out map[string]any, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("internal error in block: %v", p)
		}
	}()
	return fn(ctx, config, env)
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
