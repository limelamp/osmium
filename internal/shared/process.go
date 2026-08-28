package shared

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

const LockFileName = ".osmium_process.lock"

// LockPath is where a server directory keeps its process lock file.
func LockPath(dir string) string {
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, LockFileName)
}

// ReadLockPID reads the lock file of the process working directory.
func ReadLockPID() (int, error) { return ReadLockPIDIn(".") }

// WriteLockPID writes the lock file of the process working directory.
func WriteLockPID(pid int) error { return WriteLockPIDIn(".", pid) }

// RemoveLockFile removes the lock file of the process working directory.
func RemoveLockFile() error { return RemoveLockFileIn(".") }

// ReadLockPIDIn reads the PID recorded for a specific server directory.
func ReadLockPIDIn(dir string) (int, error) {
	data, err := os.ReadFile(LockPath(dir))
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid lock file PID: %w", err)
	}

	return pid, nil
}

// WriteLockPIDIn records a PID for a specific server directory.
func WriteLockPIDIn(dir string, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}

	if existingPID, err := ReadLockPIDIn(dir); err == nil {
		if IsPIDRunning(existingPID) {
			return fmt.Errorf("server already running with PID %d", existingPID)
		}

		if err := RemoveLockFileIn(dir); err != nil {
			return fmt.Errorf("failed to remove stale lock file: %w", err)
		}
	}

	if err := os.WriteFile(LockPath(dir), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

// RemoveLockFileIn clears the lock file of a specific server directory.
func RemoveLockFileIn(dir string) error {
	if err := os.Remove(LockPath(dir)); err != nil && !os.IsNotExist(err) {
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
