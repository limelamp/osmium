// Handles the models' data and all actions

package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/constants"
	"github.com/limelamp/osmium/internal/shared"
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
		if len(m.options) == 0 {
			m.options = []string{""}
		}

		osmiumConf, err := config.ReadConfig()
		if err != nil {
			m.err = err
			return m, nil
		}

		if pid, err := shared.ReadLockPID(); err == nil {
			if shared.IsPIDRunning(pid) {
				m.err = fmt.Errorf("server already running (pid %d). stop it with 'osmium stop'", pid)
				m.firstRun = false
				return m, nil
			}

			if err := shared.RemoveLockFile(); err != nil {
				m.err = err
				return m, nil
			}
		}

		javaPath, args := util.GetServerRunCommand(osmiumConf.Loader)

		m.javaCMD = exec.Command(javaPath, args...)

		m.javaCMD.Dir, _ = os.Getwd()

		// Point both outputs to our buffer
		m.javaCMD.Stdout = m.output
		m.javaCMD.Stderr = m.output

		m.inputPipe, err = m.javaCMD.StdinPipe() // This is the "entrance"
		if err != nil {
			m.err = err
			return m, nil
		}

		if err := m.javaCMD.Start(); err != nil {
			m.err = err
			return m, nil
		}

		if err := shared.WriteLockPID(m.javaCMD.Process.Pid); err != nil {
			m.err = err
			_ = m.javaCMD.Process.Kill()
			return m, nil
		}

		// Start the socket listener in the background
		go shared.StartBasicSocketServer(m.inputPipe)

		go func(cmd *exec.Cmd) {
			_ = cmd.Wait()
			_ = shared.RemoveLockFile()
		}(m.javaCMD)

		m.firstRun = false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// Make sure to kill the java process if ctrl-c is used.
			if m.javaCMD != nil && m.javaCMD.Process != nil {
				_ = m.javaCMD.Process.Kill()
			}

			// Remove the .lock file once the process is killed.
			if err := shared.RemoveLockFile(); err != nil {
				m.err = err
			}

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

