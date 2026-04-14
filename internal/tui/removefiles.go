package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RemoveFiles Model
type RemoveFilesModel struct {
	cursor   int
	options  []os.DirEntry
	selected map[int]bool
	GoBack   bool
	err      error
}

func NewRemoveFilesModel() RemoveFilesModel {
	entries, _ := os.ReadDir(".")

	return RemoveFilesModel{
		cursor:   0,
		options:  entries,
		selected: make(map[int]bool),
		GoBack:   false,
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
			m.GoBack = true
			return m, nil
		case " ":
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

			m.GoBack = true
			if len(m.options) == 0 {
				m.cursor = 0
			}
		}
	}
	return m, nil
}

// RemoveFiles View
func (m RemoveFilesModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#ff00a6ff")).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))

	s := headerStyle.Render(" OSMIUM - REMOVING FILES ") + "\n\n"

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		s += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
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
			s += fmt.Sprintf("%s%s\n", cursor, selectedStyle.Render("* "+name))
		} else {
			// Not selected: normal display
			s += fmt.Sprintf("%s  %s\n", cursor, name)
		}
	}

	s += "\n\n" + "Navigate using arrow keys. Press ctrl + a to toggle all. Press 'q' to exit, 'backspace' to go back.\n\n"
	return s
}
