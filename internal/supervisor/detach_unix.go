//go:build !windows

package supervisor

import (
	"os/exec"
	"syscall"
)

// detach puts the supervisor in a session of its own, so closing the terminal
// that started it — or quitting the TUI — does not take the server with it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
