// Basically big text printing functions

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

//* In Go, different types can have methods with the same name, so both SetupModel.Init()
//* and DashboardModel.Init() can coexist without conflict since they're on different receiver types.

// Setup View
func (m SetupModel) View() string {
	// Style your header with Lip Gloss
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#63f456ff")).
		Padding(0, 1)

	categoryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00AAFF")).
		Bold(true)

	s := headerStyle.Render(" OSMIUM - SERVER INITIALIZATION ") + "\n\n"

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		s += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	s += "There appears to be no server initialized in the current folder!" + "\n"
	s += "This setup wizard will be guiding you through the creation of the server." + "\n\n"

	// Show current selections as breadcrumbs
	if m.step > 0 && m.category != "" {
		s += "Category: " + categoryStyle.Render(m.category)
		if m.step > 1 && m.jarType != "" {
			s += " → Software: " + categoryStyle.Render(m.jarType)
		}
		if m.step > 2 && m.jarVersion != "" {
			s += " → Version: " + categoryStyle.Render(m.jarVersion)
		}
		s += "\n"
	}

	s += "\n" + m.infoText + "\n\n"

	// Display options based on step
	// Step 4 is EULA input, so we show text input instead of options
	if m.step != 4 {
		for i := 0; i < len(m.options); i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}
			s += fmt.Sprintf("%s %s\n", cursor, m.options[i])
		}
	} else {
		s += "> eula=" + m.textInput.View()
	}

	s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit.\n\n"
	return s
}

// Dashboard View
func (m DashboardModel) View() string {
	// Style your header with Lip Gloss
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#c256f4ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM - DASHBOARD ") + "\n\n"

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

// RunScript View
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

// RunServer View
func (m RunServerModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#d56509ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM - RUNNING SERVER ") + " Ctrl-C to Exit" + "\n\n"

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		s += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	// Get everything in the bucket
	allLogs := m.output.String()

	// Split into lines
	lines := strings.Split(allLogs, "\n")

	for i := 0; i < len(lines); i++ {
		s += lines[i] + "\n"
	}

	s += "> " + m.textInput.View()
	// s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'backspace' to go back.\n\n"
	return s
}

// ManageConfigs View
func (m ManageConfigsModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#c256f4ff")).
		Padding(0, 1)

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#2ae012ff")).
		Padding(0, 1)

	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ce2614ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM - CREATING A RUN SCRIPT ") + "\n\n"

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		s += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	switch m.step {
	case 0:
		// Create a simple list
		for i := 0; i < len(m.options); i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}
			s += fmt.Sprintf("%s %s\n", cursor, m.options[i])
		}
	case 1:
		for i := 0; i < len(m.configOptionKeys); i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}

			if m.selected == i {
				s += fmt.Sprintf("%s %s=%s\n", cursor, keyStyle.Render(m.configOptionKeys[i]), valueStyle.Render(m.textInput.View()))
			} else {
				s += fmt.Sprintf("%s %s=%s\n", cursor, keyStyle.Render(m.configOptionKeys[i]), valueStyle.Render(m.configOptionValues[i]))
			}

		}

	}

	s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'ctrl+backspace' to go back.\n\n"
	return s
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

// PluginManagement View
func (m PluginManagementModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#05fae6ff")).
		Padding(0, 1)

	// Header
	s := headerStyle.Render(" OSMIUM - PLUGIN MANAGEMENT") + "\n\n"

	// Error display
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

	s += m.queryInput.View() + "\n\n"

	s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'ctrl+backspace' to go back.\n\n"
	return s
}
