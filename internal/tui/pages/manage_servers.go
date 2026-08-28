package pages

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/components"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// Pane identifies which of the page's three panels holds keyboard focus.
const (
	paneServers = iota
	paneAction
	paneActivity
	paneCount
)

// Pane proportions. They are constants rather than state so the panels keep
// their size when focus moves — a pane that resized itself on focus made the
// whole page jump around.
const (
	serversWidthRatio = 0.32
	actionHeightRatio = 0.6
)

type ManageServersModel struct {
	layout core.Layout

	servers      components.ServersModel
	activity     components.ActivityModel
	activeAction core.Action

	// selected is the server every panel on the page currently acts on.
	selected *storage.Server

	focus int
}

// NewManageServersModel accepts the ServerStore and passes it down to the child components
func NewManageServersModel(store *storage.ServerStore) ManageServersModel {
	return ManageServersModel{
		servers:      components.NewServersModel(store).SetFocus(true),
		activeAction: components.NewActionsModel().SetFocus(false), // Set it to default action "dashboard"'s model
		activity:     components.NewActivityModel().SetFocus(false),
	}
}

func (m ManageServersModel) Init() tea.Cmd {
	// forward Init to child components (tea.Batch() is concurrent, no strict order of execution)
	return tea.Batch(
		m.servers.Init(),
		m.activeAction.Init(),
		m.activity.Init(),
	)
}

// setFocus moves keyboard focus to a pane and tells every child about it.
func (m ManageServersModel) setFocus(pane int) ManageServersModel {
	m.focus = (pane + paneCount) % paneCount

	m.servers = m.servers.SetFocus(m.focus == paneServers)
	m.activeAction = m.activeAction.SetFocus(m.focus == paneAction)
	m.activity = m.activity.SetFocus(m.focus == paneActivity)

	return m
}

// paneAt reports which pane covers an absolute terminal coordinate.
func (m ManageServersModel) paneAt(x, y int) int {
	switch {
	case m.serversLayout().Hit(x, y):
		return paneServers
	case m.actionLayout().Hit(x, y):
		return paneAction
	case m.activityLayout().Hit(x, y):
		return paneActivity
	}
	return -1
}

// "conditional message routing" pattern
func (m ManageServersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Sent when the Action needs to be switched.
	case core.SwitchActionMsg:
		return m.swapAction(msg.NewAction)

	// Sent when an action panel wants to hand control back to the root menu.
	case core.CloseActionMsg:
		return m.swapAction(components.NewActionsModel())

	case core.SelectServerMsg:
		// Panels reload from disk when they are handed a server, so only a real
		// change is propagated — otherwise every arrow key would rescan the
		// selected server's folder.
		if sameServer(m.selected, msg.Server) {
			return m, nil
		}

		m.selected = msg.Server
		m.activeAction = m.activeAction.SetServer(msg.Server).SetLayout(m.actionLayout())
		m.activity = m.activity.SetServer(msg.Server)
		return m, nil

	// Process control, log output and shutdown all belong to the Activity
	// panel, which owns the running server process.
	case core.ServerControlMsg, core.LogMsg, core.ShutdownMsg:
		updated, cmd := m.activity.Update(msg)
		m.activity = updated.(components.ActivityModel)
		return m, cmd

	case tea.MouseMsg:
		// Clicking a pane focuses it, then the click is delivered to that pane
		// so a single click can both focus and select.
		mouse := msg.Mouse()
		if _, isClick := msg.(tea.MouseClickMsg); isClick {
			if pane := m.paneAt(mouse.X, mouse.Y); pane >= 0 {
				m = m.setFocus(pane)
			}
		}
		return m.forwardToPane(msg, m.paneAt(mouse.X, mouse.Y))

	case tea.KeyPressMsg:
		// An action panel with an open text field keeps every key, including
		// the ones the page would otherwise claim for navigation.
		if m.focus == paneAction && m.activeAction.Editing() {
			return m.forwardToPane(msg, paneAction)
		}

		switch msg.String() {
		case "esc":
			return m, core.RouteTo("Home")

		case "tab":
			return m.setFocus(m.focus + 1), nil

		case "shift+tab":
			return m.setFocus(m.focus - 1), nil
		}

		//! inside the case tea.KeyPressMsg after all msg.String() switch/cases
		return m.forwardToPane(msg, m.focus)
	}

	// forward the Update message to child components
	updatedServers, cmd1 := m.servers.Update(msg)
	updatedActions, cmd2 := m.activeAction.Update(msg)
	updatedActivity, cmd3 := m.activity.Update(msg)

	m.servers = updatedServers.(components.ServersModel)
	m.activeAction = updatedActions.(core.Action)
	m.activity = updatedActivity.(components.ActivityModel)

	return m, tea.Batch(cmd1, cmd2, cmd3)
}

// sameServer reports whether two selections point at the same server folder.
func sameServer(a, b *storage.Server) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Path == b.Path
}

// swapAction replaces the action panel, handing the new one the page's current
// layout, focus and selection so it never renders a frame misconfigured.
func (m ManageServersModel) swapAction(action core.Action) (tea.Model, tea.Cmd) {
	m.activeAction = action.
		SetServer(m.selected).
		SetLayout(m.actionLayout()).
		SetFocus(m.focus == paneAction)

	return m, m.activeAction.Init()
}

// forwardToPane delivers a message to a single pane and stores the result.
func (m ManageServersModel) forwardToPane(msg tea.Msg, pane int) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var updated tea.Model

	switch pane {
	case paneServers:
		updated, cmd = m.servers.Update(msg)
		m.servers = updated.(components.ServersModel)
	case paneAction:
		updated, cmd = m.activeAction.Update(msg)
		m.activeAction = updated.(core.Action)
	case paneActivity:
		updated, cmd = m.activity.Update(msg)
		m.activity = updated.(components.ActivityModel)
	}

	return m, cmd
}

func (m ManageServersModel) View() tea.View {
	if m.layout.Width == 0 {
		return tea.NewView("loading...")
	}

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.servers.View().Content,
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.activeAction.View().Content,
			m.activity.View().Content,
		),
	)

	return tea.NewView(content)
}

// Pane rectangles. Each is derived only from the page layout so that every
// panel keeps the same size no matter which one has focus.
func (m ManageServersModel) serversWidth() int {
	return int(float64(m.layout.Width) * serversWidthRatio)
}

func (m ManageServersModel) serversLayout() core.Layout {
	return core.Layout{
		X:      m.layout.X,
		Y:      m.layout.Y,
		Width:  m.serversWidth(),
		Height: m.layout.Height,
	}
}

func (m ManageServersModel) actionLayout() core.Layout {
	return core.Layout{
		X:      m.layout.X + m.serversWidth(),
		Y:      m.layout.Y,
		Width:  m.layout.Width - m.serversWidth(),
		Height: int(float64(m.layout.Height) * actionHeightRatio),
	}
}

func (m ManageServersModel) activityLayout() core.Layout {
	action := m.actionLayout()
	return core.Layout{
		X:      action.X,
		Y:      action.Y + action.Height,
		Width:  action.Width,
		Height: m.layout.Height - action.Height,
	}
}

// additional methods
func (m ManageServersModel) Title() string {
	return "Manage Servers"
}

func (m ManageServersModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l

	// propagate to children components
	m.servers = m.servers.SetLayout(m.serversLayout())
	m.activeAction = m.activeAction.SetLayout(m.actionLayout())
	m.activity = m.activity.SetLayout(m.activityLayout())

	return m
}
