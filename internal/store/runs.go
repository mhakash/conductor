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
	if r.Inputs == nil {
		r.Inputs = map[string]any{}
	}
	inputs, err := json.Marshal(r.Inputs)
	if err != nil {
		return run.Run{}, fmt.Errorf("encode inputs: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return run.Run{}, err
	}
	defer tx.Rollback()

	var parentRun, parentStep sql.NullString
	var parentItem sql.NullInt64
	if p := r.Parent; p != nil {
		parentRun = sql.NullString{String: p.RunID, Valid: true}
		parentStep = sql.NullString{String: p.StepID, Valid: true}
		if p.Item != nil {
			parentItem = sql.NullInt64{Int64: int64(*p.Item), Valid: true}
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO runs (id, workflow_id, workflow_name, definition, inputs, status, created_at,
		                  parent_run_id, parent_step_id, parent_item)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.WorkflowID, r.WorkflowName, string(def), string(inputs), r.Status, formatTime(r.CreatedAt),
		parentRun, parentStep, parentItem)
	if err != nil {
		return run.Run{}, fmt.Errorf("insert run: %w", err)
	}

	r.Outputs = map[string]any{}
	r.Children = []run.Summary{}
	r.Steps = make([]run.StepRun, len(r.Definition.Steps))
	for i, step := range r.Definition.Steps {
		r.Steps[i] = run.StepRun{StepID: step.ID, Status: run.StatusQueued,
			Outputs: json.RawMessage("{}"), Config: json.RawMessage("{}")}
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
	var def, inputs, outputs, created string
	var started, finished sql.NullString
	var parent parentCols
	err := s.db.QueryRowContext(ctx, `
		SELECT runs.id, runs.workflow_id, runs.workflow_name, runs.definition, runs.inputs, runs.outputs,
		       runs.status, runs.error, runs.created_at, runs.started_at, runs.finished_at, `+parentSelect+`
		FROM runs `+parentJoin+` WHERE runs.id = ?`, id,
	).Scan(&r.ID, &r.WorkflowID, &r.WorkflowName, &def, &inputs, &outputs, &r.Status, &r.Error, &created, &started, &finished,
		&parent.run, &parent.workflow, &parent.step, &parent.item)
	if errors.Is(err, sql.ErrNoRows) {
		return run.Run{}, run.ErrNotFound
	}
	if err != nil {
		return run.Run{}, fmt.Errorf("get run: %w", err)
	}
	if err := json.Unmarshal([]byte(def), &r.Definition); err != nil {
		return run.Run{}, fmt.Errorf("decode run definition: %w", err)
	}
	if err := json.Unmarshal([]byte(inputs), &r.Inputs); err != nil {
		return run.Run{}, fmt.Errorf("decode run inputs: %w", err)
	}
	if err := json.Unmarshal([]byte(outputs), &r.Outputs); err != nil {
		return run.Run{}, fmt.Errorf("decode run outputs: %w", err)
	}
	r.Parent = parent.value()
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
		SELECT step_id, status, error, outputs, config, started_at, finished_at, attempt, items, items_done
		FROM step_runs WHERE run_id = ? ORDER BY position`, id)
	if err != nil {
		return run.Run{}, fmt.Errorf("get step runs: %w", err)
	}
	defer rows.Close()
	r.Steps = []run.StepRun{}
	for rows.Next() {
		var st run.StepRun
		var outputs, config string
		var started, finished sql.NullString
		if err := rows.Scan(&st.StepID, &st.Status, &st.Error, &outputs, &config, &started, &finished,
			&st.Attempt, &st.Items, &st.ItemsDone); err != nil {
			return run.Run{}, fmt.Errorf("scan step run: %w", err)
		}
		st.Outputs = json.RawMessage(outputs)
		st.Config = json.RawMessage(config)
		if st.StartedAt, err = parseNullTime(started); err != nil {
			return run.Run{}, err
		}
		if st.FinishedAt, err = parseNullTime(finished); err != nil {
			return run.Run{}, err
		}
		r.Steps = append(r.Steps, st)
	}
	if err := rows.Err(); err != nil {
		return run.Run{}, err
	}
	r.Children, err = s.Children(ctx, id)
	return r, err
}

// parentSelect and parentJoin read a run's parent; the join supplies the
// parent's workflow name.
const (
	parentSelect = `runs.parent_run_id, parent.workflow_name, runs.parent_step_id, runs.parent_item`
	parentJoin   = `LEFT JOIN runs AS parent ON parent.id = runs.parent_run_id`
)

type parentCols struct {
	run, workflow, step sql.NullString
	item                sql.NullInt64
}

func (p parentCols) value() *run.Parent {
	if !p.run.Valid {
		return nil
	}
	out := &run.Parent{RunID: p.run.String, WorkflowName: p.workflow.String, StepID: p.step.String}
	if p.item.Valid {
		i := int(p.item.Int64)
		out.Item = &i
	}
	return out
}

func (s *Runs) List(ctx context.Context, f run.ListFilter) ([]run.Summary, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var where []string
	args := []any{}
	if f.WorkflowID != "" {
		where = append(where, `runs.workflow_id = ?`)
		args = append(args, f.WorkflowID)
	}
	if f.Status != "" {
		where = append(where, `runs.status = ?`)
		args = append(args, f.Status)
	}
	if f.TopLevel {
		where = append(where, `runs.parent_run_id IS NULL`)
	}
	query := ""
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	query += ` ORDER BY runs.created_at DESC LIMIT ?`
	args = append(args, limit)
	return s.summaries(ctx, query, args...)
}

func (s *Runs) Children(ctx context.Context, id string) ([]run.Summary, error) {
	return s.summaries(ctx, ` WHERE runs.parent_run_id = ? ORDER BY runs.created_at`, id)
}

// summaries lists runs; tail is the WHERE/ORDER BY part of the query.
func (s *Runs) summaries(ctx context.Context, tail string, args ...any) ([]run.Summary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT runs.id, runs.workflow_id, runs.workflow_name, runs.status, runs.error,
		       json_array_length(runs.definition, '$.steps'), runs.created_at, runs.started_at, runs.finished_at,
		       `+parentSelect+`
		FROM runs `+parentJoin+tail, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	out := []run.Summary{}
	for rows.Next() {
		var r run.Summary
		var created string
		var started, finished sql.NullString
		var parent parentCols
		if err := rows.Scan(&r.ID, &r.WorkflowID, &r.WorkflowName, &r.Status, &r.Error,
			&r.StepCount, &created, &started, &finished,
			&parent.run, &parent.workflow, &parent.step, &parent.item); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		r.Parent = parent.value()
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

func (s *Runs) SetRunStatus(ctx context.Context, id string, status run.Status) error {
	return s.exec(ctx, `UPDATE runs SET status = ? WHERE id = ?`, status, id)
}

func (s *Runs) FinishRun(ctx context.Context, id string, status run.Status, outputs map[string]any, errMsg string, at time.Time) error {
	if outputs == nil {
		outputs = map[string]any{}
	}
	data, err := json.Marshal(outputs)
	if err != nil {
		return fmt.Errorf("encode outputs: %w", err)
	}
	return s.exec(ctx, `UPDATE runs SET status = ?, outputs = ?, error = ?, finished_at = ? WHERE id = ?`,
		status, string(data), errMsg, formatTime(at), id)
}

func (s *Runs) StartStep(ctx context.Context, runID, stepID string, status run.Status, config json.RawMessage, at time.Time) error {
	if len(config) == 0 {
		config = json.RawMessage("{}")
	}
	return s.exec(ctx, `
		UPDATE step_runs SET status = ?, config = ?, started_at = ? WHERE run_id = ? AND step_id = ?`,
		status, string(config), formatTime(at), runID, stepID)
}

func (s *Runs) UpdateStep(ctx context.Context, runID, stepID string, p run.Progress) error {
	return s.exec(ctx, `
		UPDATE step_runs SET attempt = ?, items = ?, items_done = ? WHERE run_id = ? AND step_id = ?`,
		p.Attempt, p.Items, p.ItemsDone, runID, stepID)
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

func (s *Runs) AppendLogs(ctx context.Context, runID string, lines []run.LogLine) ([]run.LogLine, error) {
	if len(lines) == 0 {
		return nil, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO run_logs (run_id, step_id, stream, text, time, item) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	out := make([]run.LogLine, len(lines))
	for i, l := range lines {
		res, err := stmt.ExecContext(ctx, runID, l.StepID, l.Stream, l.Text, formatTime(l.Time), l.Item)
		if err != nil {
			return nil, fmt.Errorf("insert log line: %w", err)
		}
		if l.Seq, err = res.LastInsertId(); err != nil {
			return nil, err
		}
		out[i] = l
	}
	return out, tx.Commit()
}

func (s *Runs) Logs(ctx context.Context, runID, stepID string, after int64, limit int) ([]run.LogLine, error) {
	if limit <= 0 || limit > 10_000 {
		limit = 10_000
	}
	query := `SELECT seq, step_id, stream, text, time, item FROM run_logs WHERE run_id = ? AND seq > ?`
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
		var item sql.NullInt64
		if err := rows.Scan(&l.Seq, &l.StepID, &l.Stream, &l.Text, &t, &item); err != nil {
			return nil, fmt.Errorf("scan log line: %w", err)
		}
		if item.Valid {
			i := int(item.Int64)
			l.Item = &i
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

	// Runs to fail: unfinished, with no step waiting for approval, in them
	// or in a run they started (at any depth).
	const doomed = `
		WITH RECURSIVE keep (id) AS (
			SELECT run_id FROM step_runs WHERE status = 'waiting'
			UNION
			SELECT runs.parent_run_id FROM runs JOIN keep ON runs.id = keep.id
			WHERE runs.parent_run_id IS NOT NULL
		)
		SELECT id FROM runs WHERE status IN ('queued', 'running', 'waiting')
		AND id NOT IN (SELECT id FROM keep)`
	ts := formatTime(at)
	_, err = tx.ExecContext(ctx, `
		UPDATE step_runs SET status = ?, error = ?, finished_at = ?
		WHERE status IN ('queued', 'running') AND run_id IN (`+doomed+`)`,
		run.StatusFailed, msg, ts)
	if err != nil {
		return 0, fmt.Errorf("fail unfinished steps: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE runs SET status = ?, error = ?, finished_at = ? WHERE id IN (`+doomed+`)`,
		run.StatusFailed, msg, ts)
	if err != nil {
		return 0, fmt.Errorf("fail unfinished runs: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), tx.Commit()
}

func (s *Runs) Unfinished(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM runs WHERE status IN ('queued', 'running', 'waiting') ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list unfinished runs: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
