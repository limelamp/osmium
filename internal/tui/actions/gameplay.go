package actions

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/properties"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// GameplayModel edits the settings that decide what a player meets: the
// message they see in their server list, the difficulty, whether they can hurt
// each other, whether they are let in at all.
//
// These live in server.properties along with fifty others, and the file is
// read and written whole so that everything else in it — including anything a
// mod added — stays exactly as it was.
type GameplayModel struct {
	base

	props *properties.Properties

	// editing is the setting being changed, with the field or the choices it
	// is being changed with.
	editing int
	value   core.Field
	choice  int
}

// NewGameplayModel returns the gameplay settings panel.
func NewGameplayModel() GameplayModel {
	m := GameplayModel{
		base:    newBase(),
		editing: -1,
		value:   core.NewField("").WithPrompt("» ").WithLimit(120).SetWidth(34),
	}

	m.base = m.setCount(len(properties.Gameplay))
	return m
}

func (m GameplayModel) Init() tea.Cmd { return nil }

// Editing reports whether a value is being typed.
func (m GameplayModel) Editing() bool { return m.value.IsOpen() }

func (m GameplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.common(msg); handled {
		m.base = updated
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
		if m.editing >= 0 {
			return m.change(msg)
		}

		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

		switch msg.String() {
		case "enter":
			return m.open()
		case "backspace":
			return m, core.CloseAction()
		}
	}

	return m, nil
}

// open starts editing the setting under the cursor.
func (m GameplayModel) open() (tea.Model, tea.Cmd) {
	if m.props == nil || m.list.Cursor >= len(properties.Gameplay) {
		return m, nil
	}

	setting := properties.Gameplay[m.list.Cursor]
	m.editing = m.list.Cursor

	if len(setting.Choices) > 0 {
		m.choice = 0
		for i, choice := range setting.Choices {
			if choice == m.props.Get(setting.Key) {
				m.choice = i
			}
		}
		return m, nil
	}

	field, cmd := m.value.Open(m.props.Get(setting.Key))
	m.value = field

	return m, cmd
}

// change handles the keys while a setting is being edited.
func (m GameplayModel) change(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	setting := properties.Gameplay[m.editing]

	if len(setting.Choices) > 0 {
		switch msg.String() {
		case "up", "k":
			m.choice = (m.choice - 1 + len(setting.Choices)) % len(setting.Choices)
		case "down", "j":
			m.choice = (m.choice + 1) % len(setting.Choices)
		case "enter":
			return m.save(setting.Key, setting.Choices[m.choice])
		case "esc", "backspace":
			m.editing = -1
		}
		return m, nil
	}

	field, result, cmd := m.value.HandleKey(msg)
	m.value = field

	switch result {
	case core.FieldCommitted:
		return m.save(setting.Key, m.value.Value())
	case core.FieldCancelled:
		m.editing = -1
	}

	return m, cmd
}

// save writes one setting back.
func (m GameplayModel) save(key, value string) (tea.Model, tea.Cmd) {
	m.editing = -1

	if m.props == nil {
		return m, nil
	}

	m.props.Set(key, value)

	if err := m.props.Save(); err != nil {
		updated, cmd := m.report(err.Error(), true)
		m.base = updated
		return m, cmd
	}

	note := key + " is now " + value
	if supervisor.Running(m.dir()) {
		note += " — the server reads this when it starts, so restart it"
	}

	updated, cmd := m.report(note, false)
	m.base = updated

	return m.reload(), cmd
}

// reload rereads the file.
func (m GameplayModel) reload() GameplayModel {
	m.props, _ = properties.Read(m.dir())
	m.base = m.setCount(len(properties.Gameplay))

	return m
}

func (m GameplayModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	if m.props == nil {
		return m.emptyView(m.Title(),
			"No server.properties yet — start the server once and it will write one.",
			styles.Hint{Key: "backspace", Desc: "back"})
	}

	if m.editing >= 0 {
		return m.editView()
	}

	body := m.list.Render(func(i int) string {
		setting := properties.Gameplay[i]
		return m.row(i, m.rowMarker(i)+setting.Label, m.props.Get(setting.Key))
	})

	return m.panelView(m.Title(), body, m.Hints()...)
}

// editView is one setting being changed.
func (m GameplayModel) editView() tea.View {
	setting := properties.Gameplay[m.editing]
	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	rows := []string{
		lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(setting.Label),
		muted.Render(setting.Key),
		"",
	}

	if len(setting.Choices) > 0 {
		for i, choice := range setting.Choices {
			rows = append(rows, styles.ListRow(m.layout.ContentWidth(), i == m.choice, choice, ""))
		}
	} else {
		rows = append(rows, m.value.View())
	}

	return m.panelView(m.Title()+" · "+setting.Label,
		lipgloss.JoinVertical(lipgloss.Left, rows...), m.Hints()...)
}

// Hints are the keys this panel responds to.
func (m GameplayModel) Hints() []styles.Hint {
	if m.editing >= 0 {
		return []styles.Hint{
			{Key: "enter", Desc: "save"},
			{Key: "esc", Desc: "cancel"},
		}
	}

	return []styles.Hint{
		{Key: "↑/↓", Desc: "move"},
		{Key: "enter", Desc: "change"},
		{Key: "backspace", Desc: "back"},
	}
}

func (m GameplayModel) Title() string { return "Gameplay" }

func (m GameplayModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(len(properties.Gameplay))
	return m
}

func (m GameplayModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m GameplayModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.editing = -1

	return m.reload()
}
