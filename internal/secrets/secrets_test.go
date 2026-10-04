package secrets_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"conductor/internal/secrets"
	"conductor/internal/store"
)

func newCipher(t *testing.T) *secrets.Cipher {
	t.Helper()
	key := make([]byte, secrets.KeySize)
	_, _ = rand.Read(key)
	c, err := secrets.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCipher(t *testing.T) {
	c := newCipher(t)
	data, err := c.Seal("GITLAB_TOKEN", "glpat-secret")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("glpat-secret")) {
		t.Fatal("ciphertext contains the plaintext")
	}
	if got, err := c.Open("GITLAB_TOKEN", data); err != nil || got != "glpat-secret" {
		t.Errorf("Open = %q, %v", got, err)
	}
	if _, err := c.Open("JIRA_TOKEN", data); err == nil {
		t.Error("ciphertext opened under a different name")
	}
	if _, err := newCipher(t).Open("GITLAB_TOKEN", data); err == nil {
		t.Error("ciphertext opened with a different key")
	}
}

func TestStore(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := secrets.NewStore(db, newCipher(t))

	if _, err := s.Set(ctx, "gitlab-token", "x"); err == nil {
		t.Error("accepted a lower-case name")
	}
	first, err := s.Set(ctx, "GITLAB_TOKEN", "one")
	if err != nil {
		t.Fatal(err)
	}
	second, _ := s.Set(ctx, "GITLAB_TOKEN", "two")
	if !second.CreatedAt.Equal(first.CreatedAt) || !second.UpdatedAt.After(first.UpdatedAt) {
		t.Errorf("replace changed created_at or kept updated_at: %+v %+v", first, second)
	}

	vals, err := s.Values(ctx, []string{"GITLAB_TOKEN"})
	if err != nil || vals["GITLAB_TOKEN"] != "two" {
		t.Errorf("Values = %v, %v", vals, err)
	}
	var missing *secrets.MissingError
	if _, err := s.Values(ctx, []string{"GITLAB_TOKEN", "JIRA", "JIRA"}); !errors.As(err, &missing) || len(missing.Names) != 1 {
		t.Errorf("missing = %v", err)
	}

	list, _ := s.List(ctx)
	if len(list) != 1 || list[0].Name != "GITLAB_TOKEN" {
		t.Errorf("list = %+v", list)
	}
	if err := s.Delete(ctx, "GITLAB_TOKEN"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "GITLAB_TOKEN"); !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
}

func TestMasker(t *testing.T) {
	// A value that contains another must be masked whole, not in pieces.
	m := secrets.NewMasker([]string{"open-sesame", "open-sesame-please", "ab"})
	got := m.Mask("long: open-sesame-please, short: open-sesame, tiny: ab")
	want := "long: " + secrets.Mask + ", short: " + secrets.Mask + ", tiny: ab"
	if got != want {
		t.Errorf("Mask = %q, want %q", got, want)
	}
	quoted := secrets.NewMasker([]string{`pa"ss\word`})
	if got := quoted.Mask(`{"pw":"pa\"ss\\word"}`); got != `{"pw":"`+secrets.Mask+`"}` {
		t.Errorf("JSON-escaped secret not masked: %s", got)
	}
	var nilMasker *secrets.Masker
	if nilMasker.Mask("x") != "x" {
		t.Error("nil masker changed text")
	}
}
