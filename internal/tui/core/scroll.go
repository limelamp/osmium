package core

// Scroll tracks a vertical scroll offset over Total items of which Visible fit
// on screen at once. Every scrollable surface in the app embeds one so they all
// clamp, follow the cursor and report overflow identically.
type Scroll struct {
	Offset  int
	Total   int
	Visible int
}

// Max is the largest valid offset.
func (s Scroll) Max() int {
	return max(0, s.Total-s.Visible)
}

// Overflows reports whether there is more content than fits.
func (s Scroll) Overflows() bool {
	return s.Total > s.Visible && s.Visible > 0
}

// Resize records new content and viewport sizes, keeping the offset in range.
func (s Scroll) Resize(total, visible int) Scroll {
	s.Total = max(0, total)
	s.Visible = max(0, visible)
	return s.clamp()
}

// By moves the offset by n rows.
func (s Scroll) By(n int) Scroll {
	s.Offset += n
	return s.clamp()
}

// Follow scrolls the minimum amount needed to bring cursor into view.
func (s Scroll) Follow(cursor int) Scroll {
	if s.Visible <= 0 {
		return s
	}
	if cursor < s.Offset {
		s.Offset = cursor
	} else if cursor >= s.Offset+s.Visible {
		s.Offset = cursor - s.Visible + 1
	}
	return s.clamp()
}

// Window returns the half-open range of item indices currently on screen.
func (s Scroll) Window() (start, end int) {
	start = s.Offset
	end = min(s.Total, s.Offset+s.Visible)
	return start, end
}

func (s Scroll) clamp() Scroll {
	s.Offset = min(s.Offset, s.Max())
	s.Offset = max(s.Offset, 0)
	return s
}
