package pages

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Data --------------------------------------------------------------------
type RunScriptModel struct {
	cursor  int
	options []string
}

func InitializedRunScriptModel() RunScriptModel {
	return RunScriptModel{
		cursor:  0,
		options: []string{"Recommended settings", "Detailed"},
	}
}

// State --------------------------------------------------------------------------------------------------------
// Handles the dashboard model's data and all actions
func (m RunScriptModel) Init() tea.Cmd {
	return nil
}

func (m RunScriptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		}
	}
	return m, nil
}

func (m RunScriptModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#c256f4ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM - CREATING A RUN SCRIPT ") + "\n\n"

	// Create a simple list
	for i := 0; i < len(m.options); i++ {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		s += fmt.Sprintf("%s %s\n", cursor, m.options[i])
	}

	s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit.\n\n"
	return s
}
