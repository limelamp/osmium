package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// An operation has to be visible while it runs and answerable when it ends:
// the row it was started from carries a spinner, and the panel's footer says
// what happened afterwards.
func TestPanelShowsWhatItIsDoing(t *testing.T) {
	// A panel is always laid out before it is drawn, and the footer is fitted
	// to that width.
	started := newBase()
	started.layout = core.Layout{Width: 60, Height: 20}

	panel, _ := started.begin(2, "Installing tracked mods…",
		func(scope.Scope) error { return nil })

	if !panel.busy() {
		t.Fatal("the panel does not consider itself busy after starting an operation")
	}

	if panel.rowMarker(2) == "" {
		t.Error("the row the operation was started from has no spinner")
	}
	if marker := panel.rowMarker(1); marker != "" {
		t.Errorf("a row that started nothing has a spinner: %q", marker)
	}

	if footer := panel.footer(); !strings.Contains(footer, "Installing tracked mods") {
		t.Errorf("the footer does not say what is running: %q", footer)
	}

	// The operation reports back; the panel stops spinning and says how it went.
	finished, cmd := panel.finished(core.OperationDoneMsg{Lines: []supervisor.Line{
		{Tag: supervisor.TagMod, Text: "Installed sodium 0.5.11"},
	}})

	if finished.busy() {
		t.Error("the panel is still busy after the operation finished")
	}
	if finished.rowMarker(2) != "" {
		t.Error("the spinner is still on the row after the operation finished")
	}
	if footer := finished.footer(); !strings.Contains(footer, "Installed sodium") {
		t.Errorf("the footer does not report the result: %q", footer)
	}
	if cmd == nil {
		t.Error("nothing was scheduled to time the result line out")
	}
}

// A failure is reported as one, rather than as the last thing that was said
// before it went wrong.
func TestPanelReportsFailures(t *testing.T) {
	started := newBase()
	started.layout = core.Layout{Width: 60, Height: 20}

	panel, _ := started.begin(0, "Adding sodium…", func(scope.Scope) error { return nil })

	finished, _ := panel.finished(core.OperationDoneMsg{Lines: []supervisor.Line{
		{Tag: supervisor.TagMod, Text: "Adding sodium…"},
		{Tag: supervisor.TagError, Text: "no build for 1.20.1"},
	}})

	footer := finished.footer()
	if !strings.Contains(footer, "no build for 1.20.1") {
		t.Errorf("the failure is not in the footer: %q", footer)
	}
	if !finished.failed {
		t.Error("the panel does not know the operation failed")
	}
}

// Read-only panels have to scroll: the details panel is taller than the pane it
// is drawn in as soon as a server has players, and the wheel and the arrow keys
// both have to move it.
func TestDetailsPanelScrolls(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "osmium.json"),
		[]byte(`{"loader":"Paper","version":"1.21.1","ram":"4G"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	panel := NewDetailsModel().
		SetServer(&storage.Server{Name: "survival", Path: dir}).
		SetLayout(core.Layout{Width: 60, Height: 8})

	before := strings.Split(ansi.Strip(panel.View().Content), "\n")

	scrolled, _ := panel.Update(tea.MouseWheelMsg{X: 2, Y: 2, Button: tea.MouseWheelDown})
	after := strings.Split(ansi.Strip(scrolled.(DetailsModel).View().Content), "\n")

	if before[2] == after[2] {
		t.Errorf("the wheel did not scroll the panel; it still shows:\n%s", after[2])
	}

	// And back up again with the keyboard.
	back, _ := scrolled.(DetailsModel).Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if row := strings.Split(ansi.Strip(back.(DetailsModel).View().Content), "\n")[2]; row == after[2] {
		t.Errorf("the arrow keys did not scroll the panel; it still shows:\n%s", row)
	}
}
