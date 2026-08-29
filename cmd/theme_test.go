package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// The commands print in the theme the user chose, not in the one Osmium ships
// with: the progress bars and marks a command draws come from the same palette
// the full-screen interface does.
func TestCommandsDrawInTheChosenTheme(t *testing.T) {
	t.Cleanup(func() { theme.SetTheme("Teal") })

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	app := filepath.Join(dir, config.AppFolderName)
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatalf("the config folder could not be made: %v", err)
	}
	if err := os.WriteFile(filepath.Join(app, config.ConfigFileName),
		[]byte(`{"theme":"Dracula"}`), 0o600); err != nil {
		t.Fatalf("the config could not be written: %v", err)
	}

	theme.SetTheme("Teal")
	rootCmd.PersistentPreRun(rootCmd, nil)

	if theme.CurrentTheme != "Dracula" {
		t.Errorf("the commands run in the %s theme, want the configured Dracula",
			theme.CurrentTheme)
	}
}
