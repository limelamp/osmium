package pages

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Data --------------------------------------------------------------------
type RemoveFilesModel struct {
	cursor   int
	options  map[int]os.DirEntry
	selected map[int]bool
	GoBack   bool
	err      error
}

func NewRemoveFilesModel() RemoveFilesModel {
	entries, _ := os.ReadDir(".")
	options := make(map[int]os.DirEntry)
	for index, value := range entries {
		options[index] = value
	}

	return RemoveFilesModel{
		cursor:   0,
		options:  options,
		selected: make(map[int]bool),
		GoBack:   false,
	}
}

// State --------------------------------------------------------------------------------------------------------
// Handles the dashboard model's data and all actions
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
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "ctrl+a":
			for i := 0; i < len(m.options); i++ {
				m.selected[i] = !m.selected[i]
			}
		//! panic when deleting the second time in a single session, the cause is cursor index misalign with the m.options map
		case "enter":
			for key, value := range m.selected {
				if value {
					os.RemoveAll(m.options[key].Name())
					delete(m.options, key)
				}
			}
			time.Sleep(2 * time.Second)
			m.selected = make(map[int]bool)
			entries, _ := os.ReadDir(".")
			m.options = make(map[int]os.DirEntry)
			for index, value := range entries {
				m.options[index] = value
			}
			m.GoBack = true
			m.cursor = 0
		}
	}
	return m, nil
}

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
