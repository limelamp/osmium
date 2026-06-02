package core

import (
	tea "charm.land/bubbletea/v2"
)

// what an "Action" should look like
type Action interface {
	Init() tea.Cmd

	Update(msg tea.Msg) (tea.Model, tea.Cmd)

	View() tea.View

	SetLayout(Layout) Action

	SetFocus(bool) Action

	Title() string
}
