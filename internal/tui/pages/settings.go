package pages

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui/constants"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// settingsSections are the groups listed down the left of the Settings page.
var settingsSections = []string{"Appearance", "Defaults", "About"}

const (
	sectionAppearance = iota
	sectionDefaults
	sectionAbout
)

// sectionPaneWidth is how wide the section column is.
const sectionPaneWidth = 20

// Rows of the Defaults section, in display order.
const (
	defaultServerDir = iota
	defaultRAM
	defaultsCount
)

// SettingsModel edits the application configuration. The left column picks a
// section and the right column edits that section's settings, which is the same
// nested two-pane shape used elsewhere in the app.
type SettingsModel struct {
	layout core.Layout

	cfg config.AppConfig

	sections core.List
	settings core.List

	// inSection is true once focus has moved from the section column into the
	// settings column.
	inSection bool

	// editing is true while the folder input has the keyboard.
	editing bool
	dirIn   textinput.Model

	status string
}

func NewSettingsModel() SettingsModel {
	cfg, _ := config.LoadConfig()

	ti := textinput.New()
	ti.Placeholder = cfg.ServersDir()
	ti.Prompt = "› "
	ti.CharLimit = 300

	m := SettingsModel{
		cfg:   cfg,
		dirIn: ti,
	}
	m.sections = m.sections.Resize(len(settingsSections), len(settingsSections))

	// Match cursor position to currently active global theme
	for i, name := range theme.ThemeNames {
		if name == theme.CurrentTheme {
			m.settings.Cursor = i
			break
		}
	}
	return m.syncSettings()
}

func (m SettingsModel) Init() tea.Cmd {
	return nil
}

// settingsCount is how many editable rows the selected section has.
func (m SettingsModel) settingsCount() int {
	switch m.sections.Cursor {
	case sectionAppearance:
		return len(theme.ThemeNames)
	case sectionDefaults:
		return defaultsCount
	default:
		return 0
	}
}

// syncSettings reflows the settings column to the selected section.
func (m SettingsModel) syncSettings() SettingsModel {
	m.settings = m.settings.Resize(m.settingsCount(), max(m.layout.ContentHeight()-2, 1))
	return m
}

func (m SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.handleMouse(msg), nil

	case tea.KeyPressMsg:
		if m.editing {
			return m.updateEditor(msg)
		}

		key := msg.String()
		if key == "esc" {
			return m, core.RouteTo("Home")
		}

		if !m.inSection {
			if sections, handled := m.sections.HandleKey(key); handled {
				m.sections = sections
				m.settings.Cursor = 0
				return m.syncSettings(), nil
			}

			if key == "enter" || key == "right" || key == "l" {
				if m.settingsCount() > 0 {
					m.inSection = true
				}
			}
			return m, nil
		}

		if settings, handled := m.settings.HandleKey(key); handled {
			m.settings = settings
			return m, nil
		}

		switch key {
		case "left", "h", "backspace":
			m.inSection = false
			return m, nil

		case "enter", " ":
			return m.activate()
		}
	}

	return m, nil
}

// updateEditor routes keys to the folder input while it is open.
func (m SettingsModel) updateEditor(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.cfg.DefaultServerDir = strings.TrimSpace(m.dirIn.Value())
		m.editing = false
		m.dirIn.Blur()
		return m, m.save()

	case "esc":
		m.editing = false
		m.dirIn.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.dirIn, cmd = m.dirIn.Update(msg)
	return m, cmd
}

// activate applies the setting under the cursor.
func (m SettingsModel) activate() (tea.Model, tea.Cmd) {
	switch m.sections.Cursor {
	case sectionAppearance:
		// Update running styles
		selected := theme.ThemeNames[m.settings.Cursor]
		theme.SetTheme(selected)
		m.cfg.Theme = selected
		return m, m.save()

	case sectionDefaults:
		switch m.settings.Cursor {
		case defaultServerDir:
			m.editing = true
			m.dirIn.SetValue(m.cfg.ServersDir())
			return m, m.dirIn.Focus()

		case defaultRAM:
			m.cfg.DefaultRAM = nextRAM(m.cfg.DefaultRAM)
			return m, m.save()
		}
	}

	return m, nil
}

// save persists the configuration to disk.
func (m SettingsModel) save() tea.Cmd {
	if err := config.SaveConfig(m.cfg); err != nil {
		return core.Log("failed to save settings: " + err.Error())
	}
	return nil
}

// nextRAM steps to the following preset memory size, wrapping around.
func nextRAM(current string) string {
	for i, option := range constants.RamOptions {
		if option == current {
			return constants.RamOptions[(i+1)%len(constants.RamOptions)]
		}
	}
	return constants.RamOptions[0]
}

// handleMouse focuses a column and moves its cursor to the clicked row.
func (m SettingsModel) handleMouse(msg tea.MouseMsg) SettingsModel {
	mouse := msg.Mouse()
	if !m.layout.Hit(mouse.X, mouse.Y) {
		return m
	}

	row := m.layout.ContentRow(mouse.Y)
	if row < 0 {
		return m
	}

	// The section column occupies the left edge of the panel; anything to the
	// right of it belongs to the settings column.
	if mouse.X < m.layout.X+2+sectionPaneWidth {
		if row < len(settingsSections) {
			m.sections = m.sections.MoveTo(row)
			m.settings.Cursor = 0
			m.inSection = false
			return m.syncSettings()
		}
		return m
	}

	if row < m.settingsCount() {
		m.settings = m.settings.MoveTo(row)
		m.inSection = true
	}
	return m
}

