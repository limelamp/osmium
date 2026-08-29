package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/limelamp/osmium/internal/clipboard"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// copiedLinger is how long the confirmation stays in the page's corner.
const copiedLinger = 3 * time.Second

// dragScrollInterval is how often a pane moves on while the pointer is held
// past its edge. It is the one thing that lets a drag reach text far below
// what is on screen without the mouse having to be wiggled to keep it going.
const dragScrollInterval = 60 * time.Millisecond

// copiedMsg reports the outcome of putting a selection on the clipboard.
type copiedMsg struct {
	lines int
	tool  string
	err   error
}

// clearCopiedMsg takes the confirmation back off the screen.
type clearCopiedMsg struct{}

// dragScrollMsg is one beat of a drag held past a pane's edge. The generation
// is the drag it belongs to, so the beats of an abandoned drag die out instead
// of scrolling the next one.
type dragScrollMsg struct{ generation int }

// handleMouse routes a mouse event between selecting text and working the
// interface.
//
// A press starts a possible selection but does nothing else: the click is only
// delivered when the button comes up in the same cell it went down in. That is
// what lets a drag begin on a list row without also activating it - pressing on
// "Start server" and dragging across it would otherwise start the server.
func (m appModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	mouse := msg.Mouse()

	switch event := msg.(type) {
	case tea.MouseClickMsg:
		if event.Button != tea.MouseLeft {
			return m, nil, false // right and middle clicks pass straight through
		}

		pane, ok := m.paneAt(mouse.X, mouse.Y)
		if !ok {
			return m, nil, false
		}

		m.drags++
		m.selection = m.selection.Start(pane, mouse.X, mouse.Y, m.frameRows())
		return m, nil, true

	case tea.MouseMotionMsg:
		if !m.selection.Dragging() {
			return m, nil, false
		}

		m.selection = m.track().DragTo(mouse.X, mouse.Y)

		// Dragging past an edge scrolls the pane, and keeps scrolling it for as
		// long as the pointer is held there.
		model, cmd := m.followEdge()
		return model, cmd, true

	case tea.MouseReleaseMsg:
		if !m.selection.Dragging() {
			return m, nil, false
		}

		m.selection = m.track().DragTo(mouse.X, mouse.Y).Release()

		if !m.selection.Active() {
			// The pointer never left the cell: it was a click after all.
			m.selection = m.selection.Clear()
			return m, nil, false
		}

		return m, m.copySelection(), true
	}

	return m, nil, false
}

// dragScroll is one beat of a drag held past a pane's edge: the pane is read
// as it stands, the selection is stretched to the pointer, and the pane is
// moved on by one wheel step.
func (m appModel) dragScroll(msg dragScrollMsg) (appModel, tea.Cmd) {
	m.scrolling = false
	if msg.generation != m.drags || !m.selection.Dragging() {
		return m, nil
	}

	pointer := m.selection.Pointer()
	m.selection = m.track().DragTo(pointer.X, pointer.Y)

	return m.followEdge()
}

// followEdge keeps a pane moving while the pointer is held outside it, and
// lets it stop as soon as the pointer comes back inside.
func (m appModel) followEdge() (appModel, tea.Cmd) {
	direction, past := m.selection.PastEdge()
	if !past {
		return m, nil
	}

	bounds := m.selection.Bounds()

	button := tea.MouseWheelUp
	if direction > 0 {
		button = tea.MouseWheelDown
	}

	// The pane is scrolled with an ordinary wheel event delivered to the middle
	// of it, so every pane that already scrolls with the wheel scrolls here too
	// without knowing anything about selections.
	wheel := tea.MouseWheelMsg{
		X:      bounds.X + bounds.Width/2,
		Y:      bounds.Y + bounds.Height/2,
		Button: button,
	}
	scroll := func() tea.Msg { return wheel }

	if m.scrolling {
		// A beat is already on its way; two would scroll at twice the speed.
		return m, scroll
	}

	m.scrolling = true
	generation := m.drags

	return m, tea.Batch(scroll, tea.Tick(dragScrollInterval, func(time.Time) tea.Msg {
		return dragScrollMsg{generation: generation}
	}))
}

// track re-reads the pane the drag started in: where its content has scrolled
// to, and what it is showing now. It is what fills a selection in as the pane
// moves under it.
func (m appModel) track() core.Selection {
	bounds := m.selection.Bounds()

	pane, ok := m.paneAt(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2)
	if !ok {
		return m.selection
	}
	return m.selection.Track(pane, m.frameRows())
}

// paneAt asks the active page which pane a point belongs to.
func (m appModel) paneAt(x, y int) (core.Pane, bool) {
	page, ok := m.activePage.(core.Selectable)
	if !ok {
		return core.Pane{}, false
	}
	return page.ContentAt(x, y)
}

// frameRows is the page as it would be drawn right now, row by row.
func (m appModel) frameRows() []string {
	return strings.Split(m.baseFrame(), "\n")
}

// copySelection puts what is selected on the clipboard, by both routes at once:
// the terminal's own escape sequence, and the platform's clipboard command.
func (m appModel) copySelection() tea.Cmd {
	text := m.selection.Text()
	if text == "" {
		return nil
	}

	lines := m.selection.Lines()

	return tea.Batch(
		tea.SetClipboard(text),
		func() tea.Msg {
			tool, err := clipboard.Write(text)
			return copiedMsg{lines: lines, tool: tool, err: err}
		},
	)
}

// copiedReport is what the corner says after a copy.
func copiedReport(msg copiedMsg) string {
	switch {
	case msg.err == nil:
		return fmt.Sprintf("✓ copied %s", plural(msg.lines))

	case errors.Is(msg.err, clipboard.ErrNoTool):
		// OSC 52 was still sent, so this may well have worked; what is certain
		// is that there was no local tool to be sure with.
		return fmt.Sprintf("✓ copied %s · no clipboard tool", plural(msg.lines))

	default:
		return fmt.Sprintf("✕ %s could not copy", msg.tool)
	}
}

// plural counts lines.
func plural(lines int) string {
	if lines == 1 {
		return "1 line"
	}
	return fmt.Sprintf("%d lines", lines)
}

// highlight draws the selection over the rendered frame.
func (m appModel) highlight(frame string) string {
	if !m.selection.Active() {
		return frame
	}

	rows := strings.Split(frame, "\n")
	bounds := m.selection.Bounds()

	for y := max(bounds.Y, 0); y < bounds.Y+bounds.Height && y < len(rows); y++ {
		from, to, ok := m.selection.Span(y)
		if !ok {
			continue
		}

		rows[y] = ansi.Cut(rows[y], 0, from) +
			styles.Selected(ansi.Cut(rows[y], from, to)) +
			ansi.Cut(rows[y], to, m.layout.Width)
	}

	return strings.Join(rows, "\n")
}

// clearCopied takes the confirmation off the screen after a moment.
func clearCopied() tea.Cmd {
	return tea.Tick(copiedLinger, func(time.Time) tea.Msg { return clearCopiedMsg{} })
}
