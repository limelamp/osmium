package pages

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/components"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
)

type ManageServersModel struct {
	layout core.Layout

	servers  components.ServersModel
	actions  components.ActionsModel
	activity components.ActivityModel

	value int
	focus int
}

// NewManageServersModel accepts the ServerStore and passes it down to the child components
func NewManageServersModel(store *storage.ServerStore) ManageServersModel {
	return ManageServersModel{
		servers:  components.NewServersModel(store).SetFocus(true),
		actions:  components.NewActionsModel().SetFocus(false),
		activity: components.NewActivityModel().SetFocus(false),
	}
}

func (m ManageServersModel) Init() tea.Cmd {
	// forward Init to child components (tea.Batch() is concurrent, no strict order of execution)
	return tea.Batch(
		m.servers.Init(),
		m.actions.Init(),
		m.activity.Init(),
	)
}

// "conditional message routing" pattern
func (m ManageServersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	//? in case you need to register the keymsg for all views despite being out of focus
	// isEvent := false
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "a":
			m.value++
			return m, nil

		case "tab":
			m.focus = (m.focus + 1) % 3

			m.servers = m.servers.SetFocus(m.focus == 0)
			m.actions = m.actions.SetFocus(m.focus == 1)
			m.activity = m.activity.SetFocus(m.focus == 2)

			return m.SetLayout(m.layout), nil

			// case "e":
			// 	isEvent = true
		}

		//! inside the case tea.KeyPressMsg after all msg.String() switch/cases
		var cmd tea.Cmd
		var updated tea.Model
		switch m.focus {
		case 0:
			updated, cmd = m.servers.Update(msg)
			m.servers = updated.(components.ServersModel)
		case 1:
			updated, cmd = m.actions.Update(msg)
			m.actions = updated.(components.ActionsModel)
		case 2:
			updated, cmd = m.activity.Update(msg)
			m.activity = updated.(components.ActivityModel)
		}
		return m, cmd
	}

	// if isEvent {
	// 	switch m.focus {
	// 	case 0:
	// 		_, cmd1 := m.servers.Update(msg)
	// 		return m, cmd1

	// 	case 1:
	// 		_, cmd2 := m.actions.Update(msg)
	// 		return m, cmd2
	// 	}
	// }

	// forward the Update message to child components
	updatedServers, cmd1 := m.servers.Update(msg)
	updatedActions, cmd2 := m.actions.Update(msg)
	updatedActivity, cmd3 := m.activity.Update(msg)

	m.servers = updatedServers.(components.ServersModel)
	m.actions = updatedActions.(components.ActionsModel)
	m.activity = updatedActivity.(components.ActivityModel)

	return m, tea.Batch(cmd1, cmd2, cmd3)
}

func (m ManageServersModel) View() tea.View {
	if m.layout.Width == 0 {
		return tea.NewView("loading...")
	}

	leftView := m.servers.View()
	topView := m.actions.View()
	bottomView := m.activity.View()

	content := lipgloss.JoinHorizontal(
		0,
		leftView.Content,
		lipgloss.JoinVertical(
			0,
			topView.Content,
			bottomView.Content,
		),
	)

	return tea.NewView(content)
}

// additional methods
func (m ManageServersModel) Title() string {
	return "Manage Servers"
}

func (m ManageServersModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l

	half := int(float64(l.Width) * 0.35)
	leftLayout := l
	leftLayout.Width = half

	topHeightRatio := 0.65
	if m.focus == 2 {
		topHeightRatio = 0.35
	}

	topLayout := l
	topLayout.Width = l.Width - half
	topLayout.Height = int(float64(l.Height) * topHeightRatio)

	bottomLayout := l
	bottomLayout.Width = topLayout.Width
	bottomLayout.Height = l.Height - topLayout.Height

	// propagate to children components
	m.servers = m.servers.SetLayout(leftLayout)
	m.actions = m.actions.SetLayout(topLayout)
	m.activity = m.activity.SetLayout(bottomLayout)

	return m
}