func (m SettingsModel) View() tea.View {
	sectionRows := make([]string, 0, len(settingsSections))
	for i, name := range settingsSections {
		sectionRows = append(sectionRows, styles.ListRow(
			sectionPaneWidth,
			!m.inSection && i == m.sections.Cursor,
			name, "",
		))
	}

	sectionPane := lipgloss.NewStyle().
		Width(sectionPaneWidth).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(theme.Inactive).
		Height(max(m.layout.ContentHeight()-2, 1)).
		Render(lipgloss.JoinVertical(lipgloss.Left, sectionRows...))

	detailPane := lipgloss.NewStyle().
		PaddingLeft(2).
		Width(m.detailWidth() + 2).
		Render(m.detailView())

	// Pad the body so the hint bar lands on the panel's last row.
	body := styles.Body(lipgloss.JoinHorizontal(lipgloss.Top, sectionPane, detailPane), m.layout.ContentHeight()-1)

	hints := styles.Hints(m.layout.ContentWidth(),
		styles.Hint{Key: "↑/↓", Desc: "move"},
		styles.Hint{Key: "→/enter", Desc: "edit"},
		styles.Hint{Key: "←", Desc: "sections"},
		styles.Hint{Key: "esc", Desc: "home"},
	)

	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:    m.layout.Width,
		Height:   m.layout.Height,
		Focused:  true,
		Title:    "Settings",
		Content:  body + "\n" + hints,
		ShowHelp: true,
	}))
}

// detailWidth is how many columns the settings column has for its own content,
// after the section column and the gap between them.
func (m SettingsModel) detailWidth() int {
	return max(m.layout.ContentWidth()-sectionPaneWidth-5, 20)
}

// detailView renders the selected section's settings.
func (m SettingsModel) detailView() string {
	switch m.sections.Cursor {
	case sectionAppearance:
		return m.appearanceView()
	case sectionDefaults:
		return m.defaultsView()
	default:
		return m.aboutView()
	}
}

// appearanceView lists the themes with a live swatch of each palette.
func (m SettingsModel) appearanceView() string {
	rows := []string{
		lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("Theme"),
		"",
	}

	for i, name := range theme.ThemeNames {
		scheme := theme.Themes[name]
		swatch := lipgloss.NewStyle().Foreground(scheme.Primary).Render("███") +
			lipgloss.NewStyle().Foreground(scheme.Accent).Render("███") +
			lipgloss.NewStyle().Foreground(scheme.Inactive).Render("███")

		// The badge column is always the same width so the swatches line up
		// whether or not a row is the active theme.
		badge := lipgloss.NewStyle().Foreground(theme.Accent).Width(8).Align(lipgloss.Right)
		suffix := swatch + badge.Render("")
		if name == theme.CurrentTheme {
			suffix = swatch + badge.Render("active")
		}

		rows = append(rows, styles.ListRow(
			m.detailWidth(),
			m.inSection && i == m.settings.Cursor,
			name, suffix,
		))
	}

	rows = append(rows, "", lipgloss.NewStyle().Foreground(theme.Inactive).
		Render("Applied immediately and saved to config.json."))

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// defaultsView edits the values the Create Server wizard starts from.
func (m SettingsModel) defaultsView() string {
	width := m.detailWidth()

	dirValue := m.cfg.ServersDir()
	if m.editing {
		dirValue = m.dirIn.View()
	}

	ram := m.cfg.DefaultRAM
	if ram == "" {
		ram = constants.RamOptions[0]
	}

	rows := []string{
		lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("Defaults"),
		"",
		styles.ListRow(width, m.inSection && m.settings.Cursor == defaultServerDir,
			"Default server folder", ""),
		lipgloss.NewStyle().Foreground(theme.Accent).PaddingLeft(4).Render(dirValue),
		"",
		styles.ListRow(width, m.inSection && m.settings.Cursor == defaultRAM,
			"Default memory", lipgloss.NewStyle().Foreground(theme.Accent).Render(ram)),
		"",
		lipgloss.NewStyle().Foreground(theme.Inactive).
			Render("The wizard's \"Default folder\" option creates servers here."),
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// aboutView shows where Osmium keeps its files.
func (m SettingsModel) aboutView() string {
	appDir, err := config.GetAppDir()
	if err != nil {
		appDir = "unavailable"
	}

	label := lipgloss.NewStyle().Foreground(theme.Inactive)
	value := lipgloss.NewStyle().Foreground(theme.Accent)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("About"),
		"",
		label.Render("Osmium — terminal Minecraft server manager"),
		"",
		fmt.Sprintf("%s %s", label.Render("Config directory:"), value.Render(appDir)),
		fmt.Sprintf("%s %s", label.Render("Server directory:"), value.Render(m.cfg.ServersDir())),
		fmt.Sprintf("%s %s", label.Render("Active theme:   "), value.Render(theme.CurrentTheme)),
	)
}

func (m SettingsModel) Title() string {
	return "Settings"
}

func (m SettingsModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l
	m.dirIn.SetWidth(max(l.ContentWidth()-sectionPaneWidth-8, 10))
	return m.syncSettings()
}
