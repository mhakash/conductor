package block

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

// maxCapture bounds how much stdout/stderr is kept as a step output. The full
// stream still goes to the run log.
const maxCapture = 1 << 20

// killGrace is how long a cancelled command gets to exit after SIGTERM.
const killGrace = 3 * time.Second

// OutputsEnv names the file a command may write JSON to; it becomes the
// step's "result" output.
const OutputsEnv = "CONDUCTOR_OUTPUTS"

// Paths inside a container: the step's own files, and the working directory
// when one is mounted.
const (
	containerFiles   = "/conductor"
	containerWorkdir = "/work"
)

// process is a program a step runs, on the host or in a container.
type process struct {
	// command builds the program and its arguments, given the directory
	// holding the step's files (the host or the container path).
	command func(files string) []string
	// hostShell runs command through a login shell (exec "$@") so PATH
	// matches the user's terminal. Off for commands that are a shell.
	hostShell bool
	// workdir is a host directory; "" means home, or nothing mounted in a
	// container.
	workdir string
	env     map[string]string
	// image runs the command in a fresh container of this image.
	image string
	// files are written to the step's directory before it runs.
	files map[string]string
	// describe is how the log introduces the command, and via what runs it
	// on the host (e.g. "zsh").
	describe, via string
}

// runProcess runs p, streaming its output to env, and returns stdout,
// stderr, exitCode and, if the command wrote any, result.
func runProcess(ctx context.Context, p process, env Env) (map[string]any, error) {
	files, err := os.MkdirTemp("", "conductor-step-*")
	if err != nil {
		return nil, fmt.Errorf("create step directory: %w", err)
	}
	defer os.RemoveAll(files)
	for name, content := range p.files {
		if err := os.WriteFile(filepath.Join(files, name), []byte(content), 0o600); err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
	}
	outputsFile := filepath.Join(files, "outputs.json")
	if err := os.WriteFile(outputsFile, nil, 0o600); err != nil {
		return nil, fmt.Errorf("create outputs file: %w", err)
	}

	var cmd *exec.Cmd
	var container string
	if p.image == "" {
		cmd, err = p.hostCommand(ctx, files, outputsFile)
		if err != nil {
			return nil, err
		}
		where := cmd.Dir
		if p.via != "" {
			where = p.via + " in " + where
		}
		env.Logf("$ %s  (%s)", p.describe, where)
	} else {
		container = "conductor-" + randomHex(6)
		cmd = p.containerCommand(ctx, files, container)
		where := "nothing mounted"
		if p.workdir != "" {
			where = p.workdir + " at " + containerWorkdir
		}
		env.Logf("$ %s  (in %s, %s)", p.describe, p.image, where)
	}
	configureProcessGroup(cmd)
	if container != "" {
		// Stopping the docker client doesn't stop its container.
		stop := cmd.Cancel
		cmd.Cancel = func() error {
			removeContainer(container)
			return stop()
		}
	}
	cmd.WaitDelay = killGrace

	var stdout, stderr capped
	cmd.Stdout = io.MultiWriter(env.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(env.Stderr, &stderr)
	err = cmd.Run()

	outputs := map[string]any{
		"stdout":   strings.TrimSpace(stdout.String()),
		"stderr":   strings.TrimSpace(stderr.String()),
		"exitCode": cmd.ProcessState.ExitCode(),
	}
	result, resultErr := readResult(outputsFile)
	if result != nil {
		outputs["result"] = result
	}
	if err == nil {
		return outputs, resultErr
	}
	if ctx.Err() != nil {
		return outputs, ctx.Err()
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return outputs, err
	}
	switch code := exitErr.ExitCode(); {
	case container != "" && code == 125:
		return outputs, errors.New("Docker couldn't start the container; see the log")
	case code == 127 && p.hostShell:
		return outputs, fmt.Errorf("%s wasn't found; is it installed and on your PATH?", p.command(files)[0])
	default:
		return outputs, fmt.Errorf("exited with code %d", code)
	}
}

func (p process) hostCommand(ctx context.Context, files, outputsFile string) (*exec.Cmd, error) {
	dir, err := expandHome(p.workdir)
	if err != nil {
		return nil, err
	}
	argv := p.command(files)
	if p.hostShell {
		argv = loginExec(argv...)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), OutputsEnv+"="+outputsFile)
	for k, v := range p.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return cmd, nil
}

// containerCommand runs p with `docker run`. Environment values are handed
// to the docker client in its own environment and passed on by name, so
// secrets don't appear on its command line.
func (p process) containerCommand(ctx context.Context, files, name string) *exec.Cmd {
	args := []string{"docker", "run", "--rm", "--init", "--name", name,
		"-v", files + ":" + containerFiles, "-e", OutputsEnv}
	if p.workdir != "" {
		dir, _ := expandHome(p.workdir)
		args = append(args, "-v", dir+":"+containerWorkdir, "-w", containerWorkdir)
	} else {
		args = append(args, "-w", containerFiles)
	}
	cmdEnv := append(os.Environ(), OutputsEnv+"="+containerFiles+"/outputs.json")
	for k, v := range p.env {
		args = append(args, "-e", k)
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	args = append(args, p.image)
	args = append(args, p.command(containerFiles)...)

	argv := loginExec(args...)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = cmdEnv
	return cmd
}

// loginExec runs argv through the user's login shell, for its PATH.
func loginExec(argv ...string) []string {
	return append([]string{loginShell(), "-l", "-c", `exec "$@"`, "conductor"}, argv...)
}

// removeContainer force-removes a container whose step was stopped.
func removeContainer(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	argv := loginExec("docker", "rm", "-f", name)
	_ = exec.CommandContext(ctx, argv[0], argv[1:]...).Run()
}

// readResult parses what the command wrote to its outputs file, if anything.
func readResult(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("what was written to $%s isn't JSON: %w", OutputsEnv, err)
	}
	return v, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
