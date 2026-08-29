// Package actions are the panels of the Manage Servers action pane: what can be
// done to the selected server, one panel per thing.
package actions

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/supervisor"
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

	// What the panel is doing right now: the row that started an operation,
	// and the spinner that says it has not finished.
	busyRow   int
	busyLabel string
	spinner   core.Ticker
	frame     int

	// What it last did, shown for a moment before the hints come back.
	result    string
	failed    bool
	resultAge int
}

// spinnerInterval is how often the busy indicator advances a frame, and
// resultBeats is how many of those beats a result line stays up for — about
// four seconds, long enough to read and short enough not to linger.
const (
	spinnerInterval = 100 * time.Millisecond
	resultBeats     = 40
)

// newBase returns the shared state a panel starts with.
func newBase() base {
	return base{busyRow: -1, spinner: core.NewTicker("action.spinner", spinnerInterval)}
}

// busy reports whether an operation is running.
func (b base) busy() bool { return b.busyRow >= 0 }

// begin marks a row as working and runs the operation on it. The spinner beats
// until the operation reports back.
func (b base) begin(row int, label string, op func(scope.Scope) error) (base, tea.Cmd) {
	b.busyRow, b.busyLabel = row, label
	b.result, b.failed = "", false

	ticker, tick := b.spinner.Restart()
	b.spinner = ticker

	return b, tea.Batch(Run(b.dir(), label, op), tick, core.OperationStarted())
}

// finished clears the busy state and records what the operation said, which the
// panel shows in place of its hints for a few seconds.
func (b base) finished(msg core.OperationDoneMsg) (base, tea.Cmd) {
	b.busyRow, b.busyLabel = -1, ""

	result, failed := "", false
	for _, line := range msg.Lines {
		if line.Tag == supervisor.TagError {
			result, failed = line.Text, true
			break
		}
		result = line.Text
	}

	return b.report(result, failed)
}

// report puts a line in the panel's footer for a few seconds.
//
// The beat that timed the operation goes on timing the line: stopping it here
// would leave the last thing that happened on screen for good.
func (b base) report(result string, failed bool) (base, tea.Cmd) {
	b.result, b.failed, b.resultAge = result, failed, 0

	if result == "" {
		b.spinner = b.spinner.Stop()
		return b, nil
	}

	ticker, tick := b.spinner.Restart()
	b.spinner = ticker
	return b, tick
}

// common handles the messages every panel shares: the spinner beat, and the
// ageing of the result line. It reports whether the message was one of them.
func (b base) common(msg tea.Msg) (base, tea.Cmd, bool) {
	tick, ok := msg.(core.TickMsg)
	if !ok {
		return b, nil, false
	}

	spinner, ours := b.spinner.Accepts(tick)
	if !ours {
		return b, nil, false
	}
	b.spinner = spinner

	if b.busy() {
		b.frame++

		armed, cmd := b.spinner.Arm()
		b.spinner = armed
		return b, cmd, true
	}

	// Not busy any more: the beats that remain are what times the result line
	// out, and they stop once it has gone.
	b.resultAge++
	if b.resultAge >= resultBeats {
		b.result = ""
		return b, nil, true
	}

	armed, cmd := b.spinner.Arm()
	b.spinner = armed
	return b, cmd, true
}

// rowMarker is the spinner drawn on the row an operation is running from.
func (b base) rowMarker(index int) string {
	if b.busyRow != index {
		return ""
	}
	return styles.Spinner(b.frame) + " "
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
	list, activated := b.list.HandleMouse(msg, b.layout.Inner(), 0)
	b.list = list
	return b, activated
}

// footer renders the bottom line of a panel: whatever it is doing, or what it
// last did, and the hints when it is neither.
func (b base) footer(hints ...styles.Hint) string {
	switch {
	case b.err != nil:
		return styles.Error(b.err)

	case b.busy():
		return styles.Spinner(b.frame) + " " +
			lipgloss.NewStyle().Foreground(theme.Inactive).Render(b.busyLabel)

	case b.result != "":
		mark, colour := "✓ ", theme.Accent
		if b.failed {
			mark, colour = "✕ ", styles.ErrorColor
		}

		return lipgloss.NewStyle().Foreground(colour).
			MaxWidth(max(b.layout.ContentWidth(), 1)).
			Render(mark + b.result)
	}

	return styles.Hints(b.layout.ContentWidth(), hints...)
}

// TextPane is the rectangle this panel's text occupies and how far its list
// has scrolled, which is what a mouse selection copies from.
func (b base) TextPane() core.Pane {
	return core.Pane{Bounds: b.layout.Text(), Offset: b.list.Scroll.Offset}
}

// panelView draws an arbitrary body with the footer pinned to the bottom.
func (b base) panelView(title, body string, hints ...styles.Hint) tea.View {
	content := styles.Body(body, b.layout.ContentWidth(), b.layout.ContentHeight()-1)

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

// panelViewScrolled draws a body that scrolls, so the container can show how
// much of it is off screen.
func (b base) panelViewScrolled(title, body string, scroll core.Scroll, hints ...styles.Hint) tea.View {
	content := styles.Body(body, b.layout.ContentWidth(), b.layout.ContentHeight()-1)

	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:         b.layout.Width,
		Height:        b.layout.Height,
		Focused:       b.isFocus,
		Title:         title,
		Content:       content + "\n" + b.footer(hints...),
		ScrollOffset:  scroll.Offset,
		ScrollTotal:   scroll.Total,
		ScrollVisible: scroll.Visible,
	}))
}

// listView draws the panel's list, rendering only the rows currently in view.
func (b base) listView(title string, renderRow func(index int) string, hints ...styles.Hint) tea.View {
	return b.panelView(title, b.list.Render(renderRow), hints...)
}

// row renders one list row in the app's shared list style.
func (b base) row(index int, label, suffix string) string {
	return styles.ListRow(b.layout.ContentWidth(), index == b.list.Cursor, label, suffix)
}

// Run executes a scoped operation in the background, collecting its progress
// lines into a single message so they land in the Activity panel instead of on
// the raw terminal.
//
// It is exported because the Actions menu runs the whole-server operations —
// installing everything tracked, updating everything — without opening a panel
// of its own, and those must report exactly the way a panel's operations do.
func Run(dir, label string, op func(scope.Scope) error) tea.Cmd {
	return func() tea.Msg {
		lines := []supervisor.Line{{Tag: supervisor.TagMod, Text: label}}

		sc := scope.Scope{
			Dir: dir,
			Log: func(line string) {
				lines = append(lines, supervisor.Line{Tag: supervisor.TagMod, Text: line})
			},
			// Downloads report as they go, so the console can draw a bar for
			// them the way the command line does.
			Progress: func(file string, done, total int64) {
				core.Report(dir, file, done, total)
			},
		}

		core.Report(dir, label, 0, 0)
		defer core.Finish(dir)

		if err := op(sc); err != nil {
			lines = append(lines, supervisor.Line{Tag: supervisor.TagError, Text: err.Error()})
		}
		return core.OperationDoneMsg{Lines: lines}
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
func (b base) emptyView(title, message string, hints ...styles.Hint) tea.View {
	return b.panelView(title,
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(message),
		hints...,
	)
}
