//go:build unix

package block

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup runs the command in its own process group and makes
// cancellation signal the whole group, so `sleep 100 && …` or a command's
// own children don't outlive the step.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}
