package util

import (
	"os"
)

var executables = []string{"server.jar", "run.bat", "run.sh"}

// FindExecutable is the path of the running binary, or an empty string when it
// cannot be determined.
func FindExecutable() string {
	for _, exe := range executables {
		if _, err := os.Stat(exe); !os.IsNotExist(err) {
			return exe
		}
	}

	// case for Quilt
	if _, err := os.Stat("./server/server.jar"); !os.IsNotExist(err) {
		return "server.jar"
	}

	return ""
}
