package tui

import (
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/limelamp/osmium/internal/util"
)

// Setup State ----------------------------------------------------------------------------------
// Handles the setup model's data and all actions
func (m SetupModel) Init() tea.Cmd {
	return nil
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.options) - 1
			}
		case "down":
			// assuming 4 options
			if m.cursor < len(m.options)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case "enter":
			switch m.step {
			case 0:
				m.jarType = m.options[m.cursor] // Save the type

				// Now change the options to Versions
				m.step = 1
				m.cursor = 0
				m.options = []string{"1.21.11", "1.21", "1.20", "1.19"}
				m.infoText = "Choose the Minecraft version:"

			case 1: // choose version and begin download
				m.jarVersion = m.options[m.cursor] // Save the version

				// return m, downloadServerJar(m.serverType, m.version)
				if err := util.DownloadJar(m.jarType, m.jarVersion); err != nil {
					m.err = err
					return m, nil
				}

				// Move to download state
				m.step = 2
				m.cursor = 0
				m.options = []string{"Yes", "No, skip to the dashboard"}
				m.infoText = "Would you like to initiliaze the files by running the server once?"
			case 2: // init files
				switch m.cursor {
				case 0:
					m.step = 3
					m.cursor = 0
					m.options = []string{}
					m.infoText = "Do you agree to Mojang's EULA? More info at: https://aka.ms/MinecraftEULA\nPlease type \"true\" in order to agree."
				case 1:
					m.GoBack = true
				}

			case 3:
				switch m.textInput.Value() {
				case "true":
					content := "#By changing the setting below to TRUE you are indicating your agreement to our EULA (https://aka.ms/MinecraftEULA).\n"
					content += "eula=true\n"

					os.WriteFile("eula.txt", []byte(content), 0644)

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
				}
			}
		}

	}

	// IMPORTANT: Update the internal textinput model
	// var cmd tea.Cmd
	m.textInput, _ = m.textInput.Update(msg)
	return m, nil
}
