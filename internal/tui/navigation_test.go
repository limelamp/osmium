package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/components"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// press is one arrow key.
func press(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// Lists run in a loop: down on the last row lands on the first, and up on the
// first lands on the last.
func TestListWrapsAtBothEnds(t *testing.T) {
	list := core.List{}.Resize(4, 4)

	list, _ = list.HandleKey("up")
	if list.Cursor != 3 {
		t.Errorf("up from the first row went to %d, want the last row (3)", list.Cursor)
	}

	list, _ = list.HandleKey("down")
	if list.Cursor != 0 {
		t.Errorf("down from the last row went to %d, want the first row (0)", list.Cursor)
	}

	// Paging still stops at the ends: only the arrows run in a loop.
	list = list.Move(-1)
	if list.Cursor != 0 {
		t.Errorf("Move past the start went to %d, want 0", list.Cursor)
	}
}

// The Actions menu is a list with unselectable headings in it, so wrapping has
// to carry on past them in the direction the user was going.
func TestActionsMenuWrapsPastHeadings(t *testing.T) {
	layout := core.Layout{X: 0, Y: 0, Width: 60, Height: 40}

	actions := components.NewActionsModel(storage.NewServerStore()).
		SetLayout(layout).SetFocus(true).(components.ActionsModel)

	// The cursor starts on the first entry, under the first heading.
	if selected := selectedRow(actions); selected != "Start server" {
		t.Fatalf("the menu opened on %q, want Start server", selected)
	}

	updated, _ := actions.Update(press(tea.KeyUp))
	if selected := selectedRow(updated.(components.ActionsModel)); selected != "Destroy server" {
		t.Errorf("up from the first entry selected %q, want the last entry", selected)
	}

	updated, _ = updated.(components.ActionsModel).Update(press(tea.KeyDown))
	if selected := selectedRow(updated.(components.ActionsModel)); selected != "Start server" {
		t.Errorf("down from the last entry selected %q, want the first entry", selected)
	}
}

// selectedRow is the label of the row the cursor is on, as drawn.
func selectedRow(m components.ActionsModel) string {
	for _, row := range screen(m.View().Content) {
		if index := strings.Index(row, "➔ "); index >= 0 {
			return strings.TrimSpace(strings.TrimRight(row[index+len("➔ "):], "│ "))
		}
	}
	return ""
}
