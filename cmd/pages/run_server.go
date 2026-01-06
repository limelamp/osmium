package pages

import (
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Data --------------------------------------------------------------------
type RunServerModel struct {
	cursor    int
	options   []string
	textInput textinput.Model
	firstRun  bool
	GoBack    bool
	err       error
}

func InitializedRunServerModel() RunServerModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter server name..."
	ti.Focus() // Start with the cursor blinking inside it
	ti.CharLimit = 500
	ti.Width = 20

	return RunServerModel{
		cursor:    0,
		options:   []string{"Recommended settings", "Detailed"},
		textInput: ti,
		firstRun:  true,
		GoBack:    false,
	}
}

// State --------------------------------------------------------------------------------------------------------
// Handles the dashboard model's data and all actions
func (m RunServerModel) Init() tea.Cmd {

	return nil
}

func (m RunServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.firstRun {
		// Run the server
		javaCMD := exec.Command(
			"java",
			"-jar",
			"-Xms4G",
			"server.jar",
			"nogui",
		)

		// Run in the same directory
		javaCMD.Dir, _ = os.Getwd()

		// Output stuff
		javaCMD.Stdout = os.Stdout
		javaCMD.Stderr = os.Stderr
		javaCMD.Stdin = os.Stdin

		if err := javaCMD.Run(); err != nil {
			m.err = err
		}

		m.firstRun = false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		// case "backspace":
		// 	m.GoBack = true
		// 	return m, nil
		case "enter":
			switch m.textInput.Value() {

			}
		default:

		}
	}

	m.textInput, _ = m.textInput.Update(msg)
	return m, nil
}

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

	s += "> " + m.textInput.Value()
	// s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'backspace' to go back.\n\n"
	return s
}
