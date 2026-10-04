package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"sync"
	"time"

	"conductor/internal/block"
	"conductor/internal/run"
	"conductor/internal/tmpl"
	"conductor/internal/workflow"
)

// runStep runs one step that does work: once, or once per item, retrying
// failures as its settings allow. It runs in its own goroutine.
func (rn *runner) runStep(s workflow.Step, exprEnv map[string]any) stepResult {
	res := stepResult{id: s.ID}

	// A step waits in the queue for a free slot before it counts as started.
	if !rn.acquire() {
		return rn.interrupted(res)
	}
	held := true
	defer func() {
		if held {
			rn.release()
		}
	}()

	// Resolve everything before starting, so an expression typo fails the
	// step before any item has run.
	var items []any
	var configs []json.RawMessage
	var setupErr error
	if s.ForEach != nil {
		items, setupErr = evalItems(s.ForEach.Items, exprEnv)
		if setupErr != nil {
			setupErr = fmt.Errorf("list of items: %w", setupErr)
		}
		for i, item := range items {
			if setupErr != nil {
				break
			}
			env := maps.Clone(exprEnv)
			env["item"], env["index"] = item, i
			config, err := tmpl.RenderConfig(s.Config, env)
			if err != nil {
				setupErr = fmt.Errorf("item %d: config: %w", i+1, err)
			}
			configs = append(configs, config)
		}
		res.config = json.RawMessage("[]")
		if setupErr == nil && len(configs) > 0 {
			res.config = maskJSON(rn.masker, configs)
		}
		res.progress.Items = len(items)
	} else {
		config, err := tmpl.RenderConfig(s.Config, exprEnv)
		if err != nil {
			setupErr = fmt.Errorf("config: %w", err)
			config = s.Config // record what we have
		}
		configs = []json.RawMessage{config}
		res.config = maskJSON(rn.masker, config)
	}

	startedAt := rn.e.now()
	res.startedAt = &startedAt
	if err := rn.e.runs.StartStep(rn.store, rn.r.ID, s.ID, run.StatusRunning, res.config, startedAt); err != nil {
		rn.log.Error("start step", "step", s.ID, "err", err)
	}
	if res.progress != (run.Progress{}) {
		rn.saveProgress(s.ID, res.progress)
	}
	rn.publishStep(res, run.StatusRunning)

	b, _ := rn.e.blocks.Get(s.Block)
	env, flush := rn.sink.envFor(s.ID, nil)
	switch {
	case setupErr != nil:
		res.status, res.err = run.StatusFailed, setupErr.Error()
	case s.ForEach != nil:
		held = false
		rn.release() // items take their own slots
		res.outputs, res.status, res.err = rn.runItems(s, b, items, configs, &res)
	default:
		var attempt int
		res.outputs, res.status, res.err, attempt = rn.try(s, b, configs[0], env, flush, &held, func(n int) {
			res.progress.Attempt = n
			rn.saveProgress(s.ID, res.progress)
			rn.publishStep(res, run.StatusRunning)
		})
		if s.Retry != nil {
			res.progress.Attempt = attempt
		}
	}
	switch {
	case res.err != "":
		env.Logf("%s", res.err)
	case res.status == run.StatusCancelled:
		env.Logf("cancelled")
	}
	flush()
	return res
}

