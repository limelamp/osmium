package actions

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// configStep is which of the panel's two nested screens is showing.
type configStep int

const (
	stepPickFile configStep = iota
	stepEditFile
)

// The editors a config file can be opened in. Files that have a key/value shape
// get the field editor; anything else is edited a line at a time.
const (
	editorProperties = "properties"
	editorYAML       = "yml"
	editorRaw        = "raw"
)

// configExtensions maps a supported file extension to the editor it opens in.
var configExtensions = map[string]string{
	".properties": editorProperties,
	".yml":        editorYAML,
	".yaml":       editorYAML,
	".json":       editorRaw,
	".toml":       editorRaw,
	".conf":       editorRaw,
	".cfg":        editorRaw,
}

// skippedDirs are the folders never walked when looking for config files. They
// hold world data, downloaded jars and logs — never anything worth editing —
// and walking them makes opening the panel slow on a real server.
var skippedDirs = map[string]bool{
	"world":            true,
	"world_nether":     true,
	"world_the_end":    true,
	"libraries":        true,
	"versions":         true,
	"logs":             true,
	"crash-reports":    true,
	"cache":            true,
	"mods":             true,
	"plugins":          true,
	"migration_backup": true,
}

type configFile struct {
	path   string
	entry  os.DirEntry
	editor string
}

// ManageConfigsModel browses the config files of the selected server and edits
// them: key/value files field by field, everything else line by line.
type ManageConfigsModel struct {
	base

	step     configStep
	fileType string
	fileName string

	options            []configFile
	configOptionKeys   []string
	configOptionValues []string

	// editing is the index of the row currently being typed into, or -1.
	editing int
	field   core.Field

	// A value too long for one line is edited in a wrapped box of its own
	// rather than in a field that has to be scrolled sideways to be read.
	area    textarea.Model
	wrapped bool
}

// NewManageConfigsModel returns the panel that edits a server's config files.
func NewManageConfigsModel() ManageConfigsModel {
	area := textarea.New()
	area.Placeholder = "Enter a value..."
	area.ShowLineNumbers = false
	area.SetHeight(4)

	// The same steady block the single-line fields use.
	areaStyles := textarea.DefaultDarkStyles()
	areaStyles.Cursor.Color = theme.Accent
	areaStyles.Cursor.Blink = false
	area.SetStyles(areaStyles)

	return ManageConfigsModel{
		base:    newBase(),
		step:    stepPickFile,
		editing: -1,
		field:   core.NewField("Enter a value...").WithPrompt("").SetWidth(30),
		area:    area,
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
		if m.editing >= 0 && m.wrapped {
			switch msg.String() {
			case "enter":
				// A config value is one line however it is drawn, so enter
				// still means save rather than a new line.
				return m.commitEdit()
			case "esc":
				m.editing, m.wrapped = -1, false
				m.area.Blur()
				return m, nil
			}

			area, cmd := m.area.Update(msg)
			m.area = area
			return m, cmd
		}

		if m.editing >= 0 {
			field, result, cmd := m.field.HandleKey(msg)
			m.field = field

			switch result {
			case core.FieldCommitted:
				return m.commitEdit()
			case core.FieldCancelled:
				m.editing = -1
			}

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
		return m.openFile(m.options[m.list.Cursor])

	case stepEditFile:
		if len(m.configOptionKeys) == 0 {
			return m, nil
		}
		m.editing = m.list.Cursor
		value := m.configOptionValues[m.list.Cursor]

		// A value that cannot be read in the width its row leaves gets the
		// wrapped editor instead.
		if m.wrapped = len(value) > m.valueWidth(); m.wrapped {
			m.area.SetValue(value)
			return m, m.area.Focus()
		}

		field, cmd := m.field.Open(value)
		m.field = field
		return m, cmd
	}

	return m, nil
}

// openFile parses a config file into editable rows.
func (m ManageConfigsModel) openFile(file configFile) (tea.Model, tea.Cmd) {
	var err error
	switch file.editor {
	case editorProperties:
		err = m.loadPropertiesConfig(file.path)
	case editorYAML:
		err = m.loadYamlConfig(file.path)
	default:
		err = m.loadRawConfig(file.path)
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
	value := m.field.Value()
	if m.wrapped {
		// The box wraps for reading; the value itself stays one line.
		value = strings.Join(strings.Fields(m.area.Value()), " ")
		m.area.Blur()
	}

	m.configOptionValues[m.editing] = value
	m.editing, m.wrapped = -1, false

	var lines []string
	for i := range m.configOptionKeys {
		key := m.configOptionKeys[i]
		val := m.configOptionValues[i]

		switch m.fileType {
		case editorProperties:
			lines = append(lines, fmt.Sprintf("%s=%s", key, val))

		case editorYAML:
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

		default:
			// A raw file is stored one whole line per row, so it is written
			// back exactly as it is shown.
			lines = append(lines, val)
		}
	}

	if err := os.WriteFile(m.fileName, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
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

	if m.wrapped {
		return m.panelView(m.Title()+" · "+m.key(m.editing),
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(theme.Inactive).Render("Editing a long value:"),
				"",
				m.area.View(),
			), m.Hints()...)
	}

	switch m.step {
	case stepEditFile:
		return m.listView(m.Title()+" · "+filepath.Base(m.fileName), m.editRow, m.Hints()...)

	default:
		if len(m.options) == 0 {
			return m.emptyView(m.Title(), "No editable config files in this server yet.",
				styles.Hint{Key: "backspace", Desc: "back"})
		}

		return m.listView(m.Title(), func(i int) string {
			return m.row(i, m.options[i].entry.Name(), m.relativeDir(m.options[i].path))
		}, m.Hints()...)
	}
}

// valueWidth is how much room a value has on its row before it is worth
// editing in a box of its own.
func (m ManageConfigsModel) valueWidth() int {
	if m.fileType == editorRaw {
		return max(m.layout.ContentWidth()-4, 20)
	}
	return max(m.layout.ContentWidth()/2, 20)
}

// Hints are the keys this panel responds to, which depend on whether a file is
// open and whether a value is being typed into.
func (m ManageConfigsModel) Hints() []styles.Hint {
	switch {
	case m.editing >= 0:
		return []styles.Hint{
			{Key: "enter", Desc: "save"},
			{Key: "ctrl+u", Desc: "clear"},
			{Key: "esc", Desc: "cancel"},
		}

	case m.step == stepEditFile:
		return []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "enter", Desc: "edit value"},
			{Key: "backspace", Desc: "back to files"},
		}

	default:
		return []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "enter", Desc: "open"},
			{Key: "backspace", Desc: "back"},
		}
	}
}

