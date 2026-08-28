package actions

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// hintRows is how many content rows every action panel reserves at its bottom
// for the blank separator plus the hint (or error) line.
const hintRows = 2

// base is the wiring every action panel shares: the rectangle it draws in,
// whether it holds keyboard focus, the server it operates on, and the cursor
// and scroll state of its list. Panels embed it so navigation, mouse handling
// and chrome behave identically across all of them.
type base struct {
	layout  core.Layout
	isFocus bool
	server  *storage.Server
	list    core.List
	err     error

	count int
}

// dir is the directory the panel operates in: the selected server's folder, or
// the process working directory when nothing is selected.
func (b base) dir() string {
	if b.server == nil || b.server.Path == "" {
		return "."
	}
	return b.server.Path
}

// scope builds the operation scope for this panel's server, routing progress
// lines into the Activity panel rather than to stdout.
func (b base) scope(log func(string)) scope.Scope {
	return scope.Scope{Dir: b.dir(), Log: log}
}

// listRows is how many list rows fit in the panel body.
func (b base) listRows() int {
	return max(0, b.layout.ContentHeight()-hintRows)
}

// setCount records the number of items in the panel's list and reflows the
// cursor and scroll offset to match.
func (b base) setCount(n int) base {
	b.count = n
	b.list = b.list.Resize(n, b.listRows())
	return b
}

// handleMouse applies wheel scrolling and click-to-select, reporting whether
// the click activated the row the cursor already sat on.
func (b base) handleMouse(msg tea.MouseMsg) (base, bool) {
	list, activated := b.list.HandleMouse(msg, b.layout, 0)
	b.list = list
	return b, activated
}

// footer renders the bottom line of a panel: the error when there is one, and
// the hint bar otherwise.
func (b base) footer(hints ...styles.Hint) string {
	if b.err != nil {
		return styles.Error(b.err)
	}
	return styles.Hints(b.layout.ContentWidth(), hints...)
}

// panelView draws an arbitrary body with the footer pinned to the bottom.
func (b base) panelView(title string, body string, hints ...styles.Hint) tea.View {
	content := styles.Body(body, b.layout.ContentHeight()-1)

	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:         b.layout.Width,
		Height:        b.layout.Height,
		Focused:       b.isFocus,
		Title:         title,
		Content:       content + "\n" + b.footer(hints...),
		ScrollOffset:  b.list.Scroll.Offset,
		ScrollTotal:   b.list.Scroll.Total,
		ScrollVisible: b.list.Scroll.Visible,
	}))
}

// listView draws the panel's list, rendering only the rows currently in view.
func (b base) listView(title string, renderRow func(index int) string, hints ...styles.Hint) tea.View {
	start, end := b.list.Window()

	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rows = append(rows, renderRow(i))
	}

	return b.panelView(title, lipgloss.JoinVertical(lipgloss.Left, rows...), hints...)
}

// row renders one list row in the app's shared list style.
func (b base) row(index int, label string, suffix string) string {
	return styles.ListRow(b.layout.ContentWidth(), index == b.list.Cursor, label, suffix)
}

// run executes a scoped operation in the background, collecting its progress
// lines into a single message so they land in the Activity panel instead of on
// the raw terminal.
func (b base) run(label string, op func(scope.Scope) error) tea.Cmd {
	dir := b.dir()

	return func() tea.Msg {
		lines := []string{label}
		sc := scope.Scope{Dir: dir, Log: func(line string) { lines = append(lines, line) }}

		if err := op(sc); err != nil {
			lines = append(lines, "error: "+err.Error())
		}
		return core.LogMsg{Lines: lines}
	}
}

// Editing reports whether the panel has a text field open. Panels that take
// typed input override it; for the rest the answer is always no.
func (b base) Editing() bool { return false }

// needsServer reports whether the panel is missing the server it needs, and is
// the guard every destructive action runs before touching the filesystem.
func (b base) needsServer() bool {
	return b.server == nil || b.server.Path == ""
}

// noServerView is the placeholder shown when no server is selected.
func (b base) noServerView(title string) tea.View {
	return b.emptyView(title, "Select a server on the left to use this action.",
		styles.Hint{Key: "tab", Desc: "focus servers"},
	)
}

// emptyView is the placeholder a panel shows when it has nothing to list, so an
// empty panel explains itself instead of looking broken.
func (b base) emptyView(title string, message string, hints ...styles.Hint) tea.View {
	return b.panelView(title,
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(message),
		hints...,
	)
}