// try runs config through the block, retrying failures if the step allows.
// *held says whether the caller's slot is still taken; try releases it while
// waiting to retry. onRetry is called with the number of each new attempt.
func (rn *runner) try(s workflow.Step, b block.Block, config json.RawMessage, env block.Env, flush func(), held *bool, onRetry func(n int)) (map[string]any, run.Status, string, int) {
	attempts := 1
	if s.Retry != nil {
		attempts += s.Retry.Max
	}
	timeout := DefaultStepTimeout
	if s.Timeout != "" {
		timeout, _ = time.ParseDuration(s.Timeout) // validated before the run started
	}

	for n := 1; ; n++ {
		if !*held {
			if !rn.acquire() {
				return nil, run.StatusCancelled, cancelReason(rn.ctx), n
			}
			*held = true
		}
		stepCtx, cancel := context.WithTimeout(rn.ctx, timeout)
		outputs, err := safeRun(stepCtx, b.Run, config, env)
		timedOut := errors.Is(stepCtx.Err(), context.DeadlineExceeded)
		cancel()
		flush()

		status, msg := run.StatusSucceeded, ""
		switch {
		case err == nil:
		case rn.ctx.Err() != nil:
			status, msg = run.StatusCancelled, cancelReason(rn.ctx)
		case timedOut:
			status, msg = run.StatusFailed, fmt.Sprintf("timed out after %s", timeout)
		default:
			status, msg = run.StatusFailed, err.Error()
		}
		if status != run.StatusFailed || n == attempts {
			return outputs, status, msg, n
		}

		delay := s.Retry.DelayBefore(n)
		env.Logf("Attempt %d of %d failed: %s", n, attempts, msg)
		env.Logf("Retrying in %s", delay)
		rn.release()
		*held = false
		select {
		case <-time.After(delay):
		case <-rn.ctx.Done():
			return outputs, run.StatusCancelled, cancelReason(rn.ctx), n
		}
		onRetry(n + 1)
	}
}

// runItems runs the step once per item, up to its concurrency at a time.
// The first failure stops items that haven't started; the step's outputs
// are the items' outputs in order, under "results".
func (rn *runner) runItems(s workflow.Step, b block.Block, items []any, configs []json.RawMessage, res *stepResult) (map[string]any, run.Status, string) {
	concurrency := max(s.ForEach.Concurrency, 1)
	results := make([]any, len(items))
	if len(items) == 0 {
		env, flush := rn.sink.envFor(s.ID, nil)
		env.Logf("The list is empty; nothing to do")
		flush()
		return map[string]any{"results": results}, run.StatusSucceeded, ""
	}

	var (
		mu      sync.Mutex
		status  = run.StatusSucceeded
		failure string
		wg      sync.WaitGroup
	)
	room := make(chan struct{}, concurrency)
	stopped := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return status != run.StatusSucceeded
	}

	for i := range items {
		select {
		case room <- struct{}{}:
		case <-rn.ctx.Done():
		}
		if rn.ctx.Err() != nil || stopped() {
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-room }()
			env, flush := rn.sink.envFor(s.ID, &i)
			env.Logf("Item %d of %d: %s", i+1, len(items), describe(items[i]))
			held := false
			out, st, msg, _ := rn.try(s, b, configs[i], env, flush, &held, func(int) {})
			if held {
				rn.release()
			}
			if msg != "" {
				env.Logf("%s", msg)
			}
			flush()

			mu.Lock()
			defer mu.Unlock()
			if out == nil {
				out = map[string]any{}
			}
			results[i] = out
			if st == run.StatusSucceeded {
				res.progress.ItemsDone++
				rn.saveProgress(s.ID, res.progress)
				rn.publishStep(*res, run.StatusRunning)
			} else if status == run.StatusSucceeded {
				status = st
				failure = fmt.Sprintf("item %d of %d: %s", i+1, len(items), msg)
				if st == run.StatusCancelled {
					failure = msg
				}
			}
		}()
	}
	wg.Wait()

	if status == run.StatusSucceeded && rn.ctx.Err() != nil {
		status, failure = run.StatusCancelled, cancelReason(rn.ctx)
	}
	return map[string]any{"results": results}, status, failure
}

// evalItems evaluates a step's list of items.
func evalItems(src string, env map[string]any) ([]any, error) {
	e, err := tmpl.CompileExpr(src)
	if err != nil {
		return nil, err
	}
	v, err := e.Eval(env)
	if err != nil {
		return nil, err
	}
	rv := reflect.ValueOf(v)
	if v == nil || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
		return nil, fmt.Errorf("must be a list, but is %s", describe(v))
	}
	if rv.Len() > workflow.MaxForEachItems {
		return nil, fmt.Errorf("has %d items; the most a step can run is %d", rv.Len(), workflow.MaxForEachItems)
	}
	items := make([]any, rv.Len())
	for i := range items {
		items[i] = rv.Index(i).Interface()
	}
	return items, nil
}

