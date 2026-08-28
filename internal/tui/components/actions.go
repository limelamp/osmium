package components

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/actions"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// actionEntry is one row of the Actions list: its label and what pressing enter
// on it does. Keeping the behaviour on the entry means process controls and
// panel switches are declared the same way instead of through a growing switch.
type actionEntry struct {
	label    string
	activate func(server *storage.Server) tea.Cmd
}

// actionEntries is the Actions panel's menu, in display order.
var actionEntries = []actionEntry{
	{"Start server", func(s *storage.Server) tea.Cmd { return core.ControlServer(core.ServerStart, s) }},
	{"Stop server", func(s *storage.Server) tea.Cmd { return core.ControlServer(core.ServerStop, s) }},
	{"Restart server", func(s *storage.Server) tea.Cmd { return core.ControlServer(core.ServerRestart, s) }},
	{"Remove files", func(*storage.Server) tea.Cmd { return core.SwitchAction(actions.NewRemoveFilesModel()) }},
	{"Generate run script", func(*storage.Server) tea.Cmd { return core.SwitchAction(actions.NewGenRunScriptModel()) }},
	{"Manage configs", func(*storage.Server) tea.Cmd { return core.SwitchAction(actions.NewManageConfigsModel()) }},
	{"Mod management", func(*storage.Server) tea.Cmd { return core.SwitchAction(actions.NewModManagementModel()) }},
	{"Plugin management", func(*storage.Server) tea.Cmd { return core.SwitchAction(actions.NewPluginManagementModel()) }},
}

// ActionsModel is the root menu of the Manage Servers action pane.
type ActionsModel struct {
	layout  core.Layout
	isFocus bool
	server  *storage.Server

	list core.List
}

func NewActionsModel() ActionsModel {
	m := ActionsModel{}
	m.list = m.list.Resize(len(actionEntries), m.rows())
	return m
}

func (m ActionsModel) Init() tea.Cmd {
	return nil
}

// rows is how many menu rows fit, leaving room for the hint line.
func (m ActionsModel) rows() int {
	return max(0, m.layout.ContentHeight()-2)
}

func (m ActionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		list, activated := m.list.HandleMouse(msg, m.layout, 0)
		m.list = list
		if activated {
			return m, actionEntries[m.list.Cursor].activate(m.server)
		}
		return m, nil

	case tea.KeyPressMsg:
		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

		if msg.String() == "enter" {
			return m, actionEntries[m.list.Cursor].activate(m.server)
		}
	}

	return m, nil
}

func (m ActionsModel) View() tea.View {
	start, end := m.list.Window()

	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rows = append(rows, styles.ListRow(
			m.layout.ContentWidth(),
			i == m.list.Cursor,
			actionEntries[i].label,
			"",
		))
	}

	body := styles.Body(lipgloss.JoinVertical(lipgloss.Left, rows...), m.layout.ContentHeight()-1)

	hints := styles.Hints(m.layout.ContentWidth(),
		styles.Hint{Key: "enter", Desc: "run"},
		styles.Hint{Key: "tab", Desc: "switch pane"},
	)

	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:         m.layout.Width,
		Height:        m.layout.Height,
		Focused:       m.isFocus,
		Title:         m.Title(),
		Content:       body + "\n" + hints,
		ScrollOffset:  m.list.Scroll.Offset,
		ScrollTotal:   m.list.Scroll.Total,
		ScrollVisible: m.list.Scroll.Visible,
	}))
}

// additional methods
func (m ActionsModel) Title() string {
	return "Actions"
}

func (m ActionsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.list = m.list.Resize(len(actionEntries), m.rows())
	return m
}

func (m ActionsModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

// Editing reports that the root menu never takes typed input.
func (m ActionsModel) Editing() bool { return false }

func (m ActionsModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	return m
}
