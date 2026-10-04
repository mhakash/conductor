package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "conductor"
	keyringUser    = "secret-encryption-key"
	keyEnv         = "CONDUCTOR_SECRET_KEY"
	keyFile        = "secret.key"
)

// LoadKey returns the encryption key, creating one on first use. It looks,
// in order, at the CONDUCTOR_SECRET_KEY environment variable (base64), the
// OS keychain, and — only where no keychain is available — a 0600 file in
// dataDir.
func LoadKey(dataDir string, log *slog.Logger) ([]byte, error) {
	if v := os.Getenv(keyEnv); v != "" {
		key, err := decodeKey(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", keyEnv, err)
		}
		return key, nil
	}

	encoded, err := keyring.Get(keyringService, keyringUser)
	switch {
	case err == nil:
		return decodeKey(encoded)
	case errors.Is(err, keyring.ErrNotFound):
		key, encoded, err := newKey()
		if err != nil {
			return nil, err
		}
		if err := keyring.Set(keyringService, keyringUser, encoded); err == nil {
			log.Info("created secret encryption key in the OS keychain", "service", keyringService)
			return key, nil
		} else {
			log.Warn("keychain unavailable, storing the secret key in a file", "err", err)
		}
	default:
		log.Warn("keychain unavailable, storing the secret key in a file", "err", err)
	}
	return fileKey(filepath.Join(dataDir, keyFile))
}

func fileKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return decodeKey(strings.TrimSpace(string(data)))
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key, encoded, err := newKey()
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(encoded+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write secret key: %w", err)
	}
	return key, nil
}

func newKey() ([]byte, string, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, "", err
	}
	return key, base64.StdEncoding.EncodeToString(key), nil
}

func decodeKey(s string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("secret key is not valid base64: %w", err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("secret key must be %d bytes, got %d", KeySize, len(key))
	}
	return key, nil
}