// startApproval records an approval step as waiting and waits for a decision.
func (rn *runner) startApproval(s workflow.Step, exprEnv map[string]any) {
	config, err := tmpl.RenderConfig(s.Config, exprEnv)
	if err != nil {
		config = s.Config
	}
	recorded := maskJSON(rn.masker, config)
	startedAt := rn.e.now()
	if err != nil {
		rn.complete(stepResult{id: s.ID, status: run.StatusFailed, err: "config: " + rn.masker.Mask(err.Error()),
			config: recorded, startedAt: &startedAt})
		return
	}
	if err := rn.e.runs.StartStep(rn.store, rn.r.ID, s.ID, run.StatusWaiting, recorded, startedAt); err != nil {
		rn.log.Error("start step", "step", s.ID, "err", err)
	}
	rn.publishStep(stepResult{id: s.ID, config: recorded, startedAt: &startedAt}, run.StatusWaiting)
	env, flush := rn.sink.envFor(s.ID, nil)
	env.Logf("Waiting for approval")
	flush()
	rn.await(s, recorded, startedAt)
}

// await waits for a decision on an approval step, its timeout (counted from
// when it started waiting, across restarts), or the run ending.
func (rn *runner) await(s workflow.Step, config json.RawMessage, startedAt time.Time) {
	decisions := rn.e.addApproval(rn.r.ID, s.ID)
	var deadline <-chan time.Time
	var timeout time.Duration
	stop := func() bool { return false }
	if s.Timeout != "" {
		timeout, _ = time.ParseDuration(s.Timeout)
		timer := time.NewTimer(time.Until(startedAt.Add(timeout)))
		deadline, stop = timer.C, timer.Stop
	}

	rn.inFlight++
	rn.waiting[s.ID] = true
	go func() {
		defer stop()
		defer rn.e.removeApproval(rn.r.ID, s.ID)
		res := stepResult{id: s.ID, config: config, startedAt: &startedAt, approval: true}
		env, flush := rn.sink.envFor(s.ID, nil)
		select {
		case d := <-decisions:
			res.outputs = map[string]any{"approved": d.Approve, "comment": d.Comment}
			verdict := "Approved"
			if !d.Approve {
				verdict = "Rejected"
			}
			if d.Comment != "" {
				verdict += ": " + d.Comment
			}
			env.Logf("%s", verdict)
			res.status = run.StatusSucceeded
			if !d.Approve {
				res.status, res.err = run.StatusFailed, verdict
			}
		case <-deadline:
			res.status, res.err = run.StatusFailed, fmt.Sprintf("no decision within %s", timeout)
			env.Logf("%s", res.err)
		case <-rn.ctx.Done():
			if errors.Is(context.Cause(rn.ctx), errShutdown) {
				res.suspended = true
			} else {
				res.status, res.err = run.StatusCancelled, cancelReason(rn.ctx)
			}
		}
		flush()
		rn.results <- res
	}()
}

// interrupted is the result of a step that never got to start.
func (rn *runner) interrupted(res stepResult) stepResult {
	if errors.Is(context.Cause(rn.ctx), errShutdown) {
		res.suspended = true
		return res
	}
	res.status, res.err = run.StatusCancelled, cancelReason(rn.ctx)
	return res
}

func (rn *runner) acquire() bool {
	select {
	case rn.slots <- struct{}{}:
		return true
	case <-rn.ctx.Done():
		return false
	}
}

func (rn *runner) release() { <-rn.slots }

func (rn *runner) saveProgress(stepID string, p run.Progress) {
	if err := rn.e.runs.UpdateStep(rn.store, rn.r.ID, stepID, p); err != nil {
		rn.log.Error("update step", "step", stepID, "err", err)
	}
}

// publishStep sends a step's state while it is still in progress.
func (rn *runner) publishStep(res stepResult, status run.Status) {
	rn.e.events.Publish(rn.r.ID, run.Event{Type: run.EventStep, Step: &run.StepRun{
		StepID: res.id, Status: status, Outputs: json.RawMessage("{}"),
		Config: res.config, StartedAt: res.startedAt, Progress: res.progress,
	}})
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
