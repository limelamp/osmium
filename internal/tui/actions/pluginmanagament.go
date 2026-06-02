package actions

import (
	"fmt"
	"os"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// PluginManagement Model

type SessionState int // Define menu options as an enum or constant list to avoid index confusion
const (
	StateOperations SessionState = iota
	StateAdd
	StateRemove
	StateInstall
	StateUpdate
	StateTrack
)

type StateStep int // Define menu options as an enum or constant list to avoid index confusion
const (
	StepSelect StateStep = iota
	StepAction
)

type PluginManagementModel struct {
	layout  core.Layout
	isFocus bool

	state      SessionState
	step       StateStep
	cursor     int
	options    []string
	files      map[int]os.DirEntry
	selected   map[int]bool
	GoBack     bool
	queryInput textinput.Model
	err        error
}

func NewPluginManagementModel() PluginManagementModel {
	ti := textinput.New()
	ti.Placeholder = "Enter plugin id..."
	ti.Focus() // Start with the cursor blinking inside it
	ti.CharLimit = 20
	ti.SetWidth(20)

	return PluginManagementModel{
		cursor:     0,
		step:       0,
		options:    []string{"Add a new plugin", "Remove selected plugins", "Install all added plugins", "Update selected plugins", "Track untracked plugins"},
		GoBack:     false,
		queryInput: ti,
	}
}

// PluginManagement State
func (m PluginManagementModel) Init() tea.Cmd {
	return nil
}

func (m PluginManagementModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case "backspace": // ctrl+backspace = ctrl+h
			m.GoBack = true
			return m, nil
		case "enter":
			switch m.state {
			case StateOperations: // Operations
				m.state = SessionState(m.cursor) + 1 // + 1 compensate
			case StateAdd: // Add
				if err := shared.AddProjectByID(m.queryInput.Value(), "plugins"); err != nil {
					fmt.Println(err)
				} else {
					fmt.Println("Downloaded, good luck lol")
					m.cursor = 0
				}
			case StateRemove: // Remove
				// shared.RemoveProjectByID()
			case StateInstall: // Install
			case StateUpdate: // Update
				m.cursor = 0
				switch m.step {
				case StepSelect:
					m.options = []string{m.queryInput.View(), "All mods"}
				case StepAction:
					if err := shared.UpdateAllProjects("plugins"); err != nil {
						fmt.Println(err)
					}

				}

			case StateTrack: // Track
				shared.TrackProjects()
			}

		}
	}
	var cmd tea.Cmd
	m.queryInput, cmd = m.queryInput.Update(msg)
	return m, cmd
}

// PluginManagement View
func (m PluginManagementModel) View() tea.View {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#05fae6ff")).
		Padding(0, 1)

	// Header
	content := headerStyle.Render(" OSMIUM - PLUGIN MANAGEMENT") + "\n\n"

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))

	// Error display
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		content += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	switch m.state {
	case StateOperations: // Operations
		// Create a simple list
		for i := 0; i < len(m.options); i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}
			content += fmt.Sprintf("%s %s\n", cursor, m.options[i])
		}
	case StateAdd: // Add
		content += "Enter the id/slug of the plugin you want to add and install:\n\n"

		content += m.queryInput.View() + "\n\n"
	case StateRemove: // Remove
		for i := 0; i < len(m.files); i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}

			name := m.files[i].Name()

			if m.selected[i] {
				// Selected: add asterisk and color red
				content += fmt.Sprintf("%s%s\n", cursor, selectedStyle.Render("* "+name))
			} else {
				// Not selected: normal display
				content += fmt.Sprintf("%s  %s\n", cursor, name)
			}
		}
	case StateInstall: // Install
	case StateUpdate: // Update
		content += "Enter the id/slug of the specific plugin you want to update or update all:\n\n"

		for i := 0; i < 2; i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}
			content += fmt.Sprintf("%s %s\n", cursor, m.options[i])
		}
		// s += m.queryInput.View() + "\n\n"
	case StateTrack: // Track
		content += "Tracked mods! \n"
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
func (m PluginManagementModel) Title() string {
	return "Remove Files"
}

func (m PluginManagementModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	return m
}

func (m PluginManagementModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}
