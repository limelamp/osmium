package actions

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

type configFile struct {
	path  string
	entry os.DirEntry
}

// ManageConfigsModel
type ManageConfigsModel struct {
	layout  core.Layout
	isFocus bool

	cursor             int
	fileType           string
	fileName           string
	step               int
	selected           int
	options            []configFile
	configOptionKeys   []string
	configOptionValues []string
	textInput          textinput.Model
	GoBack             bool
	topItem            int // The index of the first item currently visible
	viewHeight         int // How many items to show at once
	err                error
}

func NewManageConfigsModel() ManageConfigsModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter a value..."
	ti.Focus()     // Start with the cursor blinking inside it
	ti.Prompt = "" // Remove the ">" out of the way
	ti.CharLimit = 500
	ti.SetWidth(20)

	// terminal width, height
	_, h, _ := term.GetSize(uintptr(os.Stdout.Fd()))

	return ManageConfigsModel{
		cursor:     0,
		step:       0,
		selected:   -1,
		options:    GetConfigFiles(),
		textInput:  ti,
		GoBack:     false,
		viewHeight: h - 10,
	}
}

// ManageConfigs State
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

			// Reset the second step's keys and values option entries
			m.configOptionKeys = nil
			m.configOptionValues = nil

			return m, nil
		case "enter":
			switch m.step {
			case 0: // The file selecting step
				currentFilePath := m.options[m.cursor].path

				if strings.HasSuffix(currentFilePath, ".properties") { // server.properties
					// Read the file
					var keys []string
					var values []string

					file, err := os.Open(currentFilePath)
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
					m.fileName = currentFilePath
					m.configOptionKeys = keys
					m.configOptionValues = values
					m.step = 1
				} else if strings.HasSuffix(currentFilePath, ".yml") {
					if err := m.loadYamlConfig(currentFilePath); err != nil {
						m.err = err
						return m, nil
					}
				}
			case 1: // The file editing step
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

// ManageConfigs View
func (m ManageConfigsModel) View() tea.View {
	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#2ae012ff"))
		// Padding(0, 1)

	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ce2614ff"))
		// Padding(0, 1)

	// Options
	content := ""

	switch m.step {
	case 0:
		// Create a simple list
		for i := 0; i < len(m.options); i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}
			content += fmt.Sprintf("%s %s\n", cursor, m.options[i].entry.Name())
		}
	case 1:
		end := m.topItem + m.viewHeight

		// SAFETY CHECK: Cap 'end' at the slice length
		if end > len(m.configOptionKeys) {
			end = len(m.configOptionKeys)
		}

		for i := m.topItem; i < end; i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}

			switch m.fileType {
			case "properties":
				if m.selected == i {
					content += fmt.Sprintf("%s %s=%s\n", cursor, keyStyle.Render(m.configOptionKeys[i]), valueStyle.Render(m.textInput.View()))
				} else {
					content += fmt.Sprintf("%s %s=%s\n", cursor, keyStyle.Render(m.configOptionKeys[i]), valueStyle.Render(m.configOptionValues[i]))
				}
			case "yml":
				if m.selected == i {
					content += fmt.Sprintf("%s %s: %s\n", cursor, keyStyle.Render(m.configOptionKeys[i]), valueStyle.Render(m.textInput.View()))
				} else {
					content += fmt.Sprintf("%s %s: %s\n", cursor, keyStyle.Render(m.configOptionKeys[i]), valueStyle.Render(m.configOptionValues[i]))
				}
			}

		}

	}

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

func (m *ManageConfigsModel) loadYamlConfig(path string) error {
	fileType := "yml"
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

func GetConfigFiles() []configFile {
	extensions := []string{".yml", ".properties"} // Supported config filetypes

	var configEntries []configFile                                                // Entries that are configs
	filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error { // Walking through the whole folder
		if err != nil {
			return nil // skip unreadable entries
		}

		if entry.IsDir() {
			return nil // If it is a directory skip it
		}

		for _, ext := range extensions {
			if strings.HasSuffix(entry.Name(), ext) { // Check if found file has the supported filetype
				configEntries = append(configEntries, configFile{path, entry})
				break
			}
		}

		return nil
	})

	return configEntries
}

// additional methods
func (m ManageConfigsModel) Title() string {
	return "Remove Files"
}

func (m ManageConfigsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	return m
}

func (m ManageConfigsModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}
