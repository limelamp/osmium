package pages

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/components"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// Pane identifies which of the page's three panels holds keyboard focus.
const (
	paneServers = iota
	paneAction
	paneActivity
	paneCount
)

// Pane proportions. The focused pane is given more room: the left column grows
// sideways if focused, the servers and action panes grow vertically depending on focus.
//
// The three sizes are always derived from one another so the panes tile the
// page exactly - the right column is the full height, and the left column's two
// heights add up to it - which is what keeps the reflow on focus from leaving
// gaps or overlapping borders.
const (

	// Ratios of the 'sections'. Can be freely modified to achieve the best look.
	leftWidthRatio        = 0.32
	leftFocusedWidthRatio = 0.42

	serversHeightRatio        = 0.32
	serversFocusedHeightRatio = 0.64
	serversUnfocusedRatio     = 0.32
)

// Smallest a pane may be squeezed to. Below these the reflow would produce
// panes with no usable content, so on a small terminal the page splits evenly
// instead of honouring the ratios.
const (
	minPaneWidth  = 24
	minPaneHeight = 5
)

// ManageServersModel is the page servers are run from: the list and actions
// on the left, and the console on the right.
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

	// arrangement is which of the three layouts the page is drawn in, and
	// hidden says which panes have been put away. Both are remembered between
	// runs.
	arrangement Arrangement
	hidden      [paneCount]bool

	focus int
}

