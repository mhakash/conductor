// Package config loads runtime settings from flags and environment variables.
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	// Addr is the listen address for the HTTP server.
	Addr string
	// DataDir holds the database and run artifacts.
	DataDir string
	// Dev disables serving the embedded UI; Vite serves it instead.
	Dev bool
}

// Load parses args (without the program name). Environment variables provide
// defaults so the binary works with zero flags.
func Load(args []string) (Config, error) {
	defaultDataDir, err := defaultDataDir()
	if err != nil {
		return Config{}, err
	}

	fs := flag.NewFlagSet("conductor", flag.ContinueOnError)
	cfg := Config{}
	fs.StringVar(&cfg.Addr, "addr", envOr("CONDUCTOR_ADDR", "127.0.0.1:7070"), "HTTP listen address")
	fs.StringVar(&cfg.DataDir, "data-dir", envOr("CONDUCTOR_DATA_DIR", defaultDataDir), "directory for the database and run artifacts")
	fs.BoolVar(&cfg.Dev, "dev", os.Getenv("CONDUCTOR_DEV") == "1", "development mode (UI served by Vite)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data dir: %w", err)
	}
	return cfg, nil
}

func defaultDataDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "conductor"), nil
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
