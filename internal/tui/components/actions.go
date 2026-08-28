package components

import (
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/actions"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// actionEntry is one runnable row of the Actions list: its label, what pressing
// enter on it does, and an optional live indicator drawn at the right edge.
// Keeping the behaviour on the entry means process controls, panel switches and
// whole-server operations are all declared the same way instead of through a
// growing switch.
type actionEntry struct {
	label    string
	activate func(m ActionsModel) tea.Cmd
	badge    func(m ActionsModel) string
}

// actionGroup is a titled block of the Actions list. The groups exist to make a
// long list scannable; only the entries inside them can be selected.
type actionGroup struct {
	title   string
	entries []actionEntry
}

// actionGroups is the Actions panel's menu, in display order: what the server
// is doing, what is installed on it, how it is set up, and what cannot be
// undone — kept apart so a destructive row is never next to a routine one.
var actionGroups = []actionGroup{
	{"PROCESS", []actionEntry{
		{label: "Start server", badge: runningDot,
			activate: func(m ActionsModel) tea.Cmd { return core.ControlServer(core.ServerStart, m.server) }},
		{label: "Stop server",
			activate: func(m ActionsModel) tea.Cmd { return core.ControlServer(core.ServerStop, m.server) }},
		{label: "Restart server",
			activate: func(m ActionsModel) tea.Cmd { return core.ControlServer(core.ServerRestart, m.server) }},
	}},

	{"CONTENT", []actionEntry{
		{label: "Mods", badge: func(m ActionsModel) string { return m.count(m.mods) },
			activate: func(ActionsModel) tea.Cmd { return core.SwitchAction(actions.NewModManagementModel()) }},
		{label: "Plugins", badge: func(m ActionsModel) string { return m.count(m.plugins) },
			activate: func(ActionsModel) tea.Cmd { return core.SwitchAction(actions.NewPluginManagementModel()) }},
		{label: "Install everything tracked", activate: func(m ActionsModel) tea.Cmd {
			return m.run("Installing tracked mods and plugins...", shared.InstallProjectsFromConfig)
		}},
		{label: "Track installed files", activate: func(m ActionsModel) tea.Cmd {
			return m.run("Tracking untracked mods and plugins...", shared.TrackProjects)
		}},
		{label: "Update everything", activate: func(m ActionsModel) tea.Cmd {
			return m.run("Updating everything installed...", func(sc scope.Scope) error {
				return shared.UpdateAllProjects(sc, "all")
			})
		}},
	}},

	{"CONFIG", []actionEntry{
		{label: "Manage configs",
			activate: func(ActionsModel) tea.Cmd { return core.SwitchAction(actions.NewManageConfigsModel()) }},
		{label: "Generate run script",
			activate: func(ActionsModel) tea.Cmd { return core.SwitchAction(actions.NewGenRunScriptModel()) }},
		{label: "Migrate version", badge: func(m ActionsModel) string { return m.engine },
			activate: func(m ActionsModel) tea.Cmd { return core.RouteToServer("Migrate", m.server) }},
	}},

	{"DANGER", []actionEntry{
		{label: "Remove files",
			activate: func(ActionsModel) tea.Cmd { return core.SwitchAction(actions.NewRemoveFilesModel()) }},
		{label: "Destroy server",
			activate: func(m ActionsModel) tea.Cmd { return core.SwitchAction(actions.NewDestroyModel(m.store)) }},
	}},
}

// row is one line of the rendered menu: either a group heading or an entry.
// Building the display as rows is what lets the headings scroll with the
// entries they title instead of floating over them.
type row struct {
	heading string
	entry   *actionEntry
}

// actionRows flattens the groups into the rows they are drawn as.
func actionRows() []row {
	var rows []row

	for i, group := range actionGroups {
		if i > 0 {
			rows = append(rows, row{}) // a blank line between groups
		}
		rows = append(rows, row{heading: group.title})

		for index := range group.entries {
			rows = append(rows, row{entry: &actionGroups[i].entries[index]})
		}
	}
	return rows
}

// detailInterval is how often the running state and the installed counts beside
// the entries are refreshed.
const detailInterval = 2 * time.Second

// detailTickMsg asks the panel to re-read what it shows beside the entries.
type detailTickMsg time.Time

func detailTick() tea.Cmd {
	return tea.Tick(detailInterval, func(t time.Time) tea.Msg { return detailTickMsg(t) })
}

// ActionsModel is the root menu of the Manage Servers action pane.
type ActionsModel struct {
	layout  core.Layout
	isFocus bool
	server  *storage.Server

	// store is only needed by the one action that changes what is registered.
	store *storage.ServerStore

	rows []row
	list core.List

	// What the indicators show, refreshed on a timer rather than on every
	// frame: probing a process and reading a manifest are far too expensive to
	// do while drawing.
	running bool
	mods    int
	plugins int
	engine  string
}

func NewActionsModel(store *storage.ServerStore) ActionsModel {
	m := ActionsModel{store: store, rows: actionRows()}
	m.list = m.list.Resize(len(m.rows), m.visibleRows())
	return m.settle(1)
}

func (m ActionsModel) Init() tea.Cmd {
	return detailTick()
}

// visibleRows is how many menu rows fit, leaving room for the hint line.
func (m ActionsModel) visibleRows() int {
	return max(0, m.layout.ContentHeight()-2)
}

// selectable reports whether the cursor may rest on a row.
func (m ActionsModel) selectable(index int) bool {
	return index >= 0 && index < len(m.rows) && m.rows[index].entry != nil
}

// settle moves the cursor off a heading or a blank onto the next entry in the
// direction it was already travelling, so the headings are seen but never
// landed on.
func (m ActionsModel) settle(direction int) ActionsModel {
	if !m.hasEntries() {
		return m
	}

	for !m.selectable(m.list.Cursor) {
		next := m.list.Cursor + direction
		if next < 0 || next >= len(m.rows) {
			direction = -direction // bounced off an end: come back the other way
			continue
		}
		m.list = m.list.MoveTo(next)
	}
	return m
}

// hasEntries reports whether there is anything the cursor could rest on, which
// is what stops settle from searching a list of nothing but headings forever.
func (m ActionsModel) hasEntries() bool {
	for i := range m.rows {
		if m.rows[i].entry != nil {
			return true
		}
	}
	return false
}

// current is the entry under the cursor.
func (m ActionsModel) current() *actionEntry {
	if !m.selectable(m.list.Cursor) {
		return nil
	}
	return m.rows[m.list.Cursor].entry
}

// activate runs the entry under the cursor.
func (m ActionsModel) activate() tea.Cmd {
	entry := m.current()
	if entry == nil {
		return nil
	}
	return entry.activate(m)
}

// run executes a whole-server operation, reporting into the Activity pane.
func (m ActionsModel) run(label string, op func(scope.Scope) error) tea.Cmd {
	if m.server == nil {
		return nil
	}
	return actions.Run(m.server.Path, label, op)
}

// refreshDetails re-reads what the indicators show.
func (m ActionsModel) refreshDetails() ActionsModel {
	m.running, m.mods, m.plugins, m.engine = false, 0, 0, ""
	if m.server == nil || m.server.Path == "" {
		return m
	}

	m.running = supervisor.Running(m.server.Path)

	if conf, err := config.ReadConfigIn(m.server.Path); err == nil {
		m.mods, m.plugins = len(conf.Mods), len(conf.Plugins)
		m.engine = conf.Loader + " " + conf.Version
	}
	return m
}

func (m ActionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case detailTickMsg:
		return m.refreshDetails(), detailTick()

	case tea.MouseMsg:
		before := m.list.Cursor
		list, activated := m.list.HandleMouse(msg, m.layout.Inner(), 0)

		// A click on a heading is not a selection; the cursor stays where it is.
		if !m.selectable(list.Cursor) {
			list = list.MoveTo(before)
		}
		m.list = list

		if activated {
			return m, m.activate()
		}
		return m, nil

	case tea.KeyPressMsg:
		before := m.list.Cursor
		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list

			direction := 1
			if m.list.Cursor < before {
				direction = -1
			}
			return m.settle(direction), nil
		}

		if msg.String() == "enter" {
			return m, m.activate()
		}
	}

	return m, nil
}

