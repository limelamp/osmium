package core

import (
	tea "charm.land/bubbletea/v2"
)

// List is the cursor-and-scroll behaviour shared by every selectable list in
// the app. It deliberately holds no items and renders nothing: panels keep
// their own data and draw rows with styles.ListRow, so a single navigation
// model can back lists whose rows look nothing alike.
type List struct {
	Cursor int
	Scroll Scroll
}

// Resize records how many items the list holds and how many rows are on screen,
// keeping both the cursor and the scroll offset in range.
func (l List) Resize(total, visible int) List {
	l.Scroll = l.Scroll.Resize(total, visible)
	l.Cursor = min(l.Cursor, max(0, total-1))
	l.Cursor = max(l.Cursor, 0)
	return l.follow()
}

// Move steps the cursor by delta, stopping at either end.
func (l List) Move(delta int) List {
	if l.Scroll.Total == 0 {
		return l
	}
	l.Cursor = min(max(l.Cursor+delta, 0), l.Scroll.Total-1)
	return l.follow()
}

// Cycle steps the cursor by delta, wrapping around the ends: down on the last
// row lands on the first, up on the first lands on the last. It is what the
// arrow keys use, so every list in the app runs in a loop rather than stopping
// dead at its ends.
func (l List) Cycle(delta int) List {
	total := l.Scroll.Total
	if total == 0 {
		return l
	}

	l.Cursor = ((l.Cursor+delta)%total + total) % total
	return l.follow()
}

// MoveTo jumps the cursor to an absolute index.
func (l List) MoveTo(index int) List {
	if index < 0 || index >= l.Scroll.Total {
		return l
	}
	l.Cursor = index
	return l.follow()
}

// Window returns the half-open range of item indices currently on screen.
func (l List) Window() (start, end int) {
	return l.Scroll.Window()
}

// follow keeps the cursor inside the visible window.
func (l List) follow() List {
	l.Scroll = l.Scroll.Follow(l.Cursor)
	return l
}

// HandleKey applies the shared navigation keys. It reports whether the key was
// consumed so callers can fall through to their own bindings.
func (l List) HandleKey(key string) (List, bool) {
	switch key {
	case "up", "k":
		return l.Cycle(-1), true
	case "down", "j":
		return l.Cycle(1), true
	case "pgup":
		return l.Move(-max(1, l.Scroll.Visible)), true
	case "pgdown":
		return l.Move(max(1, l.Scroll.Visible)), true
	case "home":
		return l.MoveTo(0), true
	case "end":
		return l.MoveTo(l.Scroll.Total - 1), true
	}
	return l, false
}

// HitRow maps a mouse position to the item index under it, or -1 when the
// pointer is outside the list. rows is the rectangle the item rows are drawn
// in — Layout.Inner() for a panel inside a container, the plain layout for a
// component that draws bare content — and headerRows is how many of those rows
// the panel spends above the first item.
func (l List) HitRow(rows Layout, headerRows, x, y int) int {
	if !rows.Hit(x, y) {
		return -1
	}

	row := y - rows.Y
	if row < headerRows {
		return -1
	}

	index := l.Scroll.Offset + (row - headerRows)
	if index >= l.Scroll.Total || index >= l.Scroll.Offset+l.Scroll.Visible {
		return -1
	}
	return index
}

// WheelStep is how many rows one notch of the mouse wheel scrolls.
const WheelStep = 3

// HandleMouse applies wheel scrolling and click-to-select over the rectangle
// the rows are drawn in. It returns the updated list and whether the click
// activated the row the cursor was already on, which panels treat exactly like
// pressing enter.
func (l List) HandleMouse(msg tea.MouseMsg, rows Layout, headerRows int) (List, bool) {
	mouse := msg.Mouse()
	if !rows.Hit(mouse.X, mouse.Y) {
		return l, false
	}

	switch msg.(type) {
	case tea.MouseWheelMsg:
		switch mouse.Button {
		case tea.MouseWheelUp:
			l.Scroll = l.Scroll.By(-WheelStep)
		case tea.MouseWheelDown:
			l.Scroll = l.Scroll.By(WheelStep)
		}
		return l, false

	case tea.MouseClickMsg:
		if mouse.Button != tea.MouseLeft {
			return l, false
		}
		index := l.HitRow(rows, headerRows, mouse.X, mouse.Y)
		if index < 0 {
			return l, false
		}
		// Clicking the row the cursor is already on activates it; clicking any
		// other row just moves the cursor there.
		if index == l.Cursor {
			return l, true
		}
		return l.MoveTo(index), false
	}

	return l, false
}
