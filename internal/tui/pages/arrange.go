package pages

import (
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui/core"
)

// Arrangement is one of the ways the Manage Servers page can be laid out.
//
// The three exist because the three panes are read in different proportions
// depending on what is being done: watching a console wants it wide, working
// through a long action list wants that tall, and choosing between many
// servers wants the list full height.
type Arrangement int

// The arrangements, in the order ctrl+l steps through them.
const (
	// ServersLeft gives the servers list the whole left side, with the
	// actions above the console on the right.
	ServersLeft Arrangement = iota

	// ConsoleBelow puts the servers and the actions side by side along the
	// top, with the console across the whole bottom.
	ConsoleBelow

	// ConsoleRight stacks the servers over the actions on the left and gives
	// the console the whole right side.
	ConsoleRight

	arrangementCount
)

// Next is the arrangement after this one.
func (a Arrangement) Next() Arrangement { return (a + 1) % arrangementCount }

// Name is what to call it, in the interface and in the config file.
func (a Arrangement) Name() string {
	switch a {
	case ServersLeft:
		return "servers-left"
	case ConsoleBelow:
		return "console-below"
	default:
		return "console-right"
	}
}

// arrangementNamed is the arrangement a saved name means, and whether the name
// was one. An unrecognised or missing name leaves the default alone.
func arrangementNamed(name string) (Arrangement, bool) {
	for arrangement := Arrangement(0); arrangement < arrangementCount; arrangement++ {
		if arrangement.Name() == name {
			return arrangement, true
		}
	}
	return 0, false
}

// paneNames are what the panes are called in the config file, so that a saved
// layout survives the numbers changing.
var paneNames = [paneCount]string{"servers", "actions", "activity"}

// visible reports whether a pane is being drawn.
func (m ManageServersModel) visible(pane int) bool { return !m.hidden[pane] }

// shown is how many panes are on screen.
func (m ManageServersModel) shown() int {
	count := 0
	for pane := range paneCount {
		if m.visible(pane) {
			count++
		}
	}
	return count
}

// toggle hides a pane, or brings it back.
//
// The last visible pane cannot be hidden: a page with nothing on it is not a
// layout, it is a bug that looks like one.
func (m ManageServersModel) toggle(pane int) ManageServersModel {
	if m.visible(pane) && m.shown() == 1 {
		return m
	}

	m.hidden[pane] = !m.hidden[pane]

	// Focus cannot stay on a pane that has just gone away.
	if m.hidden[pane] && m.focus == pane {
		return m.remember().setFocus(m.firstVisible())
	}
	return m.remember().setFocus(m.focus)
}

// reveal brings a pane back if it was hidden, for the keys that open a pane by
// going to it rather than by naming it.
func (m ManageServersModel) reveal(pane int) ManageServersModel {
	if m.visible(pane) {
		return m
	}

	m.hidden[pane] = false
	return m.remember().setFocus(m.focus)
}

// rearrange moves to the next layout.
func (m ManageServersModel) rearrange() ManageServersModel {
	m.arrangement = m.arrangement.Next()
	return m.remember().setFocus(m.focus)
}

// firstVisible is the pane focus falls back to.
func (m ManageServersModel) firstVisible() int {
	for pane := range paneCount {
		if m.visible(pane) {
			return pane
		}
	}
	return paneServers
}

// remember writes the layout into the config, so the page opens tomorrow the
// way it was left today.
func (m ManageServersModel) remember() ManageServersModel {
	cfg, err := config.LoadConfig()
	if err != nil {
		return m
	}

	cfg.ManageLayout = m.arrangement.Name()

	cfg.HiddenPanes = nil
	for pane := range paneCount {
		if m.hidden[pane] {
			cfg.HiddenPanes = append(cfg.HiddenPanes, paneNames[pane])
		}
	}

	_ = config.SaveConfig(cfg)
	return m
}

// restore reads the layout back at startup.
func (m ManageServersModel) restore() ManageServersModel {
	cfg, err := config.LoadConfig()
	if err != nil {
		return m
	}

	if saved, known := arrangementNamed(cfg.ManageLayout); known {
		m.arrangement = saved
	}

	for _, name := range cfg.HiddenPanes {
		for pane, known := range paneNames {
			if known == name {
				m.hidden[pane] = true
			}
		}
	}

	// Whatever the file says, something has to be on screen.
	if m.shown() == 0 {
		m.hidden = [paneCount]bool{}
	}

	return m
}

