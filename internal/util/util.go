package util

import (
	"os"
)

var executables = []string{"server.jar", "run.bat", "run.sh"}

func FindExecutable() string {
	for _, exe := range executables {
		if _, err := os.Stat(exe); !os.IsNotExist(err) {
			return exe
		}
	}
	return ""
}
