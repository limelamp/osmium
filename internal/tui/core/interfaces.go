package core

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// Action is a swappable panel hosted by the Manage Servers page. Every action
// is told the rectangle it may draw in, whether it currently owns keyboard
// focus, and which server it is acting on, all through chainable setters so the
// host can reconfigure a freshly built action in one expression.
type Action interface {
	Init() tea.Cmd

	Update(msg tea.Msg) (tea.Model, tea.Cmd)

	View() tea.View

	SetLayout(Layout) Action

	SetFocus(bool) Action

	// SetServer points the action at the server it should operate on. A nil
	// server means nothing is selected and the action must stay read-only.
	SetServer(*storage.Server) Action

	// Editing reports whether the action currently has a text field open. The
	// host page stops claiming keys such as esc and tab while it does, so the
	// field can use them itself.
	Editing() bool

	Title() string
}
