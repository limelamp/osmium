package actions

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// Rows of the edit panel, in display order.
const (
	editName = iota
	editMemory
	editCount
)

// EditServerModel changes the settings Osmium keeps for a server: its name in
// the list, and the memory it is started with.
//
// It is deliberately not migrate: nothing here touches the loader, the version
// or the mods, which is why it is a small panel rather than a page.
type EditServerModel struct {
	base

	store *storage.ServerStore

	memory string
	field  core.Field
	row    int // which row the open field belongs to
	warn   string
}

// NewEditServerModel returns the panel that edits a server's name and memory.
func NewEditServerModel(store *storage.ServerStore) EditServerModel {
	m := EditServerModel{
		base:  newBase(),
		store: store,
		row:   -1,
		field: core.NewField("").WithLimit(60),
	}
	m.base = m.setCount(editCount)
	return m
}

func (m EditServerModel) Init() tea.Cmd { return nil }

// Editing reports whether one of the values is being typed into.
func (m EditServerModel) Editing() bool { return m.field.IsOpen() }

func (m EditServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.field.IsOpen() {
		key, ok := msg.(tea.KeyPressMsg)
		if !ok {
			return m, nil
		}

		field, result, cmd := m.field.HandleKey(key)
		m.field = field

		switch result {
		case core.FieldCommitted:
			return m.commit()
		case core.FieldCancelled:
			m.row, m.warn = -1, ""
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case core.RefreshMsg:
		return m.reload(), nil

	case tea.MouseMsg:
		updated, activated := m.handleMouse(msg)
		m.base = updated
		if activated {
			return m.open()
		}
		return m, nil

	case tea.KeyPressMsg:
		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

		switch msg.String() {
		case "backspace":
			return m, core.CloseAction()
		case "enter", "space":
			return m.open()
		}
	}

	return m, nil
}

// reload rereads the memory the server is configured with.
func (m EditServerModel) reload() EditServerModel {
	m.memory = ""
	if conf, err := config.ReadConfigIn(m.dir()); err == nil {
		m.memory = conf.AllocatedRAM
	}
	return m
}

// open starts editing the value under the cursor.
func (m EditServerModel) open() (tea.Model, tea.Cmd) {
	if m.needsServer() {
		return m, nil
	}

	value := m.server.Name
	if m.list.Cursor == editMemory {
		value = m.memory
	}

	m.row = m.list.Cursor
	m.warn = ""

	field, cmd := m.field.Open(value)
	m.field = field
	return m, cmd
}

// commit writes the edited value: the name to the list, the memory to the
// server's own manifest, which is where the supervisor reads it from.
func (m EditServerModel) commit() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.field.Value())
	row := m.row
	m.row = -1

	switch row {
	case editName:
		if value == "" {
			m.warn = "A server needs a name."
			return m, nil
		}

		server := *m.server
		server.Name = value
		if err := storage.Register(m.store, &server); err != nil {
			m.err = err
			return m, nil
		}

		m.server.Name = value
		return m, tea.Batch(core.ReloadServers(), core.Log("Renamed to "+value))

	case editMemory:
		amount, err := steps.NormaliseRAM(value)
		if err != nil {
			m.warn = err.Error()
			return m, nil
		}

		conf, err := config.ReadConfigIn(m.dir())
		if err != nil {
			m.err = err
			return m, nil
		}

		conf.AllocatedRAM = amount
		if err := config.WriteConfigIn(m.dir(), conf); err != nil {
			m.err = err
			return m, nil
		}

		m.memory = amount
		lines := []string{"Memory set to " + amount}
		if supervisor.Running(m.dir()) {
			lines = append(lines, m.server.Name+" is running with the old amount until it is restarted")
		}

		return m, tea.Batch(core.ReloadServers(), core.Log(lines...))
	}

	return m, nil
}

func (m EditServerModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	body := m.list.Render(func(i int) string {
		label, value := "Name", m.server.Name
		if i == editMemory {
			label, value = "Memory", m.memory
		}

		if m.field.IsOpen() && m.row == i {
			return styles.ListRow(m.layout.ContentWidth(), false,
				label+"  "+m.field.View(), "")
		}
		return styles.ListRow(m.layout.ContentWidth(), i == m.list.Cursor, label, value)
	})

	note := lipgloss.NewStyle().Foreground(theme.Inactive).
		Render("The engine and version are changed by migrating.")

	if m.warn != "" {
		note = styles.Error(warning(m.warn))
	}

	return m.panelView(m.Title(), body+"\n\n"+note, m.Hints()...)
}

// Hints are the keys this panel responds to.
func (m EditServerModel) Hints() []styles.Hint {
	if m.field.IsOpen() {
		return []styles.Hint{
			{Key: "enter", Desc: "save"},
			{Key: "ctrl+u", Desc: "clear"},
			{Key: "esc", Desc: "cancel"},
		}
	}

	return []styles.Hint{
		{Key: "↑/↓", Desc: "move"},
		{Key: "enter", Desc: "edit"},
		{Key: "backspace", Desc: "back"},
	}
}

func (m EditServerModel) Title() string {
	return "Edit Server"
}

func (m EditServerModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(editCount)
	m.field = m.field.SetWidth(l.ContentWidth() / 2)
	return m
}

func (m EditServerModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m EditServerModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.field = m.field.Close()
	m.row, m.warn = -1, ""
	return m.reload()
}
