package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"conductor/internal/workflow"
)

func openTestDB(t *testing.T) *Workflows {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewWorkflows(db)
}

func TestWorkflowsCRUD(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t)

	def := workflow.EmptyDefinition()
	def.Steps = append(def.Steps, workflow.Step{
		ID: "tag", Block: "shell", Name: "Create tag",
		Config:   json.RawMessage(`{"command":"git tag v1"}`),
		Position: workflow.Position{X: 10, Y: 20},
	})

	created, err := s.Create(ctx, workflow.Workflow{Name: "Build prep", Definition: def})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("create did not assign id/timestamps: %+v", created)
	}

	got, err := s.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Build prep" || len(got.Definition.Steps) != 1 || got.Definition.Steps[0].Position.Y != 20 {
		t.Errorf("get returned %+v", got)
	}
	if string(got.Definition.Steps[0].Config) != `{"command":"git tag v1"}` {
		t.Errorf("config round-trip = %s", got.Definition.Steps[0].Config)
	}

	got.Name = "Release prep"
	updated, err := s.Update(ctx, got, &got.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("update changed created_at: %v → %v", created.CreatedAt, updated.CreatedAt)
	}

	// A second writer still holding the old version is refused.
	if _, err := s.Update(ctx, got, &got.UpdatedAt); !errors.Is(err, workflow.ErrConflict) {
		t.Errorf("stale update: %v, want ErrConflict", err)
	}
	// Saving on top of the current version is fine.
	if _, err := s.Update(ctx, updated, &updated.UpdatedAt); err != nil {
		t.Errorf("update with current version: %v", err)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Release prep" || list[0].StepCount != 1 {
		t.Errorf("list = %+v", list)
	}

	if err := s.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, created.ID); !errors.Is(err, workflow.ErrNotFound) {
		t.Errorf("get after delete: %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, created.ID); !errors.Is(err, workflow.ErrNotFound) {
		t.Errorf("second delete: %v, want ErrNotFound", err)
	}
	if _, err := s.Update(ctx, got, nil); !errors.Is(err, workflow.ErrNotFound) {
		t.Errorf("update of deleted: %v, want ErrNotFound", err)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for range 2 {
		db, err := Open(context.Background(), path, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}
