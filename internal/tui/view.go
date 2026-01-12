// Basically big text printing functions

package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

//* In Go, different types can have methods with the same name, so both SetupModel.Init()
//* and DashboardModel.Init() can coexist without conflict since they're on different receiver types.

func (m SetupModel) View() string {
	// Style your header with Lip Gloss
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#63f456ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM - SERVER INITIALIZATION ") + "\n\n"

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		s += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	s += "There appears to be no server initialized in the current folder!" + "\n"
	s += "This setup wizard will be guiding you through the creation of the server." + "\n\n"

	s += "\n\n" + m.infoText + "\n\n"

	switch m.step {
	case 0:

	}
	// Create a simple list
	// serverTypes := [length]string{"Vanilla", "Bukkit", "Spigot", "Paper", "Purpur"}
	if m.step != 3 {
		for i := 0; i < len(m.options); i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}
			s += fmt.Sprintf("%s %s\n", cursor, m.options[i])
		}
	} else {
		s += "> eula=" + m.textInput.Value()
	}

	s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit.\n\n"
	return s
}

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

func (m RunScriptModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#c256f4ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM - CREATING A RUN SCRIPT ") + "\n\n"

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
		s += fmt.Sprintf("%s %s\n", cursor, m.options[i])
	}

	s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'backspace' to go back.\n\n"
	return s
}
