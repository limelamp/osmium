// Handles the models' data and all actions

package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/limelamp/osmium/internal/constants"
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
			case 0: // Choose category
				m.category = m.options[m.cursor] // Save the category

				// Get server types for this category
				serverTypes, ok := constants.CategoryOptions[m.category]
				if !ok {
					m.err = fmt.Errorf("unknown category: %s", m.category)
					return m, nil
				}

				// Move to server type selection
				m.step = 1
				m.cursor = 0
				m.options = serverTypes
				m.infoText = fmt.Sprintf("Choose your %s server software:", m.category)

			case 1: // Choose server type
				m.jarType = m.options[m.cursor] // Save the type

				// Get versions for this server type
				versions, ok := constants.ServerVersions[m.jarType]
				if !ok {
					m.err = fmt.Errorf("no versions found for %s", m.jarType)
					return m, nil
				}

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

				// Move to init prompt
				m.step = 3
				m.cursor = 0
				m.options = []string{"Yes", "No, skip to the dashboard"}
				m.infoText = "Would you like to initialize the files by running the server once?"

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

					os.WriteFile("eula.txt", []byte(content), 0644)

					// Get the appropriate run command for this server type
					javaPath, args := util.GetServerRunCommand(m.jarType)

					// Run the server
					javaCMD := exec.Command(javaPath, args...)

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
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
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
		// Set the exec command
		if _, err := os.Stat("run.bat"); err == nil { // If there is an existing run.bat file in the folder (NeoForge)
			switch runtime.GOOS {
			case "windows": // On Windows, run the batch file
				m.javaCMD = exec.Command("cmd", "/c", "run.bat", "nogui")
			case "linux":
				m.javaCMD = exec.Command("bash", "run.sh", "nogui")
			case "darwin":
				m.javaCMD = exec.Command("sh", "run.sh", "nogui")
			case "freebsd":
				m.javaCMD = exec.Command("bash", "run.sh", "nogui")
				// default:
				// 	fmt.Println("Unsupported OS!")
			}
		} else {
			m.javaCMD = exec.Command("java", "-jar", "-Xms4G", "server.jar", "nogui")
		}

		// Point both outputs to our buffer
		m.javaCMD.Stdout = m.output
		m.javaCMD.Stderr = m.output

		m.inputPipe, _ = m.javaCMD.StdinPipe() // This is the "entrance"

		// Start it in the background
		go m.javaCMD.Run()

		m.firstRun = false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// Make sure to kill the java process if ctrl-c is used.
			m.javaCMD.Process.Kill()
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
			// 1. Get the command from the input
			command := m.textInput.Value()

			if m.inputPipe != nil && command != "" {
				// 2. Write it to the server with a newline
				fmt.Fprintln(m.inputPipe, command)
			}

			// 3. Reset the text input for the next command
			m.textInput.Reset()
		default:

		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
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
		case "ctrl+h": // ctrl+backspace
			m.GoBack = true
			return m, nil
		case "enter":
			if err := util.DownloadPluginByID(m.queryInput.Value()); err != nil {
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
