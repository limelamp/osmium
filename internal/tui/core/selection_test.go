package core

import (
	"fmt"
	"strings"
	"testing"
)

// pane is a three-row window onto a numbered list of lines, which is enough to
// scroll something past a selection.
func pane(offset int) (window Pane, frame []string) {
	bounds := Layout{X: 0, Y: 0, Width: 10, Height: 3}

	frame = make([]string, bounds.Height)
	for row := range frame {
		frame[row] = fmt.Sprintf("line %d", offset+row)
	}

	return Pane{Bounds: bounds}, frame
}

// The point of holding a selection in content coordinates: a pane that scrolls
// under a drag hands over the lines that went past, not only the ones left on
// screen when the button came up.
func TestSelectionKeepsWhatScrolledPast(t *testing.T) {
	top, frame := pane(0)

	selection := Selection{}.Start(top, 0, 0, frame).DragTo(9, 2)

	// The pane scrolls by its own height twice, as it would while the pointer
	// is held past its bottom edge.
	for _, offset := range []int{3, 6} {
		scrolled, rows := pane(offset)
		scrolled.Offset = offset

		selection = selection.Track(scrolled, rows).DragTo(9, 2)
	}

	want := make([]string, 0, 9)
	for line := range 9 {
		want = append(want, fmt.Sprintf("line %d", line))
	}

	if text := selection.Text(); text != strings.Join(want, "\n") {
		t.Errorf("the selection reads\n%s\nwant\n%s", text, strings.Join(want, "\n"))
	}
	if lines := selection.Lines(); lines != 9 {
		t.Errorf("the selection counts %d lines, want 9", lines)
	}
}

// A selection is measured against the text, so scrolling the pane moves the
// highlight to wherever that text has gone.
func TestHighlightFollowsTheText(t *testing.T) {
	top, frame := pane(0)

	selection := Selection{}.Start(top, 0, 0, frame).DragTo(5, 0)
	if _, _, ok := selection.Span(0); !ok {
		t.Fatal("the first row is not drawn as selected")
	}

	// Two lines go past: what was selected on the top row is now off the pane.
	scrolled, rows := pane(2)
	scrolled.Offset = 2
	selection = selection.Track(scrolled, rows)

	if _, _, ok := selection.Span(0); ok {
		t.Error("the highlight stayed on the row instead of following the text")
	}
}

// Rows pinned above a scrolling body keep their own lines, so a table's header
// is not mistaken for the row that scrolled under it.
func TestPinnedRowsAreNotScrolled(t *testing.T) {
	header := Pane{Bounds: Layout{X: 0, Y: 0, Width: 10, Height: 3}, Fixed: 1, Offset: 5}

	if line := header.Line(0); line != 0 {
		t.Errorf("the pinned row reads line %d, want 0", line)
	}
	if line := header.Line(1); line != 6 {
		t.Errorf("the first body row reads line %d, want 6", line)
	}
}
