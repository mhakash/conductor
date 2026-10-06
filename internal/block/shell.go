package block

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type shellConfig struct {
	Command string            `json:"command"`
	Workdir string            `json:"workdir"`
	Shell   string            `json:"shell"`
	Env     map[string]string `json:"env"`
	Image   string            `json:"image"`
}

func runShell(ctx context.Context, raw json.RawMessage, env Env) (map[string]any, error) {
	var cfg shellConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	p := process{workdir: cfg.Workdir, env: cfg.Env, image: cfg.Image}
	if cfg.Image != "" {
		// Images may not have bash or the user's shell; sh is everywhere.
		p.command = func(string) []string { return []string{"sh", "-c", cfg.Command} }
		p.describe = firstLine(cfg.Command)
	} else {
		shell := cfg.Shell
		if shell == "" {
			shell = loginShell()
		}
		// -l loads the login profile so PATH and tool setup match the user's terminal.
		p.command = func(string) []string { return []string{shell, "-l", "-c", cfg.Command} }
		p.describe, p.via = firstLine(cfg.Command), filepath.Base(shell)
	}
	return runProcess(ctx, p, env)
}