// NewManageServersModel accepts the ServerStore and passes it down to the child components
func NewManageServersModel(store *storage.ServerStore) ManageServersModel {
	page := ManageServersModel{
		store: store,

		// The console beside a column of servers and actions is what Osmium
		// has always drawn; the other two are there to be chosen.
		arrangement:  ConsoleRight,
		servers:      components.NewServersModel(store).SetFocus(true),
		activeAction: components.NewActionsModel(store).SetFocus(false), // Set it to default action "dashboard"'s model
		activity:     components.NewActivityModel().SetFocus(false),
		actions:      make(map[string]core.Action),
		menus:        make(map[string]components.ActionsModel),
	}

	return page.restore()
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
// collapsed - focus must never land on something that is not on screen.
func (m ManageServersModel) focusNext(delta int) ManageServersModel {
	pane := m.focus
	for range paneCount {
		pane = (pane + delta + paneCount) % paneCount
		if m.visible(pane) {
			break
		}
	}
	return m.setFocus(pane)
}

// setFocus moves keyboard focus to a pane and tells every child about it.
func (m ManageServersModel) setFocus(pane int) ManageServersModel {
	m.focus = (pane + paneCount) % paneCount

	m.servers = m.servers.SetFocus(m.focus == paneServers)
	m.activeAction = m.activeAction.SetFocus(m.focus == paneAction)
	m.activity = m.activity.SetFocus(m.focus == paneActivity)

	// The panes are sized from whichever one has focus, so they are re-flowed
	// for the new one - otherwise the children would keep rendering at their
	// old size inside the newly sized boxes.
	page, _ := m.SetLayout(m.layout).(ManageServersModel)
	return page
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

	page, _ := updated.(ManageServersModel)
	return page.setFocus(paneAction)
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

// Update routes each message to the pane it belongs to: the focused one for
// keys, the one under the pointer for the mouse, and the pane that owns the
// subject for everything else.
func (m ManageServersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Sent when the Action needs to be switched.
	case core.SwitchActionMsg:
		return m.swapAction(msg.NewAction)

	// Sent when an action panel wants to hand control back to the root menu.
	case core.CloseActionMsg:
		return m.swapAction(m.menu())

	case core.WakeMsg:
		activity, activityCmd := m.activity.Wake()
		m.activity = activity

		servers, serversCmd := m.servers.Update(msg)
		if list, ok := servers.(components.ServersModel); ok {
			m.servers = list
		}

		// The action panel is woken too: a panel that was parked while another
		// server was selected has had its timer stranded, and it is the wake
		// that starts it again.
		action, actionCmd := m.activeAction.Update(msg)
		if panel, ok := action.(core.Action); ok {
			m.activeAction = panel
		}

		return m, tea.Batch(activityCmd, serversCmd, actionCmd)

	// A finished operation is two things: output for the console, and a reason
	// for the panel that started it to re-read what it shows.
	case core.OperationDoneMsg:
		activity, cmd := m.activity.Update(core.LogMsg(msg))
		if console, ok := activity.(components.ActivityModel); ok {
			m.activity = console
		}

		action, actionCmd := m.activeAction.Update(msg)
		if panel, ok := action.(core.Action); ok {
			m.activeAction = panel
		}

		return m, tea.Batch(cmd, actionCmd)

	case core.SelectServerMsg:
		// Panels reload from disk when they are handed a server, so only a real
		// change is propagated - otherwise every arrow key would rescan the
		// selected server's folder.
		if sameServer(m.selected, msg.Server) {
			return m, nil
		}

		return m.selectServer(msg.Server)

	// Process control and log output belong to the Activity panel, which is
	// what talks to the servers' supervisors.
	case core.ServerControlMsg, core.LogMsg, core.RestartReadyMsg:
		updated, cmd := m.activity.Update(msg)
		if console, ok := updated.(components.ActivityModel); ok {
			m.activity = console
		}
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

	case core.OperationStartedMsg:
		// Something is installing: the console starts polling so it can draw
		// the progress of it.
		activity, cmd := m.activity.Wake()
		m.activity = activity
		return m, cmd

	case tea.KeyPressMsg:
		// A pane with an open text field keeps the two keys the page would
		// otherwise claim for navigation, so esc can cancel the edit and
		// backspace can delete a character. Tab is never taken: focus must be
		// able to leave a pane whatever it is in the middle of.
		if key := msg.String(); (key == "esc" || key == "backspace") && m.editingPane() {
			return m.forwardToPane(msg, m.focus)
		}

		// Enter on a server is how you get to work on it: it opens the
		// actions for it, bringing that pane back if it was put away, and
		// leaves the keyboard there. Backspace at the top of the actions
		// walks back to the list, so the pair is symmetrical.
		if msg.String() == "enter" && m.focus == paneServers {
			return m.reveal(paneAction).setFocus(paneAction), nil
		}

		switch msg.String() {
		case "esc":
			return m, core.RouteTo("Home")

		// The panes are numbered by where they sit in the current layout, and
		// each key both hides its pane and brings it back.
		case "alt+1":
			return m.toggle(paneServers), nil

		case "alt+2":
			return m.toggle(paneAction), nil

		case "alt+3":
			return m.toggle(paneActivity), nil

		case "ctrl+l":
			return m.rearrange(), nil

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

	// Each child returns itself, so these hold by construction; a mismatch
	// would mean a child swapped its own type, which none of them do.
	if list, ok := updatedServers.(components.ServersModel); ok {
		m.servers = list
	}
	if panel, ok := updatedActions.(core.Action); ok {
		m.activeAction = panel
	}
	if console, ok := updatedActivity.(components.ActivityModel); ok {
		m.activity = console
	}

	return m, tea.Batch(cmd1, cmd2, cmd3)
}

// editingPane reports whether the focused pane currently has a text field
// mid-edit.
func (m ManageServersModel) editingPane() bool {
	switch m.focus {
	case paneServers:
		return m.servers.Editing()
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

	// A panel being shown again was not initialised, so it is woken instead:
	// whatever it runs on a timer has to start over.
	if remembered {
		return m, tea.Batch(activityCmd, core.Wake())
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
		if list, ok := updated.(components.ServersModel); ok {
			m.servers = list
		}
	case paneAction:
		updated, cmd = m.activeAction.Update(msg)
		if panel, ok := updated.(core.Action); ok {
			m.activeAction = panel
		}
	case paneActivity:
		updated, cmd = m.activity.Update(msg)
		if console, ok := updated.(components.ActivityModel); ok {
			m.activity = console
		}
	}

	return m, cmd
}

func (m ManageServersModel) View() tea.View {
	if m.layout.Width == 0 {
		return tea.NewView("loading...")
	}

	rects := m.rects()

	// Each pane is drawn where the arrangement put it, and the page is
	// assembled by joining whatever shares an edge. Working from the same
	// rectangles the clicks are tested against is what keeps the two in step.
	panes := map[int]string{}
	if m.visible(paneServers) {
		panes[paneServers] = m.servers.View().Content
	}
	if m.visible(paneAction) {
		panes[paneAction] = m.activeAction.View().Content
	}
	if m.visible(paneActivity) {
		panes[paneActivity] = m.activity.View().Content
	}

	return tea.NewView(assemble(m.layout, rects, panes))
}

// assemble puts the drawn panes where their rectangles say, row by row.
//
// The panes tile the page exactly, so the frame can be built a screen row at a
// time from whichever panes cover that row, ordered by where they start. That
// is arrangement-agnostic: a layout with a column beside a stack assembles the
// same way as one with a row above another, and adding a fourth would need no
// change here.
func assemble(page core.Layout, rects [paneCount]core.Layout, panes map[int]string) string {
	drawn := make(map[int][]string, len(panes))
	for pane, content := range panes {
		drawn[pane] = strings.Split(content, "\n")
	}

	rows := make([]string, 0, page.Height)

	for y := page.Y; y < page.Y+page.Height; y++ {
		type piece struct {
			x    int
			text string
		}

		var pieces []piece
		for pane := range paneCount {
			rect := rects[pane]
			if rect.Width == 0 || y < rect.Y || y >= rect.Y+rect.Height {
				continue
			}

			line := ""
			if row := y - rect.Y; row < len(drawn[pane]) {
				line = drawn[pane][row]
			}

			pieces = append(pieces, piece{x: rect.X, text: line})
		}

		if len(pieces) == 0 {
			continue
		}

		sort.Slice(pieces, func(a, b int) bool { return pieces[a].x < pieces[b].x })

		var row strings.Builder
		for _, part := range pieces {
			row.WriteString(part.text)
		}
		rows = append(rows, row.String())
	}

	return strings.Join(rows, "\n")
}

// ContentAt is the pane a point falls in, which is what a drag selects text
// from. Each pane answers for its own text, so a selection follows the content
// when the pane scrolls under it.
func (m ManageServersModel) ContentAt(x, y int) (core.Pane, bool) {
	switch m.paneAt(x, y) {
	case paneServers:
		return m.servers.TextPane(), true
	case paneAction:
		return m.activeAction.TextPane(), true
	case paneActivity:
		return m.activity.TextPane(), true
	}
	return core.Pane{}, false
}

// Help describes the keys of whichever pane holds focus, so the modal lists
// what works here and now rather than everything the page can do.
func (m ManageServersModel) Help() []core.HelpSection {
	section := core.HelpSection{Title: "Manage Servers - Servers", Hints: m.servers.Hints()}

	switch m.focus {
	case paneAction:
		section = core.HelpSection{
			Title: "Manage Servers - " + m.activeAction.Title(),
			Hints: m.activeAction.Hints(),
		}
	case paneActivity:
		section = core.HelpSection{Title: "Manage Servers - Activity", Hints: m.activity.Hints()}
	}

	// The page's own keys are listed beside the focused pane's, because they
	// work wherever focus happens to be.
	page := core.HelpSection{Title: "Manage Servers - the page", Hints: []styles.Hint{
		{Key: "tab", Desc: "next pane"},
		{Key: "alt+1", Desc: "servers on or off"},
		{Key: "alt+2", Desc: "actions on or off"},
		{Key: "alt+3", Desc: "console on or off"},
		{Key: "ctrl+l", Desc: "next layout (" + m.arrangement.Name() + ")"},
		{Key: "esc", Desc: "home"},
	}}

	return []core.HelpSection{section, page}
}

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
