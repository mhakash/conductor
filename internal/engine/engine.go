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
	"conductor/internal/workflow"
)

// DefaultStepTimeout applies to steps that don't set their own.
const DefaultStepTimeout = time.Hour

var ErrNotActive = errors.New("run is not active")

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
	// MaxParallel caps how many steps of one run execute at the same time.
	MaxParallel int
}

type Engine struct {
	runs        run.Repository
	events      run.Publisher
	blocks      *block.Registry
	log         *slog.Logger
	maxParallel int
	now         func() time.Time

	mu     sync.Mutex
	active map[string]context.CancelCauseFunc
	wg     sync.WaitGroup
	closed bool
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
		blocks:      o.Blocks,
		log:         o.Log,
		maxParallel: o.MaxParallel,
		now:         func() time.Time { return time.Now().UTC() },
		active:      map[string]context.CancelCauseFunc{},
	}
}

// Start validates wf, records a new run and executes it in the background.
// It returns a *workflow.ValidationError if the workflow can't run as is.
func (e *Engine) Start(ctx context.Context, wf workflow.Workflow) (run.Run, error) {
	if err := e.Check(wf); err != nil {
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
	})
	if err != nil {
		return run.Run{}, err
	}

	// The run outlives the HTTP request that started it.
	runCtx, cancel := context.WithCancelCause(context.Background())
	e.active[r.ID] = cancel
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			delete(e.active, r.ID)
			e.mu.Unlock()
			cancel(nil)
		}()
		e.execute(runCtx, r)
	}()
	return r, nil
}

// Cancel stops a running run. Running steps are interrupted; steps that
// haven't started are marked cancelled.
func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	cancel, ok := e.active[id]
	if !ok {
		return ErrNotActive
	}
	cancel(errCancelled)
	return nil
}

// Shutdown cancels all active runs and waits for them to record their final
// state, or for ctx to expire.
func (e *Engine) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	e.closed = true
	for _, cancel := range e.active {
		cancel(errShutdown)
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
// problems, unknown blocks and missing required config.
func (e *Engine) Check(wf workflow.Workflow) error {
	if err := workflow.Validate(wf, e.blocks.Has); err != nil {
		return err
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
		if b.Run == nil {
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
