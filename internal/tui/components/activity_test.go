package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// A download draws a bar in the console while it runs and leaves nothing
// behind when it ends. The log is read for what happened, and a finished
// download's percentage and byte count are no part of that; the command line
// keeps its own bar, where it is the only record there is.
func TestTheProgressBarDoesNotOutliveTheDownload(t *testing.T) {
	dir := t.TempDir()

	console := NewActivityModel().SetLayout(core.Layout{X: 0, Y: 0, Width: 70, Height: 14})
	console, _ = console.SetServer(&storage.Server{Name: "test", Path: dir})

	core.Report(dir, "fabric-api.jar", 5<<20, 10<<20)

	console, cmd := console.Wake()
	console, cmd = beat(t, console, cmd)

	if !strings.Contains(console.View().Content, "%") {
		t.Errorf("the bar is not drawn while the download runs:\n%s", console.View().Content)
	}

	core.Report(dir, "fabric-api.jar", 10<<20, 10<<20)
	core.Finish(dir)

	console, _ = beat(t, console, cmd)

	if frame := console.View().Content; strings.Contains(frame, "%") {
		t.Errorf("the finished bar is still in the log:\n%s", frame)
	}
}

// beat runs one poll of the console: the tick its own timer produced, handed
// back to it.
func beat(t *testing.T, console ActivityModel, cmd tea.Cmd) (ActivityModel, tea.Cmd) {
	t.Helper()

	if cmd == nil {
		t.Fatal("the console stopped polling")
	}

	tick, ok := cmd().(core.TickMsg)
	if !ok {
		t.Fatal("the console's timer produced no tick")
	}

	updated, next := console.Update(tick)
	polled, ok := updated.(ActivityModel)
	if !ok {
		t.Fatalf("the console came back as %T", updated)
	}
	return polled, next
}
