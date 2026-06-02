package actions

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// ModManagement Model
type ModManagementModel struct {
	layout  core.Layout
	isFocus bool

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
	ti.SetWidth(20)

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
func (m ModManagementModel) View() tea.View {
	content := ""

	// Error display
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

	content += m.queryInput.View() + "\n\n"

	content += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'ctrl+backspace' to go back.\n\n"

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
func (m ModManagementModel) Title() string {
	return "Mod Management"
}

func (m ModManagementModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	return m
}

func (m ModManagementModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}
