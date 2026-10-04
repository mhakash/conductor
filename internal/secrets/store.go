package secrets

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"
)

// Secret describes a stored secret. Its value is never part of it.
type Secret struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

var (
	ErrNotFound = errors.New("secret not found")
	// Names look like environment variables, which is also how they tend to be used.
	namePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
)

func ValidName(name string) bool { return namePattern.MatchString(name) }

type Store struct {
	db     *sql.DB
	cipher *Cipher
	now    func() time.Time
}

func NewStore(db *sql.DB, c *Cipher) *Store {
	return &Store{db: db, cipher: c, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Store) List(ctx context.Context) ([]Secret, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, created_at, updated_at FROM secrets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer rows.Close()
	out := []Secret{}
	for rows.Next() {
		var sec Secret
		var created, updated string
		if err := rows.Scan(&sec.Name, &created, &updated); err != nil {
			return nil, err
		}
		if sec.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, err
		}
		if sec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
			return nil, err
		}
		out = append(out, sec)
	}
	return out, rows.Err()
}

// Set creates or replaces a secret's value.
func (s *Store) Set(ctx context.Context, name, value string) (Secret, error) {
	if !ValidName(name) {
		return Secret{}, fmt.Errorf("secret name %q must be UPPER_SNAKE_CASE", name)
	}
	if value == "" {
		return Secret{}, errors.New("secret value is empty")
	}
	data, err := s.cipher.Seal(name, value)
	if err != nil {
		return Secret{}, err
	}
	now := s.now().Format(time.RFC3339Nano)
	var created string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO secrets (name, ciphertext, created_at, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET ciphertext = excluded.ciphertext, updated_at = excluded.updated_at
		RETURNING created_at`, name, data, now, now).Scan(&created)
	if err != nil {
		return Secret{}, fmt.Errorf("save secret: %w", err)
	}
	c, _ := time.Parse(time.RFC3339Nano, created)
	u, _ := time.Parse(time.RFC3339Nano, now)
	return Secret{Name: name, CreatedAt: c, UpdatedAt: u}, nil
}

func (s *Store) Delete(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM secrets WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Values decrypts the named secrets. Missing names are reported together.
func (s *Store) Values(ctx context.Context, names []string) (map[string]string, error) {
	out := make(map[string]string, len(names))
	var missing []string
	for _, name := range names {
		if _, done := out[name]; done {
			continue
		}
		var data []byte
		err := s.db.QueryRowContext(ctx, `SELECT ciphertext FROM secrets WHERE name = ?`, name).Scan(&data)
		if errors.Is(err, sql.ErrNoRows) {
			missing = append(missing, name)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read secret: %w", err)
		}
		if out[name], err = s.cipher.Open(name, data); err != nil {
			return nil, err
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, &MissingError{Names: slices.Compact(missing)}
	}
	return out, nil
}

type MissingError struct{ Names []string }

func (e *MissingError) Error() string {
	return fmt.Sprintf("missing secrets: %v", e.Names)
}