// ManageConfigs State
func (m *ManageConfigsModel) loadYamlConfig(path string, fileType string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Inside your scanner loop
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for the YAML separator ": " (colon + space)
		if idx := strings.Index(line, ": "); idx != -1 {
			// We found a key-value pair!
			// Key is everything before ": ", value is everything after.
			key := line[:idx]
			val := strings.TrimSpace(line[idx+2:]) // +2 to skip the ": "

			m.configOptionKeys = append(m.configOptionKeys, key)
			m.configOptionValues = append(m.configOptionValues, val)
		} else if strings.HasSuffix(trimmed, ":") {
			// This is a header/section (it ends in a colon with no value after)
			// We keep the whole line (including leading spaces) as the key
			m.configOptionKeys = append(m.configOptionKeys, line)
			m.configOptionValues = append(m.configOptionValues, "")
		} else if strings.HasPrefix(trimmed, "-") {
			// This is a list item like "- minecraft:lodestone"
			// We treat the whole line as the key and keep the value empty
			m.configOptionKeys = append(m.configOptionKeys, line)
			m.configOptionValues = append(m.configOptionValues, "")
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	m.fileName = path
	m.fileType = fileType
	m.step = 1
	return nil
}

func (m ManageConfigsModel) Init() tea.Cmd {
	return nil
}

func (m ManageConfigsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// msg.Width and msg.Height are automatically provided
		m.viewHeight = msg.Height - 10
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
			switch m.step {
			case 0:
				if m.cursor < len(m.options)-1 {
					m.cursor++
				}
			case 1:
				if m.cursor < len(m.configOptionKeys)-1 {
					m.cursor++
				}
				// If the cursor goes below the bottom of the window, scroll down
				if m.cursor >= m.topItem+m.viewHeight {
					m.topItem = m.cursor - m.viewHeight + 1
				}
			}

		case "ctrl+h": // ctrl + backspace
			m.GoBack = true
			m.cursor = 0
			m.step = 0
			m.selected = -1

			return m, nil
		case "enter":
			switch m.step {
			case 0:
				switch m.cursor {
				case 0: // server.properties

					// Read the file
					var keys []string
					var values []string

					file, err := os.Open("server.properties")
					if err != nil {
						m.err = err
						return m, nil
					}
					defer file.Close()

					scanner := bufio.NewScanner(file)
					for scanner.Scan() {
						line := strings.TrimSpace(scanner.Text())

						// Skip comments and empty lines to keep the UI clean
						if line == "" || strings.HasPrefix(line, "#") {
							continue
						}

						// Split by the first "="
						parts := strings.SplitN(line, "=", 2)
						if len(parts) == 2 {
							keys = append(keys, strings.TrimSpace(parts[0]))
							values = append(values, strings.TrimSpace(parts[1]))
						}
					}

					if err := scanner.Err(); err != nil {
						m.err = err
						return m, nil
					}

					m.fileType = "properties"
					m.fileName = "server.properties"
					m.configOptionKeys = keys
					m.configOptionValues = values
					m.step = 1

				case 1: // bukkit.yml
					if err := m.loadYamlConfig("bukkit.yml", "yml"); err != nil {
						m.err = err
						return m, nil
					}

				case 2: // spigot.yml
					if err := m.loadYamlConfig("spigot.yml", "yml"); err != nil {
						m.err = err
						return m, nil
					}

				case 3: // config/paper-global.yml
					if err := m.loadYamlConfig("./config/paper-global.yml", "yml"); err != nil {
						m.err = err
						return m, nil
					}

				case 4: // config/paper-world-defaults.yml
					if err := m.loadYamlConfig("./config/paper-world-defaults.yml", "yml"); err != nil {
						m.err = err
						return m, nil
					}

				case 5: // purpur.yml
					if err := m.loadYamlConfig("purpur.yml", "yml"); err != nil {
						m.err = err
						return m, nil
					}

				}
			case 1:
				if m.selected == m.cursor { // if this is true, then the same field was selected twice, meaning we can write smth
					switch m.fileType {
					case "properties":
						// 1. Update the local data with the new value from the input
						m.configOptionValues[m.cursor] = m.textInput.Value()

						// 2. Prepare the file content
						var lines []string
						for i := range m.configOptionKeys {
							line := fmt.Sprintf("%s=%s", m.configOptionKeys[i], m.configOptionValues[i])
							lines = append(lines, line)
						}

						// 3. Write to file (0644 is standard permissions)
						output := strings.Join(lines, "\n")
						if err := os.WriteFile(m.fileName, []byte(output), 0644); err != nil {
							m.err = err
							return m, nil
						}

						// 4. Reset selection mode
						m.selected = -1
						m.textInput.Blur() // unfocus

					case "yml":
						// 1. Update the local data with the new value from the input
						m.configOptionValues[m.cursor] = m.textInput.Value()

						// 2. Prepare the file content
						var lines []string
						for i := range m.configOptionKeys {
							key := m.configOptionKeys[i]
							val := m.configOptionValues[i]

							if val == "" {
								// 1. It's a header or list item.
								// Trim trailing spaces first to see what we're working with
								line := strings.TrimRight(key, " ")

								// Only add a colon if it doesn't have one AND isn't a list item
								if !strings.HasSuffix(line, ":") && !strings.Contains(line, "- ") {
									line += ":"
								}
								lines = append(lines, line)
							} else {
								// 2. It's a key-value pair.
								// Make sure we don't have a colon at the end of the key
								// because we are adding ": " manually.
								cleanKey := strings.TrimRight(key, " ")
								cleanKey = strings.TrimSuffix(cleanKey, ":")

								lines = append(lines, fmt.Sprintf("%s: %s", cleanKey, val))
							}
						}

						// 3. Write to file (0644 is standard permissions)
						output := strings.Join(lines, "\n")
						if err := os.WriteFile(m.fileName, []byte(output), 0644); err != nil {
							m.err = err
							return m, nil
						}

						// 4. Reset selection mode
						m.selected = -1
						m.textInput.Blur() // unfocus
					}

				} else { // Select another/new option
					m.selected = m.cursor                                // set the selected variable for the view function
					m.textInput.SetValue(m.configOptionValues[m.cursor]) // give the data of value to textInput
					m.textInput.Focus()                                  // focus after the unfocus
				}

			}

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
			if len(m.options) > 0 {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case "ctrl+a":
			for i := 0; i < len(m.options); i++ {
				m.selected[i] = true
			}
		case "enter":
			for i, selected := range m.selected {
				if !selected || i < 0 || i >= len(m.options) {
					continue
				}

				if err := os.RemoveAll(m.options[i].Name()); err != nil {
					m.err = err
					return m, nil
				}
			}

			m.selected = make(map[int]bool)
			entries, err := os.ReadDir(".")
			if err != nil {
				m.err = err
				return m, nil
			}
			m.options = entries

			if m.cursor >= len(m.options) && len(m.options) > 0 {
				m.cursor = len(m.options) - 1
			}
			if len(m.options) == 0 {
				m.cursor = 0
			}

			m.GoBack = true
			if len(m.options) == 0 {
				m.cursor = 0
			}
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
			if err := shared.AddProjectByID(m.queryInput.Value(), "plugins"); err != nil {
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
