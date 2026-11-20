//go:build !windows

package engine

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup starts claude in its own process group and makes
// cancellation kill the whole group, including subagents and MCP servers
func configureProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
}
