package actions

import (
	"fmt"
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// RunScript Model
type GenRunScriptModel struct {
	layout  core.Layout
	isFocus bool

	cursor  int
	options []string
	GoBack  bool
	err     error
}

func NewGenRunScriptModel() GenRunScriptModel {
	return GenRunScriptModel{
		cursor:  0,
		options: []string{"Recommended settings", "Detailed"},
		GoBack:  false,
	}
}

// RunScript State
func (m GenRunScriptModel) Init() tea.Cmd {
	return nil
}

func (m GenRunScriptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case "enter":
			switch m.cursor {
			case 0: // Recommended settings
				const globalContent = "java -jar -Xms4G server.jar nogui"
				var content []byte
				var outputFile string
				// Create a very basic bash script
				switch runtime.GOOS { // Create different files and contents for different OS
				case "linux":
					content = []byte("#!/bin/bash\n\n" + globalContent)
					outputFile = "run_server.sh"
				case "windows":
					content = []byte(globalContent)
					outputFile = "run_server.bat"
				case "darwin":
					content = []byte("#!/bin/sh\n\n" + globalContent)
					outputFile = "run_server.sh"
				case "freebsd":
					content = []byte("#!/bin/bash\n\n" + globalContent)
					outputFile = "run_server.sh"
				default:
					fmt.Println("Unsupported OS!")
					return m, nil
				}

				// Create the file
				err := os.WriteFile(outputFile, content, 0755)
				if err != nil {
					m.err = err
					return m, nil
				}

				fmt.Println("File Created!")
			}
		}
	}
	return m, nil
}

// RunScript View
func (m GenRunScriptModel) View() tea.View {
	content := ""
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
		content += fmt.Sprintf("%s %s\n", cursor, m.options[i])
	}

	content += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'backspace' to go back.\n\n"

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
func (m GenRunScriptModel) Title() string {
	return "Generate Run Script"
}

func (m GenRunScriptModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	return m
}

func (m GenRunScriptModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}
