package core

// Layout is the rectangle a page or component was given by its parent.
//
// X and Y are absolute terminal coordinates of the rectangle's top-left cell.
// They are what makes mouse hit-testing possible: a component receives mouse
// events in absolute coordinates and compares them against its own rect.
type Layout struct {
	X      int
	Y      int
	Width  int
	Height int
}

func NewLayout(w, h int) Layout {
	return Layout{Width: w, Height: h}
}

// Inset returns a copy of the layout moved by dx/dy and resized by dw/dh.
// Parents use it to carve a region up for their children.
func (l Layout) Inset(dx, dy, dw, dh int) Layout {
	return Layout{
		X:      l.X + dx,
		Y:      l.Y + dy,
		Width:  max(0, l.Width+dw),
		Height: max(0, l.Height+dh),
	}
}

// Hit reports whether an absolute terminal coordinate falls inside the rect.
func (l Layout) Hit(x, y int) bool {
	return x >= l.X && x < l.X+l.Width && y >= l.Y && y < l.Y+l.Height
}

// Inner is the rectangle inside a styles.Container's border, which is where
// that container's rows are actually drawn.
//
// Hit-testing is always done against the rectangle the rows occupy, never
// against the layout a component was given: a panel drawn through
// styles.Container passes Inner(), and a component that draws bare content —
// a wizard step, say — passes its layout unchanged. Getting this wrong is what
// makes a click land one row off.
func (l Layout) Inner() Layout {
	return Layout{
		X:      l.X + 1,
		Y:      l.Y + 1,
		Width:  max(0, l.Width-2),
		Height: max(0, l.Height-2),
	}
}

// ContentWidth is the number of columns available to a container's content,
// after its border and horizontal padding.
func (l Layout) ContentWidth() int {
	return max(0, l.Width-4)
}

// ContentHeight is the number of rows available to a container's content,
// after its top and bottom border.
func (l Layout) ContentHeight() int {
	return max(0, l.Height-2)
}
