package shared

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

const LockFileName = ".osmium_process.lock"

func ReadLockPID() (int, error) {
	data, err := os.ReadFile(LockFileName)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid lock file PID: %w", err)
	}

	return pid, nil
}

func WriteLockPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}

	if existingPID, err := ReadLockPID(); err == nil {
		if IsPIDRunning(existingPID) {
			return fmt.Errorf("server already running with PID %d", existingPID)
		}

		if err := RemoveLockFile(); err != nil {
			return fmt.Errorf("failed to remove stale lock file: %w", err)
		}
	}

	if err := os.WriteFile(LockFileName, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

func RemoveLockFile() error {
	if err := os.Remove(LockFileName); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}
	return nil
}

func IsPIDRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		text := strings.TrimSpace(string(output))
		if text == "" || strings.Contains(text, "No tasks are running") {
			return false
		}
		return true
	default:
		proc, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return false
		}
		return true
	}
}
