package tui

import (
	"os"
	"testing"
)

// TestMain gives the tests a configuration directory of their own.
//
// The interface saves what the user changes — the theme, the layout of the
// Manage Servers page — and a test that drives the interface changes those
// things. Without this, running the tests would rearrange the panes of whoever
// ran them, which is not a thing a test suite may do.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "osmium-tests-")
	if err != nil {
		panic(err)
	}

	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		panic(err)
	}

	code := m.Run()
	_ = os.RemoveAll(dir)

	os.Exit(code)
}
