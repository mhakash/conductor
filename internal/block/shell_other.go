//go:build !unix

package block

import "os/exec"

// configureProcessGroup is a no-op off Unix; cancellation kills only the shell.
func configureProcessGroup(*exec.Cmd) {}
