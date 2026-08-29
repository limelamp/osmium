package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// Dragging across the screen selects what was dragged across, and what it puts
// on the clipboard is the text as read, without the frame or the colours.
func TestDragSelectsTheTextItCrosses(t *testing.T) {
	var app tea.Model = NewAppModel(storage.NewServerStore())
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	rows := screen(app.View().Content)
	row := rowOf(t, rows, core.Layout{}, "Manage servers")

	// From the start of "Manage servers" to its last character. The column is
	// the printed width of what comes before it, not its byte offset: the row
	// is full of box-drawing characters.
	label := strings.Index(rows[row], "Manage")
	if label < 0 {
		t.Fatalf("the menu entry is not on the row it was found on:\n%s", rows[row])
	}

	start := ansi.StringWidth(rows[row][:label])
	end := start + ansi.StringWidth("Manage servers") - 1

	app, _ = app.Update(click(start, row))
	app, _ = app.Update(drag(end, row))

	model, ok := app.(appModel)
	if !ok {
		t.Fatalf("the app came back as %T", app)
	}

	if !model.selection.Active() {
		t.Fatal("dragging across a line did not select anything")
	}

	if text := model.selection.Text(); text != "Manage servers" {
		t.Errorf("the selection reads %q, want %q", text, "Manage servers")
	}

	// And it is drawn as selected.
	highlighted := app.View().Content
	if !strings.Contains(ansi.Strip(highlighted), "Manage servers") {
		t.Error("the selected text is no longer in the frame")
	}
	if highlighted == model.baseFrame() {
		t.Error("the selection is not drawn any differently from the rest")
	}
}

// A press and release in the same cell is a click, not a selection, and still
// works the interface.
func TestClickIsNotASelection(t *testing.T) {
	var app tea.Model = NewAppModel(storage.NewServerStore())
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	rows := screen(app.View().Content)
	row := rowOf(t, rows, core.Layout{}, "Settings")

	app, _ = app.Update(click(60, row))
	app, _ = app.Update(release(60, row))

	model, ok := app.(appModel)
	if !ok {
		t.Fatalf("the app came back as %T", app)
	}

	if model.selection.Active() {
		t.Error("a click with no movement was treated as a selection")
	}
	if !containsRow(screen(app.View().Content), "➔ Settings") {
		t.Error("the click did not reach the menu")
	}
}

// Dragging past the bottom of a pane scrolls it, and keeps scrolling it for as
// long as the pointer is held there, so a selection can reach what is not on
// screen yet.
func TestDragPastTheEdgeScrolls(t *testing.T) {
	var app tea.Model = NewAppModel(storage.NewServerStore())
	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	app, _ = app.Update(click(40, 20))

	model, ok := app.(appModel)
	if !ok {
		t.Fatalf("the app came back as %T", app)
	}

	bounds := model.selection.Bounds()

	_, cmd := app.Update(drag(40, bounds.Y+bounds.Height+2))
	if cmd == nil {
		t.Fatal("dragging past the pane produced no scroll")
	}

	wheel, beat := false, false
	for _, msg := range messages(cmd) {
		switch msg := msg.(type) {
		case tea.MouseWheelMsg:
			wheel = true
			if msg.Button != tea.MouseWheelDown {
				t.Errorf("dragging below the pane scrolled %v, want down", msg.Button)
			}
		case dragScrollMsg:
			beat = true
		}
	}

	if !wheel {
		t.Error("the drag did not scroll the pane")
	}
	if !beat {
		t.Error("the pane stops scrolling as soon as the mouse stops moving")
	}
}

// The console holds far more than it shows. Dragging from the top of it and
// holding the pointer past its bottom edge scrolls it, and everything that
// goes past on the way is copied — not only the screenful left at the end.
func TestDragThroughALogCopiesWhatScrolledPast(t *testing.T) {
	var app tea.Model = NewAppModel(storage.NewServerStore())
	app, _ = app.Update(tea.WindowSizeMsg{Width: 110, Height: 30})
	app, _ = app.Update(core.ChangePageMsg{Target: "ManageServers"})

	const logged = 120

	lines := make([]supervisor.Line, 0, logged)
	for i := range logged {
		lines = append(lines, supervisor.Line{
			Tag:  supervisor.TagServer,
			Text: fmt.Sprintf("log line %d", i),
		})
	}
	app, _ = app.Update(core.LogMsg{Lines: lines})

	model, ok := app.(appModel)
	if !ok {
		t.Fatalf("the app came back as %T", app)
	}

	console, found := model.paneAt(80, 25)
	if !found {
		t.Fatal("the console is not selectable")
	}
	bounds := console.Bounds

	// Back to the top of the log, so there is something below to drag through.
	for range logged {
		app, _ = app.Update(tea.MouseWheelMsg{
			X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2,
			Button: tea.MouseWheelUp,
		})
	}

	// Press on the first row and hold the pointer below the pane, then let it
	// beat away as it would while the button stays down.
	below := bounds.Y + bounds.Height + 3
	app, _ = app.Update(click(bounds.X, bounds.Y))
	app, _ = app.Update(drag(bounds.X+bounds.Width-1, below))

	// Each beat of the hold moves the pane on and reads what it now shows; the
	// timer that spaces them out is what TestDragPastTheEdgeScrolls covers.
	for range logged {
		model, _ = app.(appModel)

		app, _ = app.Update(tea.MouseWheelMsg{
			X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2,
			Button: tea.MouseWheelDown,
		})
		app, _ = app.Update(dragScrollMsg{generation: model.drags})
	}

	app, _ = app.Update(release(bounds.X+bounds.Width-1, below))

	model, ok = app.(appModel)
	if !ok {
		t.Fatalf("the app came back as %T", app)
	}

	copied := strings.Split(model.selection.Text(), "\n")
	if len(copied) != logged {
		t.Errorf("the drag copied %d lines of a %d line log", len(copied), logged)
	}
	if !strings.HasSuffix(copied[0], "log line 0") {
		t.Errorf("the copy starts at %q, want the first line of the log", copied[0])
	}
	if last := copied[len(copied)-1]; !strings.HasSuffix(last, fmt.Sprintf("log line %d", logged-1)) {
		t.Errorf("the copy ends at %q, want the last line of the log", last)
	}

	// The pane shows ten rows of it; the rest was only ever scrolled past.
	if len(copied) <= bounds.Height {
		t.Errorf("the copy is %d lines, no more than the %d on screen", len(copied), bounds.Height)
	}
}

// messages runs a command and flattens whatever it produces, so a batch can be
// searched for the one message a test is about.
func messages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	switch msg := cmd().(type) {
	case tea.BatchMsg:
		var all []tea.Msg
		for _, sub := range msg {
			all = append(all, messages(sub)...)
		}
		return all
	default:
		return []tea.Msg{msg}
	}
}
