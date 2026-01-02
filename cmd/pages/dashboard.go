package pages

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Data --------------------------------------------------------------------
// Dashboard DashboardModel datatype to store all the dashboard state/data.
type DashboardModel struct {
	cursor        int
	options       []string
	CurrentAction int
}

func InitializedDashboardModel() DashboardModel {
	return DashboardModel{
		cursor:        0,
		options:       []string{"Create a run script", "Run the server", "Hi", "My", "Name", "Is", "Edwin", "And", "I", "Made", "The", "Mimic"},
		CurrentAction: 0,
	}
}

// State --------------------------------------------------------------------------------------------------------
// Handles the dashboard model's data and all actions
func (m DashboardModel) Init() tea.Cmd {
	return nil
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			} // assuming 5 options
		case "enter":
			m.CurrentAction = m.cursor + 1 // +1 to compensate
		}
	}
	return m, nil
}

// Basically a big print function huh
func (m DashboardModel) View() string {
	// Style your header with Lip Gloss
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#c256f4ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM - DASHBOARD ") + "\n\n"
	s += "Navigate using arrow keys. Press 'q' to exit.\n\n"

	// Create a simple list
	for i := 0; i < len(m.options); i++ {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		s += fmt.Sprintf("%s %s\n", cursor, m.options[i])
	}

	return s
}
