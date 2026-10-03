package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"conductor/internal/run"
)

type Runs struct {
	db  *sql.DB
	now func() time.Time
}

func NewRuns(db *sql.DB) *Runs {
	return &Runs{db: db, now: func() time.Time { return time.Now().UTC() }}
}

var _ run.Repository = (*Runs)(nil)

func (s *Runs) Create(ctx context.Context, r run.Run) (run.Run, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return run.Run{}, fmt.Errorf("generate id: %w", err)
	}
	r.ID = id.String()
	r.Status = run.StatusQueued
	r.CreatedAt = s.now()

	def, err := json.Marshal(r.Definition)
	if err != nil {
		return run.Run{}, fmt.Errorf("encode definition: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return run.Run{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO runs (id, workflow_id, workflow_name, definition, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.WorkflowID, r.WorkflowName, string(def), r.Status, formatTime(r.CreatedAt))
	if err != nil {
		return run.Run{}, fmt.Errorf("insert run: %w", err)
	}

	r.Steps = make([]run.StepRun, len(r.Definition.Steps))
	for i, step := range r.Definition.Steps {
		r.Steps[i] = run.StepRun{StepID: step.ID, Status: run.StatusQueued, Outputs: json.RawMessage("{}")}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO step_runs (run_id, step_id, position, status) VALUES (?, ?, ?, ?)`,
			r.ID, step.ID, i, run.StatusQueued)
		if err != nil {
			return run.Run{}, fmt.Errorf("insert step run: %w", err)
		}
	}
	return r, tx.Commit()
}

func (s *Runs) Get(ctx context.Context, id string) (run.Run, error) {
	var r run.Run
	var def, created string
	var started, finished sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workflow_id, workflow_name, definition, status, error, created_at, started_at, finished_at
		FROM runs WHERE id = ?`, id,
	).Scan(&r.ID, &r.WorkflowID, &r.WorkflowName, &def, &r.Status, &r.Error, &created, &started, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return run.Run{}, run.ErrNotFound
	}
	if err != nil {
		return run.Run{}, fmt.Errorf("get run: %w", err)
	}
	if err := json.Unmarshal([]byte(def), &r.Definition); err != nil {
		return run.Run{}, fmt.Errorf("decode run definition: %w", err)
	}
	if r.CreatedAt, err = parseTime(created); err != nil {
		return run.Run{}, err
	}
	if r.StartedAt, err = parseNullTime(started); err != nil {
		return run.Run{}, err
	}
	if r.FinishedAt, err = parseNullTime(finished); err != nil {
		return run.Run{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT step_id, status, error, outputs, started_at, finished_at
		FROM step_runs WHERE run_id = ? ORDER BY position`, id)
	if err != nil {
		return run.Run{}, fmt.Errorf("get step runs: %w", err)
	}
	defer rows.Close()
	r.Steps = []run.StepRun{}
	for rows.Next() {
		var st run.StepRun
		var outputs string
		var started, finished sql.NullString
		if err := rows.Scan(&st.StepID, &st.Status, &st.Error, &outputs, &started, &finished); err != nil {
			return run.Run{}, fmt.Errorf("scan step run: %w", err)
		}
		st.Outputs = json.RawMessage(outputs)
		if st.StartedAt, err = parseNullTime(started); err != nil {
			return run.Run{}, err
		}
		if st.FinishedAt, err = parseNullTime(finished); err != nil {
			return run.Run{}, err
		}
		r.Steps = append(r.Steps, st)
	}
	return r, rows.Err()
}

func (s *Runs) List(ctx context.Context, f run.ListFilter) ([]run.Summary, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `
		SELECT id, workflow_id, workflow_name, status, error,
		       json_array_length(definition, '$.steps'), created_at, started_at, finished_at
		FROM runs`
	args := []any{}
	if f.WorkflowID != "" {
		query += ` WHERE workflow_id = ?`
		args = append(args, f.WorkflowID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	out := []run.Summary{}
	for rows.Next() {
		var r run.Summary
		var created string
		var started, finished sql.NullString
		if err := rows.Scan(&r.ID, &r.WorkflowID, &r.WorkflowName, &r.Status, &r.Error,
			&r.StepCount, &created, &started, &finished); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		if r.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if r.StartedAt, err = parseNullTime(started); err != nil {
			return nil, err
		}
		if r.FinishedAt, err = parseNullTime(finished); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Runs) StartRun(ctx context.Context, id string, at time.Time) error {
	return s.exec(ctx, `UPDATE runs SET status = ?, started_at = ? WHERE id = ?`,
		run.StatusRunning, formatTime(at), id)
}

func (s *Runs) FinishRun(ctx context.Context, id string, status run.Status, errMsg string, at time.Time) error {
	return s.exec(ctx, `UPDATE runs SET status = ?, error = ?, finished_at = ? WHERE id = ?`,
		status, errMsg, formatTime(at), id)
}

func (s *Runs) StartStep(ctx context.Context, runID, stepID string, at time.Time) error {
	return s.exec(ctx, `UPDATE step_runs SET status = ?, started_at = ? WHERE run_id = ? AND step_id = ?`,
		run.StatusRunning, formatTime(at), runID, stepID)
}

func (s *Runs) FinishStep(ctx context.Context, runID, stepID string, status run.Status, outputs json.RawMessage, errMsg string, at time.Time) error {
	if len(outputs) == 0 {
		outputs = json.RawMessage("{}")
	}
	return s.exec(ctx, `
		UPDATE step_runs SET status = ?, outputs = ?, error = ?, finished_at = ?
		WHERE run_id = ? AND step_id = ?`,
		status, string(outputs), errMsg, formatTime(at), runID, stepID)
}

func (s *Runs) AppendLogs(ctx context.Context, runID string, lines []run.LogLine) error {
	if len(lines) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO run_logs (run_id, step_id, stream, text, time) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, l := range lines {
		if _, err := stmt.ExecContext(ctx, runID, l.StepID, l.Stream, l.Text, formatTime(l.Time)); err != nil {
			return fmt.Errorf("insert log line: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Runs) Logs(ctx context.Context, runID, stepID string, after int64, limit int) ([]run.LogLine, error) {
	if limit <= 0 || limit > 10_000 {
		limit = 10_000
	}
	query := `SELECT seq, step_id, stream, text, time FROM run_logs WHERE run_id = ? AND seq > ?`
	args := []any{runID, after}
	if stepID != "" {
		query += ` AND step_id = ?`
		args = append(args, stepID)
	}
	query += ` ORDER BY seq LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get logs: %w", err)
	}
	defer rows.Close()

	out := []run.LogLine{}
	for rows.Next() {
		var l run.LogLine
		var t string
		if err := rows.Scan(&l.Seq, &l.StepID, &l.Stream, &l.Text, &t); err != nil {
			return nil, fmt.Errorf("scan log line: %w", err)
		}
		if l.Time, err = parseTime(t); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Runs) FailUnfinished(ctx context.Context, msg string, at time.Time) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	unfinished := []any{run.StatusQueued, run.StatusRunning}
	ts := formatTime(at)
	res, err := tx.ExecContext(ctx, `
		UPDATE runs SET status = ?, error = ?, finished_at = ? WHERE status IN (?, ?)`,
		append([]any{run.StatusFailed, msg, ts}, unfinished...)...)
	if err != nil {
		return 0, fmt.Errorf("fail unfinished runs: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE step_runs SET status = ?, error = ?, finished_at = ? WHERE status IN (?, ?)`,
		append([]any{run.StatusFailed, msg, ts}, unfinished...)...)
	if err != nil {
		return 0, fmt.Errorf("fail unfinished steps: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), tx.Commit()
}

func (s *Runs) exec(ctx context.Context, query string, args ...any) error {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", strings.Fields(query)[0], err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return run.ErrNotFound
	}
	return nil
}

func parseNullTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := parseTime(s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
