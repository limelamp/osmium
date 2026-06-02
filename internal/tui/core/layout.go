package core

type Layout struct {
	Width  int
	Height int

	HeaderHeight int
}

func NewLayout(w, h int) Layout {
	return Layout{
		Width:        w,
		Height:       h,
		HeaderHeight: 3,
	}
}

func (l Layout) ContentHeight() int {
	return l.Height - l.HeaderHeight
}
