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
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"

	"charm.land/lipgloss/v2"
)

// configStep is which of the panel's two nested screens is showing.
type configStep int

const (
	stepPickFile configStep = iota
	stepEditFile
)

type configFile struct {
	path  string
	entry os.DirEntry
}

// ManageConfigsModel browses the config files of the selected server and edits
// their key/value pairs in place.
type ManageConfigsModel struct {
	base

	step     configStep
	fileType string
	fileName string

	options            []configFile
	configOptionKeys   []string
	configOptionValues []string

	// editing is the index of the row currently being typed into, or -1.
	editing   int
	textInput textinput.Model
}

func NewManageConfigsModel() ManageConfigsModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter a value..."
	ti.Prompt = "" // Remove the ">" out of the way
	ti.CharLimit = 500
	ti.SetWidth(30)

	return ManageConfigsModel{
		step:      stepPickFile,
		editing:   -1,
		textInput: ti,
	}
}

// Editing reports whether a config value is being typed into.
func (m ManageConfigsModel) Editing() bool { return m.editing >= 0 }

func (m ManageConfigsModel) Init() tea.Cmd {
	return nil
}

// reload rescans the selected server folder for editable config files.
func (m ManageConfigsModel) reload() ManageConfigsModel {
	m.step = stepPickFile
	m.editing = -1
	m.configOptionKeys = nil
	m.configOptionValues = nil
	m.options = configFilesIn(m.dir())
	m.list = core.List{}
	m.base = m.setCount(len(m.options))
	return m
}

func (m ManageConfigsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		updated, activated := m.handleMouse(msg)
		m.base = updated
		if activated {
			return m.activate()
		}
		return m, nil

	case tea.KeyPressMsg:
		// While a value is being typed the input owns every key except the two
		// that end the edit, so navigation must not steal them.
		if m.editing >= 0 {
			switch msg.String() {
			case "enter":
				return m.commitEdit()
			case "esc":
				m.editing = -1
				m.textInput.Blur()
				return m, nil
			}

			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

		switch msg.String() {
		case "backspace":
			// Nested navigation: leave the editor before leaving the panel.
			if m.step == stepEditFile {
				return m.reload(), nil
			}
			return m, core.CloseAction()

		case "enter":
			return m.activate()
		}
	}

	return m, nil
}

// activate opens the file under the cursor, or starts editing its value.
func (m ManageConfigsModel) activate() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepPickFile:
		if len(m.options) == 0 {
			return m, nil
		}
		return m.openFile(m.options[m.list.Cursor].path)

	case stepEditFile:
		if len(m.configOptionKeys) == 0 {
			return m, nil
		}
		m.editing = m.list.Cursor
		m.textInput.SetValue(m.configOptionValues[m.list.Cursor])
		return m, m.textInput.Focus()
	}

	return m, nil
}

// openFile parses a config file into editable key/value rows.
func (m ManageConfigsModel) openFile(path string) (tea.Model, tea.Cmd) {
	var err error
	switch {
	case strings.HasSuffix(path, ".properties"):
		err = m.loadPropertiesConfig(path)
	case strings.HasSuffix(path, ".yml"):
		err = m.loadYamlConfig(path)
	default:
		return m, nil
	}

	if err != nil {
		m.err = err
		return m, nil
	}

	m.err = nil
	m.step = stepEditFile
	m.list = core.List{}
	m.base = m.setCount(len(m.configOptionKeys))
	return m, nil
}

// commitEdit writes the edited value back to the file on disk.
func (m ManageConfigsModel) commitEdit() (tea.Model, tea.Cmd) {
	m.configOptionValues[m.editing] = m.textInput.Value()
	m.editing = -1
	m.textInput.Blur()

	var lines []string
	for i := range m.configOptionKeys {
		key := m.configOptionKeys[i]
		val := m.configOptionValues[i]

		switch m.fileType {
		case "properties":
			lines = append(lines, fmt.Sprintf("%s=%s", key, val))

		case "yml":
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
	}

	if err := os.WriteFile(m.fileName, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		m.err = err
		return m, nil
	}

	m.err = nil
	return m, core.Log("Saved " + m.fileName)
}

func (m ManageConfigsModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	switch m.step {
	case stepEditFile:
		return m.listView(m.Title()+" · "+filepath.Base(m.fileName), m.editRow,
			styles.Hint{Key: "enter", Desc: "edit value"},
			styles.Hint{Key: "backspace", Desc: "back to files"},
		)

	default:
		if len(m.options) == 0 {
			return m.emptyView(m.Title(), "No .properties or .yml files in this server yet.",
				styles.Hint{Key: "backspace", Desc: "back"})
		}

		return m.listView(m.Title(), func(i int) string {
			return m.row(i, m.options[i].entry.Name(), filepath.Dir(m.options[i].path))
		},
			styles.Hint{Key: "enter", Desc: "open"},
			styles.Hint{Key: "backspace", Desc: "back"},
		)
	}
}

// editRow renders one key/value row of the file being edited.
func (m ManageConfigsModel) editRow(i int) string {
	keyStyle := lipgloss.NewStyle().Foreground(theme.Primary)
	valueStyle := lipgloss.NewStyle().Foreground(theme.Accent)

	separator := "="
	if m.fileType == "yml" {
		separator = ": "
	}

	value := valueStyle.Render(m.configOptionValues[i])
	if m.editing == i {
		value = m.textInput.View()
	}

	return m.row(i, keyStyle.Render(m.configOptionKeys[i])+separator+value, "")
}

// loadPropertiesConfig reads a java .properties file into key/value rows.
func (m *ManageConfigsModel) loadPropertiesConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var keys, values []string
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
		return err
	}

	m.fileType = "properties"
	m.fileName = path
	m.configOptionKeys = keys
	m.configOptionValues = values
	return nil
}

func (m *ManageConfigsModel) loadYamlConfig(path string) error {
	fileType := "yml"
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	m.configOptionKeys = nil
	m.configOptionValues = nil

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
	return nil
}

// configFilesIn walks a server directory for the config formats Osmium can edit.
func configFilesIn(dir string) []configFile {
	extensions := []string{".yml", ".properties"} // Supported config filetypes

	var configEntries []configFile                                                // Entries that are configs
	filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error { // Walking through the whole folder
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
	return "Manage Configs"
}

func (m ManageConfigsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.textInput.SetWidth(max(l.ContentWidth()/2, 10))

	if m.step == stepEditFile {
		m.base = m.setCount(len(m.configOptionKeys))
	} else {
		m.base = m.setCount(len(m.options))
	}
	return m
}

func (m ManageConfigsModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m ManageConfigsModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	return m.reload()
}
