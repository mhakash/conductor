// Package run defines workflow runs, their steps and logs.
package run

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"conductor/internal/workflow"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	// StatusWaiting marks an approval step waiting for a decision, and a run
	// with nothing left to do until one is made.
	StatusWaiting Status = "waiting"
	// StatusSkipped marks steps that never ran: a dependency failed, their
	// condition was false, or everything they depend on was skipped.
	StatusSkipped Status = "skipped"
)

// Done reports whether the status is final.
func (s Status) Done() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusSkipped:
		return true
	}
	return false
}

// Run is one execution of a workflow. It keeps a snapshot of the definition it
// ran, so later edits to the workflow don't rewrite history.
type Run struct {
	ID           string              `json:"id"`
	WorkflowID   string              `json:"workflowId"`
	WorkflowName string              `json:"workflowName"`
	Definition   workflow.Definition `json:"definition"`
	// Inputs are the values this run was started with, after defaults.
	Inputs     map[string]any `json:"inputs"`
	Status     Status         `json:"status"`
	Error      string         `json:"error,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	StartedAt  *time.Time     `json:"startedAt"`
	FinishedAt *time.Time     `json:"finishedAt"`
	Steps      []StepRun      `json:"steps"`
}

type StepRun struct {
	StepID  string          `json:"stepId"`
	Status  Status          `json:"status"`
	Error   string          `json:"error,omitempty"`
	Outputs json.RawMessage `json:"outputs"`
	// Config is the step's config with expressions evaluated and secrets
	// masked; for a step that runs once per item, a list with one per item.
	Config     json.RawMessage `json:"config"`
	StartedAt  *time.Time      `json:"startedAt"`
	FinishedAt *time.Time      `json:"finishedAt"`
	Progress
}

// Progress is how far a running step has got.
type Progress struct {
	// Attempt counts tries of a step that retries; 0 until it starts.
	Attempt int `json:"attempt,omitempty"`
	// Items and ItemsDone count the elements of a step that runs once per item.
	Items     int `json:"items,omitempty"`
	ItemsDone int `json:"itemsDone,omitempty"`
}

// Summary is a run without its definition and steps, for listings.
type Summary struct {
	ID           string     `json:"id"`
	WorkflowID   string     `json:"workflowId"`
	WorkflowName string     `json:"workflowName"`
	Status       Status     `json:"status"`
	Error        string     `json:"error,omitempty"`
	StepCount    int        `json:"stepCount"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
}

type Stream string

const (
	StreamStdout Stream = "stdout"
	StreamStderr Stream = "stderr"
	// StreamSystem is Conductor's own commentary: "started", "timed out", ...
	StreamSystem Stream = "system"
)

type LogLine struct {
	// Seq orders lines within a run and serves as a cursor for incremental reads.
	Seq    int64     `json:"seq"`
	StepID string    `json:"stepId"`
	Stream Stream    `json:"stream"`
	Text   string    `json:"text"`
	Time   time.Time `json:"time"`
	// Item is the loop index for lines from a step that runs once per item.
	Item *int `json:"item,omitempty"`
}

type ListFilter struct {
	WorkflowID string
	Status     Status
	Limit      int
}

var ErrNotFound = errors.New("run not found")

type Repository interface {
	// Create stores a new run and a queued StepRun for every step.
	Create(ctx context.Context, r Run) (Run, error)
	Get(ctx context.Context, id string) (Run, error)
	List(ctx context.Context, f ListFilter) ([]Summary, error)

	StartRun(ctx context.Context, id string, at time.Time) error
	// SetRunStatus switches an unfinished run between running and waiting.
	SetRunStatus(ctx context.Context, id string, status Status) error
	FinishRun(ctx context.Context, id string, status Status, errMsg string, at time.Time) error
	// StartStep records that a step began (running, or waiting for approval).
	StartStep(ctx context.Context, runID, stepID string, status Status, config json.RawMessage, at time.Time) error
	UpdateStep(ctx context.Context, runID, stepID string, p Progress) error
	FinishStep(ctx context.Context, runID, stepID string, status Status, outputs json.RawMessage, errMsg string, at time.Time) error

	// AppendLogs stores lines and returns them with their Seq assigned.
	AppendLogs(ctx context.Context, runID string, lines []LogLine) ([]LogLine, error)
	// Logs returns lines with Seq > after, oldest first. stepID "" means all steps.
	Logs(ctx context.Context, runID, stepID string, after int64, limit int) ([]LogLine, error)

	// FailUnfinished marks runs left unfinished (e.g. by a crash) as failed
	// with msg, along with their unfinished steps. Runs with a step waiting
	// for approval are left alone, to be resumed. It returns how many runs it
	// failed.
	FailUnfinished(ctx context.Context, msg string, at time.Time) (int, error)
	// Unfinished lists the ids of runs that haven't finished, oldest first.
	Unfinished(ctx context.Context) ([]string, error)
}

// EventType names a change to a run, streamed to clients as it happens.
type EventType string

const (
	// EventSnapshot carries the full run and its logs so far; it is always
	// the first event on a stream, so later events can be applied as deltas.
	EventSnapshot EventType = "snapshot"
	EventRun      EventType = "run"
	EventStep     EventType = "step"
	EventLogs     EventType = "logs"
	// EventEnd says the run has finished and the stream will close.
	EventEnd EventType = "end"
)

// State is the run-level part of a Run, sent when the run starts or finishes.
type State struct {
	Status     Status     `json:"status"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// Event is one message on a run's stream. Step and log events carry complete
// values (a step's whole state, lines with their seq), so applying an event
// twice is harmless.
type Event struct {
	Type  EventType `json:"type"`
	Run   *Run      `json:"run,omitempty"`
	State *State    `json:"state,omitempty"`
	Step  *StepRun  `json:"step,omitempty"`
	Lines []LogLine `json:"lines,omitempty"`
}

// Publisher receives events as the engine records them.
type Publisher interface {
	Publish(runID string, e Event)
	// End delivers EventEnd and closes all streams for the run.
	End(runID string)
}
