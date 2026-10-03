package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"conductor/internal/workflow"
)

type Workflows struct {
	db  *sql.DB
	now func() time.Time
}

func NewWorkflows(db *sql.DB) *Workflows {
	return &Workflows{db: db, now: func() time.Time { return time.Now().UTC() }}
}

var _ workflow.Repository = (*Workflows)(nil)

func (s *Workflows) List(ctx context.Context) ([]workflow.Summary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, json_array_length(definition, '$.steps'), created_at, updated_at
		FROM workflows
		ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	defer rows.Close()

	out := []workflow.Summary{}
	for rows.Next() {
		var w workflow.Summary
		var created, updated string
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.StepCount, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		if w.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if w.UpdatedAt, err = parseTime(updated); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Workflows) Get(ctx context.Context, id string) (workflow.Workflow, error) {
	var w workflow.Workflow
	var def, created, updated string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, definition, created_at, updated_at
		FROM workflows WHERE id = ?`, id,
	).Scan(&w.ID, &w.Name, &w.Description, &def, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return workflow.Workflow{}, workflow.ErrNotFound
	}
	if err != nil {
		return workflow.Workflow{}, fmt.Errorf("get workflow: %w", err)
	}
	if err := json.Unmarshal([]byte(def), &w.Definition); err != nil {
		return workflow.Workflow{}, fmt.Errorf("decode definition of %s: %w", id, err)
	}
	if w.CreatedAt, err = parseTime(created); err != nil {
		return workflow.Workflow{}, err
	}
	if w.UpdatedAt, err = parseTime(updated); err != nil {
		return workflow.Workflow{}, err
	}
	return w, nil
}

// Create assigns an ID and timestamps and inserts w.
func (s *Workflows) Create(ctx context.Context, w workflow.Workflow) (workflow.Workflow, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return workflow.Workflow{}, fmt.Errorf("generate id: %w", err)
	}
	w.ID = id.String()
	w.CreatedAt = s.now()
	w.UpdatedAt = w.CreatedAt

	def, err := json.Marshal(w.Definition)
	if err != nil {
		return workflow.Workflow{}, fmt.Errorf("encode definition: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO workflows (id, name, description, definition, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		w.ID, w.Name, w.Description, string(def), formatTime(w.CreatedAt), formatTime(w.UpdatedAt))
	if err != nil {
		return workflow.Workflow{}, fmt.Errorf("insert workflow: %w", err)
	}
	return w, nil
}

// Update replaces name, description and definition of an existing workflow.
func (s *Workflows) Update(ctx context.Context, w workflow.Workflow) (workflow.Workflow, error) {
	def, err := json.Marshal(w.Definition)
	if err != nil {
		return workflow.Workflow{}, fmt.Errorf("encode definition: %w", err)
	}
	w.UpdatedAt = s.now()

	var created string
	err = s.db.QueryRowContext(ctx, `
		UPDATE workflows SET name = ?, description = ?, definition = ?, updated_at = ?
		WHERE id = ?
		RETURNING created_at`,
		w.Name, w.Description, string(def), formatTime(w.UpdatedAt), w.ID,
	).Scan(&created)
	if errors.Is(err, sql.ErrNoRows) {
		return workflow.Workflow{}, workflow.ErrNotFound
	}
	if err != nil {
		return workflow.Workflow{}, fmt.Errorf("update workflow: %w", err)
	}
	if w.CreatedAt, err = parseTime(created); err != nil {
		return workflow.Workflow{}, err
	}
	return w, nil
}

func (s *Workflows) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM workflows WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return workflow.ErrNotFound
	}
	return nil
}

// Timestamps are stored as RFC 3339 text with nanoseconds so they sort
// lexically and keep their precision.
func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t, nil
}
