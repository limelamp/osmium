// Package core is the vocabulary the interface is built from: the layout
// rectangles, the list and scroll behaviour every panel shares, the messages
// pages send one another, and the small widgets — a text field, a ticker —
// that would otherwise be written once per screen.
package core

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
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

	// TextPane is where the panel's text sits and how far it has scrolled, so
	// a mouse can select from it. Every panel gets this from the shared base
	// it is built on; a panel whose body scrolls on something other than that
	// base's list says so by overriding it.
	TextPane() Pane

	// Hints are the keys the panel responds to in whatever state it is in.
	// They are drawn in the panel's hint bar and listed in the help modal, so
	// a panel states its keys exactly once.
	Hints() []styles.Hint

	Title() string
}
