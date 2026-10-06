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

	"conductor/internal/preset"
)

type Presets struct {
	db  *sql.DB
	now func() time.Time
}

func NewPresets(db *sql.DB) *Presets {
	return &Presets{db: db, now: func() time.Time { return time.Now().UTC() }}
}

var _ preset.Repository = (*Presets)(nil)

const presetColumns = `id, name, description, block, step, created_at, updated_at`

func (s *Presets) List(ctx context.Context) ([]preset.Preset, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+presetColumns+` FROM presets ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list presets: %w", err)
	}
	defer rows.Close()
	out := []preset.Preset{}
	for rows.Next() {
		p, err := scanPreset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Presets) Get(ctx context.Context, id string) (preset.Preset, error) {
	p, err := scanPreset(s.db.QueryRowContext(ctx, `SELECT `+presetColumns+` FROM presets WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return preset.Preset{}, preset.ErrNotFound
	}
	return p, err
}

func (s *Presets) Create(ctx context.Context, p preset.Preset) (preset.Preset, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return preset.Preset{}, fmt.Errorf("generate id: %w", err)
	}
	p.ID = id.String()
	p.CreatedAt = s.now()
	p.UpdatedAt = p.CreatedAt
	step, err := encodePresetStep(p.Step)
	if err != nil {
		return preset.Preset{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO presets (id, name, description, block, step, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.Block, step, formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		return preset.Preset{}, presetWriteErr("insert preset", err)
	}
	return p, nil
}

func (s *Presets) Update(ctx context.Context, p preset.Preset) (preset.Preset, error) {
	p.UpdatedAt = s.now()
	step, err := encodePresetStep(p.Step)
	if err != nil {
		return preset.Preset{}, err
	}
	var created string
	err = s.db.QueryRowContext(ctx, `
		UPDATE presets SET name = ?, description = ?, block = ?, step = ?, updated_at = ?
		WHERE id = ? RETURNING created_at`,
		p.Name, p.Description, p.Block, step, formatTime(p.UpdatedAt), p.ID).Scan(&created)
	if errors.Is(err, sql.ErrNoRows) {
		return preset.Preset{}, preset.ErrNotFound
	}
	if err != nil {
		return preset.Preset{}, presetWriteErr("update preset", err)
	}
	if p.CreatedAt, err = parseTime(created); err != nil {
		return preset.Preset{}, err
	}
	return p, nil
}

func (s *Presets) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM presets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete preset: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return preset.ErrNotFound
	}
	return nil
}

func encodePresetStep(st preset.Step) (string, error) {
	if len(st.Config) == 0 || string(st.Config) == "null" {
		st.Config = json.RawMessage("{}")
	}
	data, err := json.Marshal(st)
	if err != nil {
		return "", fmt.Errorf("encode preset: %w", err)
	}
	return string(data), nil
}

func presetWriteErr(op string, err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed: presets.name") {
		return preset.ErrNameTaken
	}
	return fmt.Errorf("%s: %w", op, err)
}

func scanPreset(row interface{ Scan(...any) error }) (preset.Preset, error) {
	var p preset.Preset
	var step, created, updated string
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Block, &step, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return p, err
		}
		return p, fmt.Errorf("scan preset: %w", err)
	}
	if err := json.Unmarshal([]byte(step), &p.Step); err != nil {
		return p, fmt.Errorf("decode preset %s: %w", p.ID, err)
	}
	var err error
	if p.CreatedAt, err = parseTime(created); err != nil {
		return p, err
	}
	p.UpdatedAt, err = parseTime(updated)
	return p, err
}
