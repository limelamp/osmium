// Handles the models' data and all actions

package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/limelamp/osmium/internal/util"
)

//* In Go, different types can have methods with the same name, so both SetupModel.Init()
//* and DashboardModel.Init() can coexist without conflict since they're on different receiver types.

// Setup State
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

// Dashboard State
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

// RunScript State
func (m RunScriptModel) Init() tea.Cmd {
	return nil
}

func (m RunScriptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

// RunServer State
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

// RemoveFiles State
func (m RemoveFilesModel) Init() tea.Cmd {
	return nil
}

func (m RemoveFilesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "ctrl+a":
			for i := 0; i < len(m.options); i++ {
				m.selected[i] = !m.selected[i]
			}
		//! panic when deleting the second time in a single session, the cause is cursor index misalign with the m.options map
		case "enter":
			for key, value := range m.selected {
				if value {
					os.RemoveAll(m.options[key].Name())
					delete(m.options, key)
				}
			}
			m.selected = make(map[int]bool)
			entries, _ := os.ReadDir(".")
			m.options = make(map[int]os.DirEntry)
			for index, value := range entries {
				m.options[index] = value
			}
			m.GoBack = true
			m.cursor = 0
		}
	}
	return m, nil
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
		case "shift+backspace":
			m.GoBack = true
			return m, nil
		case "enter":
			util.DownloadPluginByID(m.queryInput.Value())
			fmt.Println("Downloaded! Good Luck lol")
		}
	}
	var cmd tea.Cmd
	m.queryInput, cmd = m.queryInput.Update(msg)
	return m, cmd
}
