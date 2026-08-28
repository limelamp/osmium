package actions

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
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
	typing       bool
	nameIn       textinput.Model
	warn         string
}

func NewDestroyModel(store *storage.ServerStore) DestroyModel {
	ti := textinput.New()
	ti.Placeholder = "type the server's name"
	ti.Prompt = "› "
	ti.CharLimit = 60

	m := DestroyModel{store: store, keepManifest: true, nameIn: ti}
	m.base = m.setCount(destroyCount)
	return m
}

func (m DestroyModel) Init() tea.Cmd { return nil }

// Editing reports whether the confirmation field has the keyboard.
func (m DestroyModel) Editing() bool { return m.typing }

func (m DestroyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.typing {
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
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "enter", "tab":
			m.typing = false
			m.nameIn.Blur()
			// Confirming the name moves straight onto the button, which is the
			// only thing left to do.
			m.list = m.list.MoveTo(destroyButton)
			return m, nil

		case "esc":
			m.typing = false
			m.warn = ""
			m.nameIn.Reset()
			m.nameIn.Blur()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.nameIn, cmd = m.nameIn.Update(msg)
	return m, cmd
}

// activate acts on the row under the cursor.
func (m DestroyModel) activate() (tea.Model, tea.Cmd) {
	switch m.list.Cursor {
	case destroyKeep:
		m.keepManifest = !m.keepManifest
		return m, nil

	case destroyConfirm:
		m.typing = true
		return m, m.nameIn.Focus()

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

	if strings.TrimSpace(m.nameIn.Value()) != m.server.Name {
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
	if m.typing || m.nameIn.Value() != "" {
		confirm = m.nameIn.View()
	}

	button := "[ Destroy this server ]"
	if strings.TrimSpace(m.nameIn.Value()) != m.server.Name {
		button = "[ Destroy this server ]  " + muted.Render("(name not confirmed)")
	}

	rows := []string{
		styles.ListRow(m.layout.ContentWidth(), m.list.Cursor == destroyKeep,
			checkbox(m.keepManifest)+" Keep osmium.json so it can be rebuilt", ""),
		"",
		styles.ListRow(m.layout.ContentWidth(), !m.typing && m.list.Cursor == destroyConfirm,
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
	if m.typing {
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
	m.nameIn.SetWidth(max(l.ContentWidth()/2, 12))
	return m
}

func (m DestroyModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m DestroyModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.nameIn.Reset()
	m.warn = ""
	return m
}
