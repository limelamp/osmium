package actions

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// RemoveFiles Model
type RemoveFilesModel struct {
	layout core.Layout

	cursor   int
	options  []os.DirEntry
	selected map[int]bool
	isFocus  bool
	err      error
}

func NewRemoveFilesModel() RemoveFilesModel {
	entries, _ := os.ReadDir(".")

	return RemoveFilesModel{
		cursor:   0,
		options:  entries,
		selected: make(map[int]bool),
	}
}

// RemoveFiles State
func (m RemoveFilesModel) Init() tea.Cmd {
	return nil
}

func (m RemoveFilesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "backspace":
			return m, nil
		case "space":
			if len(m.options) > 0 {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case "ctrl+a":
			for i := 0; i < len(m.options); i++ {
				m.selected[i] = true
			}
		case "enter":
			for i, selected := range m.selected {
				if !selected || i < 0 || i >= len(m.options) {
					continue
				}

				if err := os.RemoveAll(m.options[i].Name()); err != nil {
					m.err = err
					return m, nil
				}
			}

			m.selected = make(map[int]bool)
			entries, err := os.ReadDir(".")
			if err != nil {
				m.err = err
				return m, nil
			}
			m.options = entries

			if m.cursor >= len(m.options) && len(m.options) > 0 {
				m.cursor = len(m.options) - 1
			}
			if len(m.options) == 0 {
				m.cursor = 0
			}

			if len(m.options) == 0 {
				m.cursor = 0
			}
		}
	}
	return m, nil
}

// RemoveFiles View
func (m RemoveFilesModel) View() tea.View {
	content := ""
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		content += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	// Create a simple list
	for i := 0; i < len(m.options); i++ {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}

		name := m.options[i].Name()

		if m.selected[i] {
			// Selected: add asterisk and color red
			content += fmt.Sprintf("%s%s\n", cursor, selectedStyle.Render("* "+name))
		} else {
			// Not selected: normal display
			content += fmt.Sprintf("%s  %s\n", cursor, name)
		}
	}

	content += "\n\n" + "Navigate using arrow keys. Press ctrl + a to toggle all. Press 'q' to exit, 'backspace' to go back.\n\n"

	return tea.NewView(styles.Container(
		m.layout.Width,
		m.layout.Height,
		m.isFocus,
		m.Title(),
		content,
		false,
	))

}

// additional methods
func (m RemoveFilesModel) Title() string {
	return "Remove Files"
}

func (m RemoveFilesModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	return m
}

func (m RemoveFilesModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}
