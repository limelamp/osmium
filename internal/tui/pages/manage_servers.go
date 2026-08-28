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

// Pane proportions. The focused pane is given more room: the servers list grows
// sideways, the action and activity panes grow downwards.
//
// The three sizes are always derived from one another so the panes tile the
// page exactly — the left column is the full height, and the right column's two
// heights add up to it — which is what keeps the reflow on focus from leaving
// gaps or overlapping borders.
const (
	serversWidthRatio        = 0.32
	serversFocusedWidthRatio = 0.42

	actionHeightRatio        = 0.55
	actionFocusedHeightRatio = 0.72
	actionUnfocusedRatio     = 0.32
)

// Smallest a pane may be squeezed to. Below these the reflow would produce
// panes with no usable content, so on a small terminal the page splits evenly
// instead of honouring the ratios.
const (
	minPaneWidth  = 24
	minPaneHeight = 5
)

type ManageServersModel struct {
	layout core.Layout

	servers      components.ServersModel
	activity     components.ActivityModel
	activeAction core.Action

	// actions remembers each server's own action panel, keyed by its path, so
	// switching selection returns to whatever that server was in the middle of
	// instead of dropping another server's panel on it.
	actions map[string]core.Action

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
		actions:      make(map[string]core.Action),
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

	// The panes are sized from whichever one has focus, so they are re-flowed
	// for the new one — otherwise the children would keep rendering at their
	// old size inside the newly sized boxes.
	return m.SetLayout(m.layout).(ManageServersModel)
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

		return m.selectServer(msg.Server)

	// Process control, log output and shutdown all belong to the Activity
	// panel, which owns the running server process.
	case core.ServerControlMsg, core.LogMsg, core.ShutdownMsg:
		updated, cmd := m.activity.Update(msg)
		m.activity = updated.(components.ActivityModel)
		return m, cmd

	case tea.MouseMsg:
		mouse := msg.Mouse()
		pane := m.paneAt(mouse.X, mouse.Y)

		// Clicking an unfocused pane only focuses it. Focusing resizes the
		// panes, so passing the click on as well would land it on whatever row
		// moved under the pointer rather than the one that was clicked.
		if _, isClick := msg.(tea.MouseClickMsg); isClick && pane >= 0 && pane != m.focus {
			return m.setFocus(pane), nil
		}

		return m.forwardToPane(msg, pane)

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

// selectServer points the whole page at another server, parking the outgoing
// server's action panel so it is still there when the user comes back.
func (m ManageServersModel) selectServer(server *storage.Server) (tea.Model, tea.Cmd) {
	if key := serverKey(m.selected); key != "" {
		m.actions[key] = m.activeAction
	}

	m.selected = server

	action, remembered := m.actions[serverKey(server)]
	if !remembered {
		action = components.NewActionsModel()
	}

	m.activeAction = action.
		SetServer(server).
		SetLayout(m.actionLayout()).
		SetFocus(m.focus == paneAction)

	m.activity = m.activity.SetServer(server)

	if remembered {
		return m, nil
	}
	return m, m.activeAction.Init()
}

// swapAction replaces the action panel, handing the new one the page's current
// layout, focus and selection so it never renders a frame misconfigured. The
// live panel is filed under its server by selectServer on the way out, so it is
// still open when the user comes back to that server.
func (m ManageServersModel) swapAction(action core.Action) (tea.Model, tea.Cmd) {
	m.activeAction = action.
		SetServer(m.selected).
		SetLayout(m.actionLayout()).
		SetFocus(m.focus == paneAction)

	return m, m.activeAction.Init()
}

// serverKey identifies a server for the per-server state maps.
func serverKey(server *storage.Server) string {
	if server == nil {
		return ""
	}
	return server.Path
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

// Pane rectangles, derived from the page layout and the focused pane.
func (m ManageServersModel) serversWidth() int {
	ratio := serversWidthRatio
	if m.focus == paneServers {
		ratio = serversFocusedWidthRatio
	}

	return fitPane(int(float64(m.layout.Width)*ratio), m.layout.Width, minPaneWidth)
}

// actionHeight is how tall the action pane is: taller when it has focus,
// shorter when the activity pane below it does.
func (m ManageServersModel) actionHeight() int {
	ratio := actionHeightRatio
	switch m.focus {
	case paneAction:
		ratio = actionFocusedHeightRatio
	case paneActivity:
		ratio = actionUnfocusedRatio
	}

	return fitPane(int(float64(m.layout.Height)*ratio), m.layout.Height, minPaneHeight)
}

// fitPane keeps a pane's size inside the page while leaving its neighbour at
// least the minimum, falling back to an even split when the page cannot hold
// two panes of that minimum at all.
func fitPane(size, total, minimum int) int {
	if total < minimum*2 {
		return total / 2
	}
	return min(max(size, minimum), total-minimum)
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
		Height: m.actionHeight(),
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