// rects is where each pane is drawn. A hidden pane gets the zero rectangle,
// which draws nothing and is hit by no click.
//
// Every arrangement is written as splits of the page rather than as three
// independent boxes, so the panes always tile it exactly: no gaps, no doubled
// borders, and nothing left over when one of them is hidden.
func (m ManageServersModel) rects() [paneCount]core.Layout {
	var out [paneCount]core.Layout

	page := m.layout
	if page.Width == 0 || page.Height == 0 {
		return out
	}

	servers, actions, activity := m.visible(paneServers), m.visible(paneAction), m.visible(paneActivity)

	switch m.arrangement {
	case ServersLeft:
		// Servers down the left; actions over the console on the right.
		var left, right core.Layout
		switch {
		case servers && (actions || activity):
			left, right = splitX(page, m.leftWidth())
		case servers:
			left = page
		default:
			right = page
		}

		out[paneServers] = left

		switch {
		case actions && activity:
			out[paneAction], out[paneActivity] = splitY(right, m.actionHeightIn(right))
		case actions:
			out[paneAction] = right
		case activity:
			out[paneActivity] = right
		}

	case ConsoleBelow:
		// Servers beside actions along the top; console across the bottom.
		var top, bottom core.Layout
		switch {
		case activity && (servers || actions):
			top, bottom = splitY(page, m.topHeight())
		case activity:
			bottom = page
		default:
			top = page
		}

		out[paneActivity] = bottom

		switch {
		case servers && actions:
			out[paneServers], out[paneAction] = splitX(top, m.leftWidth())
		case servers:
			out[paneServers] = top
		case actions:
			out[paneAction] = top
		}

	default:
		// Servers over actions on the left; console down the right.
		var left, right core.Layout
		switch {
		case activity && (servers || actions):
			left, right = splitX(page, m.leftWidth())
		case activity:
			right = page
		default:
			left = page
		}

		out[paneActivity] = right

		switch {
		case servers && actions:
			out[paneServers], out[paneAction] = splitY(left, m.serversHeightIn(left))
		case servers:
			out[paneServers] = left
		case actions:
			out[paneAction] = left
		}
	}

	return out
}

// splitX cuts a rectangle into a left of the given width and the rest.
func splitX(rect core.Layout, width int) (left, right core.Layout) {
	width = fitPane(width, rect.Width, minPaneWidth)

	left = core.Layout{X: rect.X, Y: rect.Y, Width: width, Height: rect.Height}
	right = core.Layout{X: rect.X + width, Y: rect.Y, Width: rect.Width - width, Height: rect.Height}

	return left, right
}

// splitY cuts a rectangle into a top of the given height and the rest.
func splitY(rect core.Layout, height int) (top, bottom core.Layout) {
	height = fitPane(height, rect.Height, minPaneHeight)

	top = core.Layout{X: rect.X, Y: rect.Y, Width: rect.Width, Height: height}
	bottom = core.Layout{X: rect.X, Y: rect.Y + height, Width: rect.Width, Height: rect.Height - height}

	return top, bottom
}

// leftWidth is how wide the left column is, wider when it holds the focus.
func (m ManageServersModel) leftWidth() int {
	ratio := leftWidthRatio
	if m.focus == paneServers || m.focus == paneAction {
		ratio = leftFocusedWidthRatio
	}

	return int(float64(m.layout.Width) * ratio)
}

// serversHeightIn is how tall the servers pane is inside a column it shares
// with the actions.
func (m ManageServersModel) serversHeightIn(column core.Layout) int {
	ratio := serversHeightRatio
	switch m.focus {
	case paneServers:
		ratio = serversFocusedHeightRatio
	case paneAction:
		ratio = serversUnfocusedRatio
	}

	return int(float64(column.Height) * ratio)
}

// actionHeightIn is how tall the action pane is inside a column it shares with
// the console: the same proportions, read the other way up.
func (m ManageServersModel) actionHeightIn(column core.Layout) int {
	ratio := 1 - serversHeightRatio
	switch m.focus {
	case paneAction:
		ratio = serversFocusedHeightRatio
	case paneActivity:
		ratio = serversUnfocusedRatio
	}

	return int(float64(column.Height) * ratio)
}

// topHeight is how tall the top row is when the console is below it.
func (m ManageServersModel) topHeight() int {
	ratio := serversFocusedHeightRatio
	if m.focus == paneActivity {
		ratio = serversUnfocusedRatio
	}

	return int(float64(m.layout.Height) * ratio)
}

// Pane rectangles, each read from the arrangement.
func (m ManageServersModel) serversLayout() core.Layout  { return m.rects()[paneServers] }
func (m ManageServersModel) actionLayout() core.Layout   { return m.rects()[paneAction] }
func (m ManageServersModel) activityLayout() core.Layout { return m.rects()[paneActivity] }

// fitPane keeps a pane's size inside the page while leaving its neighbour at
// least the minimum, falling back to an even split when the page cannot hold
// two panes of that minimum at all.
func fitPane(size, total, minimum int) int {
	if total < minimum*2 {
		return total / 2
	}
	return min(max(size, minimum), total-minimum)
}
