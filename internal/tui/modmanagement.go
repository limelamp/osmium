package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/limelamp/osmium/internal/shared"
)

// ModManagement Model
type ModManagementModel struct {
	cursor     int
	options    []string
	step       int
	GoBack     bool
	queryInput textinput.Model
	err        error
}

func NewModManagementModel() ModManagementModel {
	ti := textinput.New()
	ti.Placeholder = "Enter mod id..."
	ti.Focus() // Start with the cursor blinking inside it
	ti.CharLimit = 20
	ti.Width = 20

	return ModManagementModel{
		cursor:     0,
		options:    []string{"Add a new mod", "Remove selected mods", "Install all added mods", "Update selected mods", "Track untracked mods"},
		GoBack:     false,
		queryInput: ti,
	}
}

// ModManagement State
func (m ModManagementModel) Init() tea.Cmd {
	return nil
}

func (m ModManagementModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case "ctrl+h": // ctrl+backspace
			m.GoBack = true
			return m, nil
		case "enter":
			if err := shared.AddProjectByID(m.queryInput.Value(), "mods"); err != nil {
				fmt.Println(err)
			} else {
				fmt.Println("Downloaded, good luck lol")
			}
		}
	}
	var cmd tea.Cmd
	m.queryInput, cmd = m.queryInput.Update(msg)
	return m, cmd
}

// ModManagement View
func (m ModManagementModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#05fae6ff")).
		Padding(0, 1)

	// Header
	s := headerStyle.Render(" OSMIUM - MOD MANAGEMENT") + "\n\n"

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