// relativeDir shows which sub-folder a config file came from, relative to the
// server, so two files with the same name are still tellable apart.
func (m ManageConfigsModel) relativeDir(path string) string {
	relative, err := filepath.Rel(m.dir(), filepath.Dir(path))
	if err != nil || relative == "." {
		return ""
	}
	return relative + "/"
}

// editRow renders one editable row of the file being edited.
func (m ManageConfigsModel) editRow(i int) string {
	value := m.configOptionValues[i]
	if m.editing == i {
		value = m.field.View()
	} else {
		value = lipgloss.NewStyle().Foreground(theme.Accent).Render(value)
	}

	switch m.fileType {
	case editorProperties:
		return m.row(i, m.key(i)+"="+value, "")
	case editorYAML:
		return m.row(i, m.key(i)+": "+value, "")
	default:
		return m.row(i, value, "")
	}
}

// key renders a row's key in the field-editor styles.
func (m ManageConfigsModel) key(i int) string {
	return lipgloss.NewStyle().Foreground(theme.Primary).Render(m.configOptionKeys[i])
}

// loadPropertiesConfig reads a java .properties file into key/value rows.
func (m *ManageConfigsModel) loadPropertiesConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

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

	m.fileType = editorProperties
	m.fileName = path
	m.configOptionKeys = keys
	m.configOptionValues = values
	return nil
}

func (m *ManageConfigsModel) loadYamlConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

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
	m.fileType = editorYAML
	return nil
}

// loadRawConfig reads a file that has no key/value shape — json, toml and the
// rest — as one editable row per line, so it can still be corrected in place
// without Osmium having to understand its format.
func (m *ManageConfigsModel) loadRawConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	m.configOptionKeys = make([]string, len(lines))
	m.configOptionValues = lines
	m.fileName = path
	m.fileType = editorRaw
	return nil
}

// configFilesIn walks a server directory for the config formats Osmium can
// open, skipping the folders that only hold world data, jars and logs.
func configFilesIn(dir string) []configFile {
	var configEntries []configFile // Entries that are configs

	// A folder Osmium cannot read is skipped rather than failing the listing:
	// a server with one unreadable corner still has configs worth editing.
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			//nolint:nilerr // an unreadable entry is skipped, not fatal
			return nil
		}

		if entry.IsDir() {
			name := entry.Name()
			if path != dir && (skippedDirs[name] || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			return nil // If it is a directory skip it
		}

		if editor, ok := configExtensions[strings.ToLower(filepath.Ext(entry.Name()))]; ok {
			configEntries = append(configEntries, configFile{path: path, entry: entry, editor: editor})
		}

		return nil
	})

	return configEntries
}

func (m ManageConfigsModel) Title() string {
	return "Manage Configs"
}

func (m ManageConfigsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l

	// A raw line needs the whole row; a key/value field only needs the value
	// half, because the key is drawn beside it.
	inputWidth := l.ContentWidth() / 2
	if m.fileType == editorRaw {
		inputWidth = l.ContentWidth() - 4
	}
	m.field = m.field.SetWidth(inputWidth)
	m.area.SetWidth(max(l.ContentWidth()-2, 20))
	m.area.SetHeight(max(min(l.ContentHeight()-4, 6), 2))

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
