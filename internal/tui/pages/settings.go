package pages

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/build"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// settingsSections are the groups listed in the left container.
var settingsSections = []string{"Appearance", "Defaults", "About"}

const (
	sectionAppearance = iota
	sectionDefaults
	sectionAbout
)

// sectionPaneWidth is the outer width of the sections container, matching the
// proportions the page had when both columns shared one box.
const sectionPaneWidth = 24

// Rows of the Defaults section, in display order.
const (
	defaultServerDir = iota
	defaultRAM
	defaultsCount
)

// settingsField names which text field currently has the keyboard.
type settingsField int

const (
	fieldNone settingsField = iota
	fieldDir
	fieldRAM
)

// SettingsModel edits the application configuration. The left container picks a
// section and the right one edits that section's settings, which is the same
// two-container shape the Manage Servers page uses.
type SettingsModel struct {
	layout core.Layout

	cfg config.AppConfig

	sections core.List
	settings core.List

	// inSection is true once focus has moved from the section container into
	// the settings container.
	inSection bool

	editing settingsField
	dir     core.Field
	ram     core.Field
	warn    string

	// detail scrolls the right-hand pane. Sections with a cursor follow it;
	// About has none, so its own keys move this instead.
	detail core.Scroll
}

// NewSettingsModel returns the settings page, read from the stored config.
func NewSettingsModel() SettingsModel {
	cfg, _ := config.LoadConfig()

	m := SettingsModel{
		cfg: cfg,
		dir: core.NewField(cfg.ServersDir()).WithLimit(300),
		ram: core.NewField("e.g. 6G, 10G or 3072M").WithLimit(12),
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

// syncSettings reflows the settings container to the selected section.
func (m SettingsModel) syncSettings() SettingsModel {
	m.settings = m.settings.Resize(m.settingsCount(), max(m.detailLayout().ContentHeight()-2, 1))
	m.detail = core.Scroll{}

	return m.syncDetail()
}

func (m SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.handleMouse(msg), nil

	case tea.KeyPressMsg:
		// An open field owns every key, so typing into it — backspace included
		// — never doubles as navigation.
		if m.editing != fieldNone {
			return m.updateEditor(msg)
		}

		key := msg.String()
		switch key {
		case "esc":
			return m, core.RouteTo("Home")

		// Focus moves between the two containers with tab, exactly as it does
		// between the panes of Manage Servers. The arrow keys only ever move
		// the cursor inside the container that already has focus.
		case "tab", "shift+tab":
			if m.settingsCount() > 0 {
				m.inSection = !m.inSection
			}
			return m, nil
		}

		if !m.inSection {
			if sections, handled := m.sections.HandleKey(key); handled {
				m.sections = sections
				m.settings.Cursor = 0
				return m.syncSettings(), nil
			}

			if key == "enter" && m.settingsCount() > 0 {
				m.inSection = true
			}
			return m, nil
		}

		if settings, handled := m.settings.HandleKey(key); handled {
			m.settings = settings
			return m, nil
		}

		switch key {
		case "enter", "space":
			return m.activate()
		}
	}

	return m, nil
}

// updateEditor routes keys to whichever field is open.
func (m SettingsModel) updateEditor(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var (
		field  core.Field
		result core.FieldResult
		cmd    tea.Cmd
	)

	if m.editing == fieldDir {
		field, result, cmd = m.dir.HandleKey(msg)
		m.dir = field
	} else {
		field, result, cmd = m.ram.HandleKey(msg)
		m.ram = field
	}

	switch result {
	case core.FieldCommitted:
		return m.commit()
	case core.FieldCancelled:
		m.editing = fieldNone
		m.warn = ""
	}

	return m, cmd
}

// commit validates and stores whatever the open field holds.
func (m SettingsModel) commit() (tea.Model, tea.Cmd) {
	if m.editing == fieldRAM {
		amount, err := steps.NormaliseRAM(m.ram.Value())
		if err != nil {
			// The value is wrong, so the field stays open on it to be fixed.
			m.warn = err.Error()

			field, cmd := m.ram.Open(m.ram.Value())
			m.ram = field
			return m, cmd
		}
		m.cfg.DefaultRAM = amount
	} else {
		m.cfg.DefaultServerDir = strings.TrimSpace(m.dir.Value())
	}

	m.warn = ""
	m.editing = fieldNone
	return m, m.save()
}

// activate applies or opens the setting under the cursor.
func (m SettingsModel) activate() (tea.Model, tea.Cmd) {
	switch m.sections.Cursor {
	case sectionAppearance:
		// Update running styles
		selected := theme.ThemeNames[m.settings.Cursor]
		theme.SetTheme(selected)
		m.cfg.Theme = selected

		return m, tea.Batch(m.save(), core.ThemeChanged())

	case sectionDefaults:
		switch m.settings.Cursor {
		case defaultServerDir:
			m.editing = fieldDir

			field, cmd := m.dir.Open(m.cfg.ServersDir())
			m.dir = field
			return m, cmd

		case defaultRAM:
			m.editing = fieldRAM

			field, cmd := m.ram.Open(m.cfg.DefaultRAM)
			m.ram = field
			return m, cmd
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

// handleMouse focuses a container and moves its cursor to the clicked row, or
// scrolls whichever container the wheel was turned over.
func (m SettingsModel) handleMouse(msg tea.MouseMsg) SettingsModel {
	mouse := msg.Mouse()

	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		step := core.WheelStep
		if wheel.Button == tea.MouseWheelUp {
			step = -step
		}

		switch {
		case m.sectionsLayout().Hit(mouse.X, mouse.Y):
			m.sections = m.sections.Move(step)
			m.settings.Cursor = 0
			return m.syncSettings()

		case m.detailLayout().Hit(mouse.X, mouse.Y):
			if m.settingsCount() > 0 {
				m.settings = m.settings.Move(step)
				return m
			}
			m.detail = m.detail.By(step)
		}
		return m
	}

	if sections := m.sectionsLayout(); sections.Hit(mouse.X, mouse.Y) {
		row := mouse.Y - sections.Inner().Y
		if row >= 0 && row < len(settingsSections) {
			m.sections = m.sections.MoveTo(row)
			m.settings.Cursor = 0
			m.inSection = false
			return m.syncSettings()
		}
		return m
	}

	detail := m.detailLayout()
	if !detail.Hit(mouse.X, mouse.Y) {
		return m
	}

	if index, ok := m.settingRows()[mouse.Y-detail.Inner().Y]; ok {
		m.settings = m.settings.MoveTo(index)
		m.inSection = true
	}
	return m
}

// settingRows maps a display row of the settings container to the setting it
// belongs to. The Defaults section puts each value on its own line under its
// label, so the rows are not one per setting.
func (m SettingsModel) settingRows() map[int]int {
	rows := make(map[int]int, m.settingsCount())

	switch m.sections.Cursor {
	case sectionAppearance:
		start, end := m.settings.Window()
		for i := start; i < end; i++ {
			rows[i-start] = i
		}
	case sectionDefaults:
		rows[0] = defaultServerDir
		rows[3] = defaultRAM
	}

	return rows
}

func (m SettingsModel) View() tea.View {
	sections := m.sectionsLayout()
	detail := m.detailLayout()

	sectionRows := make([]string, 0, len(settingsSections))
	for i, name := range settingsSections {
		sectionRows = append(sectionRows, styles.ListRow(
			sections.ContentWidth(),
			!m.inSection && i == m.sections.Cursor,
			name, "",
		))
	}

	sectionsPane := styles.Container(styles.ContainerOpts{
		Width:   sections.Width,
		Height:  sections.Height,
		Focused: !m.inSection,
		Title:   "Settings",
		Content: styles.Body(lipgloss.JoinVertical(lipgloss.Left, sectionRows...),
			sections.ContentWidth(), sections.ContentHeight()-1) + "\n" +
			styles.Hints(sections.ContentWidth(), styles.Hint{Key: "tab", Desc: "switch pane"}),
	})

	body, scroll := m.detailBody()

	detailPane := styles.Container(styles.ContainerOpts{
		Width:   detail.Width,
		Height:  detail.Height,
		Focused: m.inSection,
		Title:   settingsSections[m.sections.Cursor],
		Content: styles.Body(body, detail.ContentWidth(), detail.ContentHeight()-1) + "\n" +
			m.hintBar(detail),
		ShowHelp:      true,
		ScrollOffset:  scroll.Offset,
		ScrollTotal:   scroll.Total,
		ScrollVisible: scroll.Visible,
	})

	return tea.NewView(lipgloss.JoinHorizontal(lipgloss.Top, sectionsPane, detailPane))
}

// hintBar renders the settings container's bottom line: the warning when there
// is one, and the keys otherwise.
func (m SettingsModel) hintBar(detail core.Layout) string {
	if m.warn != "" {
		return styles.Error(errText(m.warn))
	}
	return styles.Hints(detail.ContentWidth(), m.hints()...)
}

// hints describe what the settings container responds to right now.
func (m SettingsModel) hints() []styles.Hint {
	switch {
	case m.editing != fieldNone:
		return []styles.Hint{
			{Key: "enter", Desc: "save"},
			{Key: "ctrl+u", Desc: "clear"},
			{Key: "esc", Desc: "cancel"},
		}

	case !m.inSection:
		return []styles.Hint{
			{Key: "↑/↓", Desc: "section"},
			{Key: "tab", Desc: "settings"},
			{Key: "esc", Desc: "home"},
		}

	default:
		return []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "enter", Desc: "apply"},
			{Key: "tab", Desc: "sections"},
			{Key: "esc", Desc: "home"},
		}
	}
}

// ContentAt is the container a point falls in. The section list always fits;
// the detail pane scrolls, so it says how far.
func (m SettingsModel) ContentAt(x, y int) (core.Pane, bool) {
	if sections := m.sectionsLayout(); sections.Hit(x, y) {
		return core.Pane{Bounds: sections.Text()}, true
	}
	if detail := m.detailLayout(); detail.Hit(x, y) {
		_, scroll := m.detailBody()
		return core.Pane{Bounds: detail.Text(), Offset: scroll.Offset}, true
	}
	return core.Pane{}, false
}

// Help lists the keys of the container that holds focus.
func (m SettingsModel) Help() []core.HelpSection {
	pane := "Sections"
	if m.inSection {
		pane = settingsSections[m.sections.Cursor]
	}

	return []core.HelpSection{{Title: "Settings — " + pane, Hints: m.hints()}}
}

// detailBody is the visible part of the section's content, and how much of it
// is off screen. Content taller than the pane scrolls rather than being cut.
func (m SettingsModel) detailBody() (string, core.Scroll) {
	rows := strings.Split(m.detailView(), "\n")

	scroll := m.detail
	if m.settingsCount() > 0 {
		// A section with a cursor follows it; the row it is on is the one that
		// has to stay in view.
		scroll = scroll.Follow(m.cursorRow())
	}

	start, end := scroll.Window()
	return strings.Join(rows[start:min(end, len(rows))], "\n"), scroll
}

// syncDetail records how tall the section's content is, so the keys that scroll
// it have something to scroll within. It is kept here rather than in View,
// which cannot store what it works out.
func (m SettingsModel) syncDetail() SettingsModel {
	rows := strings.Count(m.detailView(), "\n") + 1
	m.detail = m.detail.Resize(rows, max(m.detailLayout().ContentHeight()-1, 1))
	return m
}

// cursorRow is the display row the selected setting is drawn on.
func (m SettingsModel) cursorRow() int {
	for row, index := range m.settingRows() {
		if index == m.settings.Cursor {
			return row
		}
	}
	return 0
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

// appearanceView lists the themes with a live swatch of each palette. Only the
// themes in the scroll window are drawn, so the list stays inside the container
// however many themes are installed.
func (m SettingsModel) appearanceView() string {
	start, end := m.settings.Window()

	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rows = append(rows, m.themeRow(i, theme.ThemeNames[i]))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// themeRow draws one theme with its palette right beside the name, so the
// colours read as belonging to it rather than floating at the far edge.
//
// It is built from independently styled segments instead of styles.ListRow,
// because the swatch has to keep its own colours even on the selected row.
func (m SettingsModel) themeRow(index int, name string) string {
	scheme := theme.Themes[name]

	marker := "  "
	nameStyle := lipgloss.NewStyle().Width(12)
	if m.inSection && index == m.settings.Cursor {
		marker = "➔ "
		nameStyle = nameStyle.Foreground(theme.Primary).Bold(true)
	}

	swatch := lipgloss.NewStyle().Foreground(scheme.Primary).Render("███") +
		lipgloss.NewStyle().Foreground(scheme.Accent).Render("███") +
		lipgloss.NewStyle().Foreground(scheme.Inactive).Render("███")

	badge := ""
	if name == theme.CurrentTheme {
		badge = lipgloss.NewStyle().Foreground(theme.Accent).Render("  active")
	}

	return marker + nameStyle.Render(name) + swatch + badge
}

// defaultsView edits the values the Create Server wizard starts from.
func (m SettingsModel) defaultsView() string {
	width := m.detailLayout().ContentWidth()
	value := lipgloss.NewStyle().Foreground(theme.Accent).PaddingLeft(4)

	dir := value.Render(m.cfg.ServersDir())
	if m.editing == fieldDir {
		dir = lipgloss.NewStyle().PaddingLeft(4).Render(m.dir.View())
	}

	ram := value.Render(defaultOr(m.cfg.DefaultRAM, "4G"))
	if m.editing == fieldRAM {
		ram = lipgloss.NewStyle().PaddingLeft(4).Render(m.ram.View())
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		styles.ListRow(width, m.inSection && m.settings.Cursor == defaultServerDir,
			"Default server folder", ""),
		dir,
		"",
		styles.ListRow(width, m.inSection && m.settings.Cursor == defaultRAM,
			"Default memory", ""),
		ram,
	)
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
		label.Render("Osmium — terminal Minecraft server manager"),
		fmt.Sprintf("%s %s", label.Render("Version:         "), value.Render(build.Info())),
		"",
		fmt.Sprintf("%s %s", label.Render("Config directory:"), value.Render(appDir)),
		fmt.Sprintf("%s %s", label.Render("Server directory:"), value.Render(m.cfg.ServersDir())),
		fmt.Sprintf("%s %s", label.Render("Active theme:   "), value.Render(theme.CurrentTheme)),
	)
}

// Container rectangles. The section column keeps the width it had when both
// columns lived in one box, and the settings container takes the rest.
func (m SettingsModel) sectionsLayout() core.Layout {
	return core.Layout{
		X:      m.layout.X,
		Y:      m.layout.Y,
		Width:  min(sectionPaneWidth, m.layout.Width),
		Height: m.layout.Height,
	}
}

func (m SettingsModel) detailLayout() core.Layout {
	sections := m.sectionsLayout()
	return core.Layout{
		X:      sections.X + sections.Width,
		Y:      sections.Y,
		Width:  m.layout.Width - sections.Width,
		Height: m.layout.Height,
	}
}

func (m SettingsModel) Title() string {
	return "Settings"
}

func (m SettingsModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l

	width := max(m.detailLayout().ContentWidth()-8, 10)
	m.dir = m.dir.SetWidth(width)
	m.ram = m.ram.SetWidth(min(width, 24))

	return m.syncSettings().syncDetail()
}

// defaultOr falls back to a placeholder when a setting has never been set.
func defaultOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// errText lets a plain warning reuse the shared error styling.
type errText string

func (e errText) Error() string { return string(e) }
