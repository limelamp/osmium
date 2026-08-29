package actions

import (
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// RemoveFilesModel lists the selected server's directory and deletes whichever
// entries the user marks.
type RemoveFilesModel struct {
	base

	options  []os.DirEntry
	selected map[int]bool
}

func NewRemoveFilesModel() RemoveFilesModel {
	return RemoveFilesModel{
		base:     newBase(),
		selected: make(map[int]bool),
	}
}

func (m RemoveFilesModel) Init() tea.Cmd {
	return nil
}

// reload re-reads the server directory after a change on disk.
func (m RemoveFilesModel) reload() RemoveFilesModel {
	entries, err := os.ReadDir(m.dir())
	if err != nil {
		m.err = err
		entries = nil
	}

	m.options = entries
	m.base = m.setCount(len(entries))
	return m
}

func (m RemoveFilesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.RefreshMsg:
		return m.reload(), nil

	case tea.MouseMsg:
		updated, activated := m.handleMouse(msg)
		m.base = updated
		if activated {
			return m.toggle(), nil
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

		case "space":
			return m.toggle(), nil

		case "ctrl+a":
			return m.toggleAll(len(m.options)), nil

		case "enter":
			return m.removeSelected()
		}
	}

	return m, nil
}

// toggle marks or unmarks the entry under the cursor.
func (m RemoveFilesModel) toggle() RemoveFilesModel {
	if len(m.options) > 0 {
		m.selected[m.list.Cursor] = !m.selected[m.list.Cursor]
	}
	return m
}

// toggleAll marks every row, or clears them all when they already are.
func (m RemoveFilesModel) toggleAll(count int) RemoveFilesModel {
	marked := 0
	for i := range count {
		if m.selected[i] {
			marked++
		}
	}

	for i := range count {
		m.selected[i] = marked < count
	}
	return m
}

// removeSelected deletes every marked entry and reloads the listing.
func (m RemoveFilesModel) removeSelected() (tea.Model, tea.Cmd) {
	removed := make([]string, 0, len(m.selected))

	for i, selected := range m.selected {
		if !selected || i < 0 || i >= len(m.options) {
			continue
		}

		name := m.options[i].Name()
		if err := os.RemoveAll(m.scope(nil).Path(name)); err != nil {
			m.err = err
			return m, nil
		}
		removed = append(removed, "Removed "+name)
	}

	m.err = nil
	m.selected = make(map[int]bool)

	return m.reload(), core.Log(removed...)
}

func (m RemoveFilesModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	if len(m.options) == 0 {
		return m.emptyView(m.Title(), "This server's folder is empty.",
			styles.Hint{Key: "backspace", Desc: "back"})
	}

	return m.listView(m.Title(), func(i int) string {
		label := m.options[i].Name()
		if m.options[i].IsDir() {
			label += "/"
		}

		if m.selected[i] {
			label = lipgloss.NewStyle().Foreground(theme.Accent).Render("✓ " + label)
		} else {
			label = "  " + label
		}
		return m.row(i, label, "")
	}, m.Hints()...)
}

// Hints are the keys this panel responds to.
func (m RemoveFilesModel) Hints() []styles.Hint {
	return []styles.Hint{
		{Key: "space", Desc: "mark"},
		{Key: "ctrl+a", Desc: "mark all"},
		{Key: "enter", Desc: "delete marked"},
		{Key: "backspace", Desc: "back"},
	}
}

// additional methods
func (m RemoveFilesModel) Title() string {
	return "Remove Files"
}

func (m RemoveFilesModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(len(m.options))
	return m
}

func (m RemoveFilesModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m RemoveFilesModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	return m.reload()
}
