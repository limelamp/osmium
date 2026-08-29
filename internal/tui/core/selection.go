package core

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Point is a position in a pane's own content: X is the column, Y the line.
type Point struct {
	X, Y int
}

// Pane is where a page keeps text a mouse can select: the rectangle it is
// drawn in, how many rows at the top of it are pinned there, and how far the
// body under them has been scrolled.
//
// Fixed and Offset are what turn a screen row into a line of content. A pane
// that does not scroll leaves both at zero, and its rows are its lines.
type Pane struct {
	Bounds Layout
	Fixed  int
	Offset int
}

// Line is the content line drawn on a screen row.
func (p Pane) Line(y int) int {
	row := y - p.Bounds.Y
	if row < p.Fixed {
		return row
	}
	return row + p.Offset
}

// At is the content position under a screen point. The point is pulled into
// the pane first, so dragging past an edge holds on to the nearest line rather
// than losing the selection off the side of it.
func (p Pane) At(x, y int) Point {
	row := min(max(y-p.Bounds.Y, 0), max(p.Bounds.Height-1, 0))
	col := min(max(x-p.Bounds.X, 0), max(p.Bounds.Width-1, 0))

	return Point{X: col, Y: p.Line(p.Bounds.Y + row)}
}

// Selection is a range of text the user has dragged over.
//
// It is held in the content coordinates of the pane the drag started in, not
// in screen cells: a pane that scrolls under a drag moves its text past the
// selection instead of moving the selection off its text. That is what lets a
// drag reach through a log far longer than the pane showing it.
type Selection struct {
	pane   Pane
	anchor Point
	cursor Point

	// pointer is the last place the mouse was, in screen cells, which is what
	// says whether it is being held past an edge.
	pointer Point

	// seen is what the pane has shown during the drag, keyed by content line.
	// Keying it by the line rather than by the row it appeared on is what lets
	// a selection keep hold of text that has since scrolled away, and what
	// stops the same line being copied twice when a pane scrolls back.
	seen map[int]string

	dragging bool
	active   bool
}

// Start begins a drag at a point inside a pane, reading what the pane is
// showing right now.
func (s Selection) Start(pane Pane, x, y int, frame []string) Selection {
	at := pane.At(x, y)

	started := Selection{
		pane:     pane,
		anchor:   at,
		cursor:   at,
		pointer:  Point{X: x, Y: y},
		seen:     make(map[int]string),
		dragging: true,
	}

	return started.read(frame)
}

// Track takes the pane's scroll position as it is now and reads whatever it is
// showing. It runs on every step of a drag, which is how the lines that scroll
// past during one end up in the copy.
//
// A pane is recognised by its rectangle. If the layout changed under the drag,
// the selection keeps what it already had rather than following a different
// pane's text.
func (s Selection) Track(pane Pane, frame []string) Selection {
	if !s.dragging || pane.Bounds != s.pane.Bounds {
		return s
	}

	s.pane = pane
	return s.read(frame)
}

// read records the pane's rows against the content lines they are showing.
func (s Selection) read(frame []string) Selection {
	bounds := s.pane.Bounds

	for row := range bounds.Height {
		y := bounds.Y + row
		if y < 0 || y >= len(frame) {
			continue
		}

		s.seen[s.pane.Line(y)] = ansi.Cut(frame[y], bounds.X, bounds.X+bounds.Width)
	}

	return s
}

// DragTo extends a drag. A selection only becomes real once the pointer has
// left the cell it started in, so a plain click is never a selection.
func (s Selection) DragTo(x, y int) Selection {
	if !s.dragging {
		return s
	}

	s.pointer = Point{X: x, Y: y}
	s.cursor = s.pane.At(x, y)

	if s.cursor != s.anchor {
		s.active = true
	}
	return s
}

// Release ends the drag, keeping whatever was selected on screen.
func (s Selection) Release() Selection {
	s.dragging = false
	return s
}

// Clear forgets the selection entirely.
func (s Selection) Clear() Selection {
	return Selection{}
}

// Dragging reports whether a button is currently held down.
func (s Selection) Dragging() bool { return s.dragging }

// Active reports whether anything is selected.
func (s Selection) Active() bool { return s.active }

// Bounds is the rectangle of the pane the selection lives in.
func (s Selection) Bounds() Layout { return s.pane.Bounds }

// Pointer is where the mouse was last seen, in screen cells.
func (s Selection) Pointer() Point { return s.pointer }

// PastEdge reports which way the pointer is being held outside the pane, as
// -1 above it and 1 below it, and false while it is inside.
func (s Selection) PastEdge() (int, bool) {
	bounds := s.pane.Bounds

	switch {
	case !s.dragging || bounds.Height == 0:
		return 0, false
	case s.pointer.Y < bounds.Y:
		return -1, true
	case s.pointer.Y >= bounds.Y+bounds.Height:
		return 1, true
	}
	return 0, false
}

// ordered returns the two ends of the selection, topmost first.
func (s Selection) ordered() (from, to Point) {
	if s.anchor.Y < s.cursor.Y || (s.anchor.Y == s.cursor.Y && s.anchor.X <= s.cursor.X) {
		return s.anchor, s.cursor
	}
	return s.cursor, s.anchor
}

// span is the column range of the content line selected on it, as columns of
// the pane rather than of the screen.
func (s Selection) span(line int) (from, to int, ok bool) {
	start, end := s.ordered()
	if !s.active || line < start.Y || line > end.Y {
		return 0, 0, false
	}

	from, to = 0, s.pane.Bounds.Width
	if line == start.Y {
		from = start.X
	}
	if line == end.Y {
		// The cell under the pointer is included, as it is in a terminal.
		to = end.X + 1
	}

	if from >= to {
		return 0, 0, false
	}
	return from, to, true
}

// Span is the column range selected on a screen row, in screen columns, or
// ok=false when nothing on that row is selected.
func (s Selection) Span(y int) (from, to int, ok bool) {
	bounds := s.pane.Bounds
	if y < bounds.Y || y >= bounds.Y+bounds.Height {
		return 0, 0, false
	}

	left, right, ok := s.span(s.pane.Line(y))
	if !ok {
		return 0, 0, false
	}
	return bounds.X + left, bounds.X + min(right, bounds.Width), true
}

// Text is what the selection covers, as plain text: the pane's own content,
// without the frame around it and without the colours.
//
// It is read from what the pane showed during the drag, so it includes the
// lines that scrolled past on the way rather than only the ones left on screen
// at the end of it.
func (s Selection) Text() string {
	if !s.active {
		return ""
	}

	start, end := s.ordered()

	lines := make([]string, 0, end.Y-start.Y+1)
	for line := start.Y; line <= end.Y; line++ {
		row, seen := s.seen[line]
		if !seen {
			continue
		}

		from, to, ok := s.span(line)
		if !ok {
			continue
		}

		lines = append(lines, strings.TrimRight(ansi.Strip(ansi.Cut(row, from, to)), " "))
	}

	// Blank rows at the end of a drag are not part of what was read: dragging
	// down through the empty half of a pane should not copy the empty half.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// A selection that covers only blank space is not worth copying.
	if strings.TrimSpace(strings.Join(lines, "")) == "" {
		return ""
	}
	return strings.Join(lines, "\n")
}

// Lines is how many lines the selection would copy.
func (s Selection) Lines() int {
	text := s.Text()
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}
