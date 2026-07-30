package config

import (
	"os"
	"path/filepath"
)

const AppFolderName = "osmium"

// GetAppDir returns the platform-specific config/data directory.
func GetAppDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to user home directory if system config dir isn't resolvable
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "."+AppFolderName), nil
	}
	return filepath.Join(baseDir, AppFolderName), nil
}

// EnsureAppDirExists creates the configuration directory if it does not yet exist.
func EnsureAppDirExists() (string, error) {
	dir, err := GetAppDir()
	if err != nil {
		return "", err
	}

	// Ensure directory structure exists (e.g., ~/.config/osmium)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}
