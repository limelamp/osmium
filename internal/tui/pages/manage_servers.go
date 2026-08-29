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
	store  *storage.ServerStore

	servers      components.ServersModel
	activity     components.ActivityModel
	activeAction core.Action

	// actions remembers each server's own action panel, keyed by its path, so
	// switching selection returns to whatever that server was in the middle of
	// instead of dropping another server's panel on it.
	actions map[string]core.Action

	// menus remembers where the cursor was in each server's root menu, so
	// backing out of a panel returns to the entry that opened it rather than
	// to the top of the list.
	menus map[string]components.ActionsModel

	// selected is the server every panel on the page currently acts on.
	selected *storage.Server

	// collapsed hides the servers pane, giving the whole width to the action
	// and activity panes. The selected server is still named in their titles,
	// so nothing is lost but the list.
	collapsed bool

	focus int
}

// NewManageServersModel accepts the ServerStore and passes it down to the child components
func NewManageServersModel(store *storage.ServerStore) ManageServersModel {
	return ManageServersModel{
		store:        store,
		servers:      components.NewServersModel(store).SetFocus(true),
		activeAction: components.NewActionsModel(store).SetFocus(false), // Set it to default action "dashboard"'s model
		activity:     components.NewActivityModel().SetFocus(false),
		actions:      make(map[string]core.Action),
		menus:        make(map[string]components.ActionsModel),
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

// focusNext moves focus by one pane, stepping over the servers pane while it is
// collapsed — focus must never land on something that is not on screen.
func (m ManageServersModel) focusNext(delta int) ManageServersModel {
	pane := (m.focus + delta + paneCount) % paneCount
	if m.collapsed && pane == paneServers {
		pane = (pane + delta + paneCount) % paneCount
	}
	return m.setFocus(pane)
}

// toggleServers shows or hides the servers pane.
func (m ManageServersModel) toggleServers() ManageServersModel {
	m.collapsed = !m.collapsed

	// Focus cannot stay on a pane that just went away.
	if m.collapsed && m.focus == paneServers {
		return m.setFocus(paneAction)
	}

	// Re-flow: the remaining panes have a different width now.
	return m.setFocus(m.focus)
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

// OpenServer opens the page on one server rather than on the list.
//
// The action pane takes focus: the server was named on the command line or is
// the folder the user is standing in, so choosing one is not what they came
// here to do.
func (m ManageServersModel) OpenServer(server *storage.Server) ManageServersModel {
	if server == nil {
		return m
	}

	m.servers = m.servers.Select(server.Path)

	updated, _ := m.selectServer(server)
	return updated.(ManageServersModel).setFocus(paneAction)
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
		return m.swapAction(m.menu())

	case core.WakeMsg:
		activity, cmd := m.activity.Wake()
		m.activity = activity
		return m, cmd

	// A finished operation is two things: output for the console, and a reason
	// for the panel that started it to re-read what it shows.
	case core.OperationDoneMsg:
		activity, cmd := m.activity.Update(core.LogMsg{Lines: msg.Lines})
		m.activity = activity.(components.ActivityModel)

		action, refreshCmd := m.activeAction.Update(core.RefreshMsg{})
		m.activeAction = action.(core.Action)

		return m, tea.Batch(cmd, refreshCmd)

	case core.SelectServerMsg:
		// Panels reload from disk when they are handed a server, so only a real
		// change is propagated — otherwise every arrow key would rescan the
		// selected server's folder.
		if sameServer(m.selected, msg.Server) {
			return m, nil
		}

		return m.selectServer(msg.Server)

	// Process control and log output belong to the Activity panel, which is
	// what talks to the servers' supervisors.
	case core.ServerControlMsg, core.LogMsg, core.RestartReadyMsg:
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
		// A pane with an open text field keeps the two keys the page would
		// otherwise claim for navigation, so esc can cancel the edit and
		// backspace can delete a character. Tab is never taken: focus must be
		// able to leave a pane whatever it is in the middle of.
		if key := msg.String(); (key == "esc" || key == "backspace") && m.editingPane() {
			return m.forwardToPane(msg, m.focus)
		}

		switch msg.String() {
		case "esc":
			return m, core.RouteTo("Home")

		case "ctrl+b":
			return m.toggleServers(), nil

		case "tab":
			return m.focusNext(1), nil

		case "shift+tab":
			return m.focusNext(-1), nil
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

// editingPane reports whether the focused pane currently has a text field
// mid-edit.
func (m ManageServersModel) editingPane() bool {
	switch m.focus {
	case paneAction:
		return m.activeAction.Editing()
	case paneActivity:
		return m.activity.Editing()
	}
	return false
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
		action = m.menu()
	}

	m.activeAction = action.
		SetServer(server).
		SetLayout(m.actionLayout()).
		SetFocus(m.focus == paneAction)

	activity, activityCmd := m.activity.SetServer(server)
	m.activity = activity

	if remembered {
		return m, activityCmd
	}
	return m, tea.Batch(activityCmd, m.activeAction.Init())
}

// menu is the root Actions list for the selected server, as it was left.
func (m ManageServersModel) menu() core.Action {
	if remembered, ok := m.menus[serverKey(m.selected)]; ok {
		return remembered
	}
	return components.NewActionsModel(m.store)
}

// swapAction replaces the action panel, handing the new one the page's current
// layout, focus and selection so it never renders a frame misconfigured. The
// live panel is filed under its server by selectServer on the way out, so it is
// still open when the user comes back to that server.
func (m ManageServersModel) swapAction(action core.Action) (tea.Model, tea.Cmd) {
	// The root menu is filed away as it leaves, so the panel it opened returns
	// to the entry it was opened from.
	if menu, ok := m.activeAction.(components.ActionsModel); ok {
		m.menus[serverKey(m.selected)] = menu
	}

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

	right := lipgloss.JoinVertical(
		lipgloss.Left,
		m.activeAction.View().Content,
		m.activity.View().Content,
	)

	if m.collapsed {
		return tea.NewView(right)
	}

	return tea.NewView(lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.servers.View().Content,
		right,
	))
}

// Pane rectangles, derived from the page layout and the focused pane.
func (m ManageServersModel) serversWidth() int {
	if m.collapsed {
		return 0
	}

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

// Help describes the keys of whichever pane holds focus, so the modal lists
// what works here and now rather than everything the page can do.
func (m ManageServersModel) Help() []core.HelpSection {
	section := core.HelpSection{Title: "Manage Servers — Servers", Hints: m.servers.Hints()}

	switch m.focus {
	case paneAction:
		section = core.HelpSection{
			Title: "Manage Servers — " + m.activeAction.Title(),
			Hints: m.activeAction.Hints(),
		}
	case paneActivity:
		section = core.HelpSection{Title: "Manage Servers — Activity", Hints: m.activity.Hints()}
	}

	return []core.HelpSection{section}
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
