//go:build windows

package engine

import "os/exec"

// configureProcessGroup uses the default cancellation on Windows, which kills
// the claude process itself
func configureProcessGroup(command *exec.Cmd) {
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return command.Process.Kill()
	}
}
