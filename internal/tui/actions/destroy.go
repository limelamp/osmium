package actions

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// Rows of the destroy panel, in display order.
const (
	destroyKeep = iota
	destroyConfirm
	destroyButton
	destroyCount
)

// DestroyModel deletes a server: its folder and its entry in the list.
//
// Nothing here can be undone, so it is deliberately the slowest action in the
// app to complete: the server's name has to be typed out before the button does
// anything, the way the CLI's destroy asks for it.
type DestroyModel struct {
	base

	store *storage.ServerStore

	keepManifest bool
	confirm      core.Field
	warn         string
}

func NewDestroyModel(store *storage.ServerStore) DestroyModel {
	m := DestroyModel{
		base:         newBase(),
		store:        store,
		keepManifest: true,
		confirm:      core.NewField("type the server's name").WithLimit(60),
	}
	m.base = m.setCount(destroyCount)
	return m
}

func (m DestroyModel) Init() tea.Cmd { return nil }

// Editing reports whether the confirmation field has the keyboard.
func (m DestroyModel) Editing() bool { return m.confirm.IsOpen() }

func (m DestroyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.confirm.IsOpen() {
		return m.updateInput(msg)
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		updated, activated := m.handleMouse(msg)
		m.base = updated
		if activated {
			return m.activate()
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
			return m.activate()
		}
	}

	return m, nil
}

// updateInput handles keys while the name is being typed.
func (m DestroyModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	field, result, cmd := m.confirm.HandleKey(key)
	m.confirm = field

	switch result {
	case core.FieldCommitted:
		// Confirming the name moves straight onto the button, which is the
		// only thing left to do.
		m.list = m.list.MoveTo(destroyButton)
	case core.FieldCancelled:
		m.warn = ""
	}

	return m, cmd
}

// activate acts on the row under the cursor.
func (m DestroyModel) activate() (tea.Model, tea.Cmd) {
	switch m.list.Cursor {
	case destroyKeep:
		m.keepManifest = !m.keepManifest
		return m, nil

	case destroyConfirm:
		field, cmd := m.confirm.Open(m.confirm.Value())
		m.confirm = field
		return m, cmd

	case destroyButton:
		return m.destroy()
	}

	return m, nil
}

// destroy deletes the server, once everything guarding it agrees.
func (m DestroyModel) destroy() (tea.Model, tea.Cmd) {
	if m.needsServer() {
		return m, nil
	}

	if strings.TrimSpace(m.confirm.Value()) != m.server.Name {
		m.warn = "Type the server's name exactly to confirm."
		return m, nil
	}

	if supervisor.Running(m.server.Path) {
		m.warn = m.server.Name + " is still running — stop it first."
		return m, nil
	}

	kept, err := storage.Destroy(m.store, *m.server, m.keepManifest)
	if err != nil {
		m.err = err
		return m, nil
	}

	lines := []string{fmt.Sprintf("Destroyed %s (%s)", m.server.Name, m.server.Path)}
	if kept != "" {
		lines = append(lines, "Kept its manifest at "+kept)
	}

	// In order: drop the server from the list and close this panel first, so
	// the report lands in the console left showing rather than in the one
	// belonging to the server that has just been deleted.
	return m, tea.Sequence(core.ReloadServers(), core.CloseAction(), core.Log(lines...))
}

func (m DestroyModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	confirm := muted.Render("Type the name to confirm")
	if m.confirm.IsOpen() || m.confirm.Value() != "" {
		confirm = m.confirm.View()
	}

	button := "[ Destroy this server ]"
	if strings.TrimSpace(m.confirm.Value()) != m.server.Name {
		button = "[ Destroy this server ]  " + muted.Render("(name not confirmed)")
	}

	rows := []string{
		styles.ListRow(m.layout.ContentWidth(), m.list.Cursor == destroyKeep,
			checkbox(m.keepManifest)+" Keep osmium.json so it can be rebuilt", ""),
		"",
		styles.ListRow(m.layout.ContentWidth(), !m.confirm.IsOpen() && m.list.Cursor == destroyConfirm,
			confirm, ""),
		"",
		styles.ListRow(m.layout.ContentWidth(), m.list.Cursor == destroyButton, button, ""),
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		muted.Render("This deletes "+m.server.Path+" and everything inside it,"),
		muted.Render("world included. It cannot be undone."),
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)

	if m.warn != "" {
		body += "\n\n" + styles.Error(warning(m.warn))
	}

	return m.panelView(m.Title(), body, m.Hints()...)
}

// Hints are the keys this panel responds to.
func (m DestroyModel) Hints() []styles.Hint {
	if m.confirm.IsOpen() {
		return []styles.Hint{
			{Key: "enter", Desc: "confirm name"},
			{Key: "esc", Desc: "clear"},
		}
	}

	return []styles.Hint{
		{Key: "↑/↓", Desc: "move"},
		{Key: "enter", Desc: "select"},
		{Key: "backspace", Desc: "back"},
	}
}

// checkbox renders a tick box in the theme's accent colour when set.
func checkbox(checked bool) string {
	if checked {
		return lipgloss.NewStyle().Foreground(theme.Accent).Render("[✓]")
	}
	return "[ ]"
}

// warning lets a plain message reuse the shared error styling.
type warning string

func (w warning) Error() string { return string(w) }

// additional methods
func (m DestroyModel) Title() string {
	return "Destroy Server"
}

func (m DestroyModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(destroyCount)
	m.confirm = m.confirm.SetWidth(l.ContentWidth() / 2)
	return m
}

func (m DestroyModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m DestroyModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.confirm = m.confirm.SetValue("")
	m.warn = ""
	return m
}
