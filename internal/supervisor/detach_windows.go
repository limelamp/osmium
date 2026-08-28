//go:build windows

package supervisor

import (
	"os/exec"
	"syscall"
)

// Windows equivalents of a new session: its own process group, and no console
// inherited from whatever started it.
const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// detach frees the supervisor from the console that started it, so closing that
// console — or quitting the TUI — does not take the server with it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess,
	}
}
