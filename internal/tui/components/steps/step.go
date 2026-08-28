// Package steps holds the individual screens of the Create Server wizard. The
// wizard page owns the order, the progress bar and the shared configuration;
// each step here only knows how to collect one piece of it.
package steps

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/core"
)

// NextMsg advances the wizard to the following step.
type NextMsg struct{}

// PrevMsg takes the wizard back one step.
type PrevMsg struct{}

// Next and Prev are the commands steps use to move the wizard along.
func Next() tea.Cmd { return func() tea.Msg { return NextMsg{} } }
func Prev() tea.Cmd { return func() tea.Msg { return PrevMsg{} } }

// Config is the server the wizard is assembling. Steps hold a pointer to it so
// each one can fill in its own field without the wizard having to marshal
// values back and forth.
type Config struct {
	// Dir is the folder the server will be created in.
	Dir string

	Name     string
	Category string
	Software string
	Version  string
	RAM      string
	Eula     bool
	AutoRun  bool
}

// Step is one screen of the wizard. The wizard renders the themed "Step N —
// Title" heading and the subtitle itself, so a step's View returns its body
// only and every step's heading looks the same.
type Step interface {
	Init() tea.Cmd

	Update(msg tea.Msg) (tea.Model, tea.Cmd)

	View() tea.View

	// Title names the step in its heading.
	Title() string

	// Subtitle is the muted one-line explanation under the heading.
	Subtitle() string

	// SetLayout gives the step the body rectangle it may draw in.
	SetLayout(core.Layout) Step

	// Scroll reports the step's scroll position so the wizard can draw the
	// shared scrollbar for it. A zero value means the step always fits.
	Scroll() core.Scroll

	// Editing reports whether the step has a text field open. While it does,
	// the wizard hands backspace and esc to the step instead of treating them
	// as navigation, so typing into a field cannot walk the wizard backwards.
	// A step that answers yes must therefore handle esc itself.
	Editing() bool
}