func (m ActionsModel) View() tea.View {
	start, end := m.list.Window()

	rendered := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rendered = append(rendered, m.renderRow(i))
	}

	body := styles.Body(lipgloss.JoinVertical(lipgloss.Left, rendered...),
		m.layout.ContentWidth(), m.layout.ContentHeight()-1)

	hints := styles.Hints(m.layout.ContentWidth(), m.Hints()...)

	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:         m.layout.Width,
		Height:        m.layout.Height,
		Focused:       m.isFocus,
		Title:         m.title(),
		Content:       body + "\n" + hints,
		ScrollOffset:  m.list.Scroll.Offset,
		ScrollTotal:   m.list.Scroll.Total,
		ScrollVisible: m.list.Scroll.Visible,
	}))
}

// Hints are the keys this panel responds to.
func (m ActionsModel) Hints() []styles.Hint {
	return []styles.Hint{
		{Key: "↑/↓", Desc: "move"},
		{Key: "enter", Desc: "run"},
		{Key: "ctrl+b", Desc: "hide servers"},
		{Key: "tab", Desc: "switch pane"},
	}
}

// renderRow draws a heading, a blank or an entry.
func (m ActionsModel) renderRow(index int) string {
	current := m.rows[index]

	switch {
	case current.entry == nil && current.heading == "":
		return ""

	case current.entry == nil:
		return lipgloss.NewStyle().Foreground(theme.Inactive).Render(current.heading)
	}

	badge := ""
	if current.entry.badge != nil {
		badge = current.entry.badge(m)
	}

	return styles.ListRow(m.layout.ContentWidth(), index == m.list.Cursor, current.entry.label, badge)
}

// runningDot marks the process rows when the server is up.
func runningDot(m ActionsModel) string {
	if !m.running {
		return ""
	}
	return lipgloss.NewStyle().Foreground(theme.Accent).Render("● running")
}

// count renders an installed count, or nothing when there are none.
func (m ActionsModel) count(n int) string {
	if n == 0 {
		return ""
	}
	return lipgloss.NewStyle().Foreground(theme.Inactive).Render("(" + strconv.Itoa(n) + ")")
}

// title names the pane and the server it acts on, which is the only place the
// selection is visible while the servers pane is collapsed.
func (m ActionsModel) title() string {
	if m.server == nil {
		return m.Title()
	}
	return m.Title() + " · " + m.server.Name
}

// additional methods
func (m ActionsModel) Title() string {
	return "Actions"
}

func (m ActionsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.list = m.list.Resize(len(m.rows), m.visibleRows())
	return m.settle(1)
}

func (m ActionsModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

// Editing reports that the root menu never takes typed input.
func (m ActionsModel) Editing() bool { return false }

func (m ActionsModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	return m.refreshDetails()
}
