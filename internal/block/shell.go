package block

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type shellConfig struct {
	Command string            `json:"command"`
	Workdir string            `json:"workdir"`
	Shell   string            `json:"shell"`
	Env     map[string]string `json:"env"`
}

// maxCapture bounds how much stdout/stderr is kept as a step output. The full
// stream still goes to the run log.
const maxCapture = 1 << 20

// killGrace is how long a cancelled command gets to exit after SIGTERM.
const killGrace = 3 * time.Second

func runShell(ctx context.Context, raw json.RawMessage, env Env) (map[string]any, error) {
	var cfg shellConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	shell := cfg.Shell
	if shell == "" {
		shell = loginShell()
	}
	dir, err := expandHome(cfg.Workdir)
	if err != nil {
		return nil, err
	}

	// -l loads the login profile so PATH and tool setup match the user's terminal.
	cmd := exec.CommandContext(ctx, shell, "-l", "-c", cfg.Command)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	configureProcessGroup(cmd)
	cmd.WaitDelay = killGrace

	var stdout, stderr capped
	cmd.Stdout = io.MultiWriter(env.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(env.Stderr, &stderr)

	env.Logf("$ %s  (%s in %s)", firstLine(cfg.Command), filepath.Base(shell), dir)
	err = cmd.Run()

	outputs := map[string]any{
		"stdout":   strings.TrimSpace(stdout.String()),
		"stderr":   strings.TrimSpace(stderr.String()),
		"exitCode": cmd.ProcessState.ExitCode(),
	}
	if err == nil {
		return outputs, nil
	}
	if ctx.Err() != nil {
		return outputs, ctx.Err()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return outputs, fmt.Errorf("command exited with code %d", exitErr.ExitCode())
	}
	return outputs, err
}

func loginShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

func expandHome(dir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	switch {
	case dir == "" || dir == "~":
		return home, nil
	case strings.HasPrefix(dir, "~/"):
		return filepath.Join(home, dir[2:]), nil
	}
	return dir, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
}

// capped is a buffer that silently stops growing at maxCapture bytes.
type capped struct {
	buf       []byte
	truncated bool
}

func (c *capped) Write(p []byte) (int, error) {
	if room := maxCapture - len(c.buf); room > 0 {
		c.buf = append(c.buf, p[:min(room, len(p))]...)
	}
	if len(c.buf) >= maxCapture {
		c.truncated = true
	}
	return len(p), nil
}

func (c *capped) String() string {
	if c.truncated {
		return string(c.buf) + "\n… (truncated)"
	}
	return string(c.buf)
}
