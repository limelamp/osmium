package pages

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/components"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/core"
)

type ServerFiles struct {
	layout core.Layout

	actions  components.ActionsModel
	activity components.ActivityModel

	value int
	count int
	focus int
}

func NewServerFiles() ServerFiles {
	return ServerFiles{
		actions:  components.NewActionsModel().SetFocus(true),
		activity: components.NewActivityModel().SetFocus(false),
		count:    1,
	}
}

func (m ServerFiles) Init() tea.Cmd {
	// forward Init to child components (tea.Batch() is concurrent, no strict order of execution)
	return tea.Batch(
		m.actions.Init(),
		m.activity.Init(),
	)
}

// "conditional message routing" pattern
func (m ServerFiles) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	//? in case you need to register the keymsg for all views despite being out of focus
	// isEvent := false
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "a":
			m.value++
			return m, nil

		case "tab":
			m.focus = (m.focus + 1) % 2

			m.actions = m.actions.SetFocus(m.focus == 0)
			m.activity = m.activity.SetFocus(m.focus == 1)

			return m, nil

			// case "e":
			// 	isEvent = true
		}

		//! inside the case tea.KeyPressMsg after all msg.String() switch/cases
		var cmd tea.Cmd
		var updated tea.Model
		switch m.focus {
		case 0:
			updated, cmd = m.actions.Update(msg)
			m.actions = updated.(components.ActionsModel)
		case 1:
			updated, cmd = m.activity.Update(msg)
			m.activity = updated.(components.ActivityModel)
		}
		return m, cmd
	}

	// if isEvent {
	// 	switch m.focus {
	// 	case 0:
	// 		_, cmd1 := m.actions.Update(msg)
	// 		return m, cmd1

	// 	case 1:
	// 		_, cmd2 := m.activity.Update(msg)
	// 		return m, cmd2
	// 	}
	// }

	// forward the Update message to child components
	updatedActions, cmd1 := m.actions.Update(msg)
	updatedActivity, cmd2 := m.activity.Update(msg)

	m.actions = updatedActions.(components.ActionsModel)
	m.activity = updatedActivity.(components.ActivityModel)

	return m, tea.Batch(cmd1, cmd2)
}

func (m ServerFiles) View() tea.View {
	if m.layout.Width == 0 {
		return tea.NewView("loading...")
	}

	leftView := m.actions.View()
	rightView := m.activity.View()

	return tea.NewView(
		lipgloss.JoinVertical(
			0,
			lipgloss.JoinHorizontal(0, leftView.Content, rightView.Content),
			fmt.Sprintf("Dashboard %d", m.layout.Width),
			fmt.Sprintf("Value: %d\nInit count: %d", m.value, m.count),
		),
	)
}

// additional methods
func (m ServerFiles) Title() string {
	return "Server Files"
}

func (m ServerFiles) SetLayout(l core.Layout) tea.Model {
	m.layout = l

	half := l.Width / 2
	leftLayout := l
	leftLayout.Width = half

	rightLayout := l
	rightLayout.Width = l.Width - half

	// propagate to children components
	m.actions = m.actions.SetLayout(leftLayout)
	m.activity = m.activity.SetLayout(rightLayout)

	return m
}
