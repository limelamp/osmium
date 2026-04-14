package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/constants"
	"github.com/limelamp/osmium/internal/util"
)

// Setup Model
type SetupModel struct {
	cursor     int
	step       int
	GoBack     bool
	category   string // Vanilla/Simple, Plugin-Based, Mod Loaders, Hybrid
	jarType    string
	jarVersion string
	osmiumConf config.OsmiumConfig
	options    []string
	infoText   string
	textInput  textinput.Model
	topItem    int // The index of the first item currently visible
	viewHeight int // How many items to show at once
	State      int
	err        error
}

func NewSetupModel() SetupModel {
	// textInput creating
	ti := textinput.New()
	ti.Placeholder = "Type \"true\" to accept..."
	ti.Focus()     // Start with the cursor blinking inside it
	ti.Prompt = "" // Remove the ">" out of the way
	ti.CharLimit = 20
	ti.Width = 20
	ti.SetValue("false")

	// terminal width, height
	_, h, _ := term.GetSize(uintptr(os.Stdout.Fd()))

	return SetupModel{
		cursor:     0,
		options:    []string{"Vanilla/Simple", "Plugin-Based", "Mod Loaders", "Hybrid"},
		infoText:   "Choose the type of server you would like to create:",
		textInput:  ti,
		viewHeight: h - 15,
		State:      0,
	}
}

// Setup State
func (m SetupModel) Init() tea.Cmd {
	return nil
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// msg.Width and msg.Height are automatically provided
		m.viewHeight = msg.Height - 15
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}

			// If the cursor goes above the top, scroll up
			if m.cursor < m.topItem {
				m.topItem = m.cursor
			}
		case "down":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}

			// If the cursor goes below the bottom of the window, scroll down
			if m.cursor >= m.topItem+m.viewHeight {
				m.topItem = m.cursor - m.viewHeight + 1
			}
		case "enter":
			switch m.step {
			case 0: // Choose category
				m.category = m.options[m.cursor] // Save the category

				// Get server types for this category
				serverTypes, ok := constants.CategoryOptions[m.category]
				if !ok {
					m.err = fmt.Errorf("unknown category: %s", m.category)
					return m, nil
				}
				m.osmiumConf.Category = m.category

				// Move to server type selection
				m.step = 1
				m.cursor = 0
				m.options = serverTypes
				m.infoText = fmt.Sprintf("Choose your %s server software:", m.category)

			case 1: // Choose server type
				m.jarType = m.options[m.cursor] // Save the type

				// Get versions for this server type
				// versions, ok := constants.ServerVersions[m.jarType]
				// Get all versions
				versions, err := util.GetVersionStrings("release")
				if err != nil {
					m.err = fmt.Errorf("no versions found for %s", m.jarType)
					return m, nil
				}
				m.osmiumConf.Loader = m.jarType

				// Move to version selection
				m.step = 2
				m.cursor = 0
				m.options = versions
				m.infoText = fmt.Sprintf("Choose the Minecraft version for %s:", m.jarType)

			case 2: // Choose version and begin download
				m.jarVersion = m.options[m.cursor] // Save the version

				// Download the server jar (or installer)
				if err := util.DownloadJar(m.jarType, m.jarVersion); err != nil {
					m.err = err
					return m, nil
				}
				m.osmiumConf.Version = m.jarVersion
				m.osmiumConf.Mods = make(map[string]config.Project)
				m.osmiumConf.Plugins = make(map[string]config.Project)
				if err := config.WriteConfig(&m.osmiumConf); err != nil {
					m.err = err
					return m, nil
				}

				// Move to init prompt
				m.step = 3
				m.cursor = 0
				m.options = []string{"Yes", "No, skip to the dashboard"}
				m.infoText = "Would you like to initialize the files by running the server once?"

				// Fixes an issue where the last step's first lines are still printing until terminal height is updated
				return m, tea.ClearScreen
			case 3: // Init files prompt
				switch m.cursor {
				case 0:
					m.step = 4
					m.cursor = 0
					m.options = []string{}
					m.infoText = "Do you agree to Mojang's EULA? More info at: https://aka.ms/MinecraftEULA\nPlease type \"true\" in order to agree."
				case 1:
					m.GoBack = true
				}

			case 4: // EULA agreement and first run
				switch m.textInput.Value() {
				case "true":
					content := "#By changing the setting below to TRUE you are indicating your agreement to our EULA (https://aka.ms/MinecraftEULA).\n"
					content += "eula=true\n"

					if err := os.WriteFile("eula.txt", []byte(content), 0644); err != nil {
						m.err = err
						return m, nil
					}

					// Set the state to that of RunServer's in app.go
					m.State = 1

				}
			}
		}

	}

	// IMPORTANT: Update the internal textinput model
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

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
		end := m.topItem + m.viewHeight

		// SAFETY CHECK: Cap 'end' at the slice length
		if end > len(m.options) {
			end = len(m.options)
		}

		for i := m.topItem; i < end; i++ {
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
