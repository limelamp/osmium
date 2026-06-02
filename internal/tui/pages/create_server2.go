package pages

// import (
// 	"strings"

// 	tea "charm.land/bubbletea/v2"
// 	"charm.land/lipgloss/v2"
// 	"github.com/limelamp/osmium/internal/tui/core"
// 	"github.com/limelamp/osmium/internal/tui/styles"
// )

// type CreateServerModel struct {
// 	layout core.Layout

// 	// Active focus index (0: Software, 1: Version, 2: RAM, 3: Eula/Rules, 4: Submit Buttons)
// 	focusIndex int

// 	// Form State
// 	selectedSoftware string
// 	selectedVersion  string
// 	selectedRAM      string
// 	eulaAccepted     bool
// 	startImmediately bool

// 	// UI Cursors
// 	softwareCursor int
// 	versionCursor  int
// 	ramCursor      int
// 	rulesCursor    int // 0: Eula checkbox, 1: Auto-start checkbox
// 	buttonCursor   int // 0: Create, 1: Cancel
// }

// func NewCreateServerModel() CreateServerModel {
// 	return CreateServerModel{
// 		focusIndex:       0,
// 		selectedSoftware: "Vanilla",
// 		selectedVersion:  "1.21",
// 		selectedRAM:      "4 GB",
// 	}
// }

// func (m CreateServerModel) Init() tea.Cmd {
// 	return nil
// }

// func (m CreateServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
// 	switch msg := msg.(type) {
// 	case tea.KeyPressMsg:
// 		switch msg.String() {
// 		case "tab":
// 			// Cycle active focus panel between 0 and 4
// 			m.focusIndex = (m.focusIndex + 1) % 5
// 			return m, nil

// 		case "shift+tab":
// 			m.focusIndex = (m.focusIndex - 1 + 5) % 5
// 			return m, nil

// 		case "up", "k":
// 			m.handleUp()
// 		case "down", "j":
// 			m.handleDown()

// 		case "enter", " ":
// 			return m.handleAction()
// 		}
// 	}
// 	return m, nil
// }

// // Handle vertical navigation within the currently focused panel
// func (m *CreateServerModel) handleUp() {
// 	switch m.focusIndex {
// 	case 0: // Software Column
// 		if m.softwareCursor > 0 {
// 			m.softwareCursor--
// 			m.selectedSoftware = softwares[m.softwareCursor]
// 			// Reset version cursor when software changes to prevent index out of bounds
// 			m.versionCursor = 0
// 			m.selectedVersion = getVersions(m.selectedSoftware)[0]
// 		}
// 	case 1: // Version Column
// 		if m.versionCursor > 0 {
// 			m.versionCursor--
// 			m.selectedVersion = getVersions(m.selectedSoftware)[m.versionCursor]
// 		}
// 	case 2: // RAM Column
// 		if m.ramCursor > 0 {
// 			m.ramCursor--
// 			m.selectedRAM = ramOptions[m.ramCursor]
// 		}
// 	case 3: // Rules Column
// 		if m.rulesCursor > 0 {
// 			m.rulesCursor--
// 		}
// 	case 4: // Buttons
// 		m.buttonCursor = (m.buttonCursor - 1 + 2) % 2
// 	}
// }

// func (m *CreateServerModel) handleDown() {
// 	switch m.focusIndex {
// 	case 0:
// 		if m.softwareCursor < len(softwares)-1 {
// 			m.softwareCursor++
// 			m.selectedSoftware = softwares[m.softwareCursor]
// 			m.versionCursor = 0
// 			m.selectedVersion = getVersions(m.selectedSoftware)[0]
// 		}
// 	case 1:
// 		versions := getVersions(m.selectedSoftware)
// 		if m.versionCursor < len(versions)-1 {
// 			m.versionCursor++
// 			m.selectedVersion = versions[m.versionCursor]
// 		}
// 	case 2:
// 		if m.ramCursor < len(ramOptions)-1 {
// 			m.ramCursor++
// 			m.selectedRAM = ramOptions[m.ramCursor]
// 		}
// 	case 3:
// 		if m.rulesCursor < 1 {
// 			m.rulesCursor++
// 		}
// 	case 4:
// 		m.buttonCursor = (m.buttonCursor + 1) % 2
// 	}
// }

// func (m CreateServerModel) handleAction() (tea.Model, tea.Cmd) {
// 	if m.focusIndex == 3 {
// 		// Toggle checkboxes in Rules column
// 		if m.rulesCursor == 0 {
// 			m.eulaAccepted = !m.eulaAccepted
// 		} else {
// 			m.startImmediately = !m.startImmediately
// 		}
// 	} else if m.focusIndex == 4 {
// 		// Submit or cancel
// 		if m.buttonCursor == 0 {
// 			if m.eulaAccepted {
// 				// Process creation, then route to servers
// 				return m, core.RouteTo("ManageServers")
// 			}
// 			// (Optionally track/render a warning string if EULA is false)
// 		} else {
// 			return m, core.RouteTo("Home")
// 		}
// 	}
// 	return m, nil
// }

// func (m CreateServerModel) View() tea.View {
// 	if m.layout.Width == 0 {
// 		return tea.NewView("loading...")
// 	}

// 	// Calculate proportional column widths
// 	colWidth := (m.layout.Width - 6) / 3

// 	col1 := m.renderSoftwareColumn(colWidth)
// 	col2 := m.renderVersionColumn(colWidth)
// 	col3 := m.renderRulesColumn(colWidth)

// 	// Join the columns horizontally
// 	columns := lipgloss.JoinHorizontal(
// 		lipgloss.Top,
// 		col1,
// 		"  ", // spacers
// 		col2,
// 		"  ",
// 		col3,
// 	)

// 	// Render Action Buttons
// 	var createBtn, cancelBtn string
// 	if m.focusIndex == 4 && m.buttonCursor == 0 {
// 		createBtn = lipgloss.NewStyle().Foreground(styles.Primary).Bold(true).Render("[ Build Server ]")
// 	} else {
// 		createBtn = "  Build Server  "
// 	}

// 	if m.focusIndex == 4 && m.buttonCursor == 1 {
// 		cancelBtn = lipgloss.NewStyle().Foreground(styles.Primary).Bold(true).Render("[ Cancel ]")
// 	} else {
// 		cancelBtn = "  Cancel  "
// 	}

// 	buttons := lipgloss.JoinHorizontal(0, "  ", createBtn, "    ", cancelBtn)

// 	// Combine all elements vertically
// 	content := lipgloss.JoinVertical(
// 		lipgloss.Left,
// 		columns,
// 		"\n----------------------------------------------------------------------\n",
// 		buttons,
// 	)

// 	return tea.NewView(content)
// }

// // Individual column view builders (which visually highlight when they have focus)

// func (m CreateServerModel) renderSoftwareColumn(width int) string {
// 	var b strings.Builder
// 	titleStyle := lipgloss.NewStyle().Bold(true)
// 	if m.focusIndex == 0 {
// 		titleStyle = titleStyle.Foreground(styles.Primary)
// 	}

// 	b.WriteString(titleStyle.Render("[ ENGINE SOFTWARE ]") + "\n\n")

// 	for i, sw := range softwares {
// 		if i == m.softwareCursor {
// 			marker := "  "
// 			if m.focusIndex == 0 {
// 				marker = "➔ "
// 			}
// 			b.WriteString(lipgloss.NewStyle().Foreground(styles.Primary).Bold(true).Render(marker+sw) + "\n")
// 		} else {
// 			b.WriteString("  " + sw + "\n")
// 		}
// 	}

// 	return lipgloss.NewStyle().Width(width).Render(b.String())
// }

// func (m CreateServerModel) renderVersionColumn(width int) string {
// 	var b strings.Builder
// 	titleStyle := lipgloss.NewStyle().Bold(true)
// 	if m.focusIndex == 1 {
// 		titleStyle = titleStyle.Foreground(styles.Primary)
// 	}

// 	b.WriteString(titleStyle.Render("[ SELECTION VERSION ]") + "\n\n")

// 	versions := getVersions(m.selectedSoftware)
// 	for i, v := range versions {
// 		if i == m.versionCursor {
// 			marker := "  "
// 			if m.focusIndex == 1 {
// 				marker = "➔ "
// 			}
// 			b.WriteString(lipgloss.NewStyle().Foreground(styles.Primary).Bold(true).Render(marker+v) + "\n")
// 		} else {
// 			b.WriteString("  " + v + "\n")
// 		}
// 	}

// 	return lipgloss.NewStyle().Width(width).Render(b.String())
// }

// func (m CreateServerModel) renderRulesColumn(width int) string {
// 	var b strings.Builder
// 	titleStyle := lipgloss.NewStyle().Bold(true)
// 	if m.focusIndex == 2 || m.focusIndex == 3 {
// 		titleStyle = titleStyle.Foreground(styles.Primary)
// 	}

// 	b.WriteString(titleStyle.Render("[ SPECS & SETTINGS ]") + "\n\n")

// 	// 1. Memory Cap Selection (always displays active configuration value)
// 	b.WriteString("Allocated Memory: " + lipgloss.NewStyle().Foreground(styles.Accent).Render(m.selectedRAM) + "\n")
// 	b.WriteString(lipgloss.NewStyle().Foreground(styles.Inactive).Render("  (Use Tab to select RAM column directly)") + "\n\n")

// 	// 2. Agreements Toggles
// 	eulaCheck := "[ ]"
// 	if m.eulaAccepted {
// 		eulaCheck = "[X]"
// 	}
// 	if m.focusIndex == 3 && m.rulesCursor == 0 {
// 		b.WriteString(lipgloss.NewStyle().Foreground(styles.Primary).Bold(true).Render("➔ "+eulaCheck+" Accept EULA") + "\n")
// 	} else {
// 		b.WriteString("  " + eulaCheck + " Accept EULA\n")
// 	}

// 	startCheck := "[ ]"
// 	if m.startImmediately {
// 		startCheck = "[X]"
// 	}
// 	if m.focusIndex == 3 && m.rulesCursor == 1 {
// 		b.WriteString(lipgloss.NewStyle().Foreground(styles.Primary).Bold(true).Render("➔ "+startCheck+" Auto-Boot Server") + "\n")
// 	} else {
// 		b.WriteString("  " + startCheck + " Auto-Boot Server\n")
// 	}

// 	return lipgloss.NewStyle().Width(width).Render(b.String())
// }

// func (m CreateServerModel) Title() string {
// 	return "Create Server"
// }

// func (m CreateServerModel) SetLayout(l core.Layout) tea.Model {
// 	m.layout = l
// 	return m
// }

// var softwares = []string{"Vanilla", "Fabric", "Forge", "Paper", "Purpur"}
// var softwareDesc = map[string]string{
// 	"Vanilla": "The official, unmodified Minecraft server software.",
// 	"Fabric":  "A lightweight, modular modding toolchain for modern versions.",
// 	"Forge":   "The classic, heavy-duty modding platform for legacy and custom packs.",
// 	"Paper":   "High-performance server built for plugins and public play.",
// 	"Purpur":  "A Paper drop-in replacement designed for ultimate customizability.",
// }

// func getVersions(software string) []string {
// 	switch software {
// 	case "Vanilla":
// 		return []string{"1.21", "1.20.6", "1.20.4", "1.19.4"}
// 	case "Fabric":
// 		return []string{"1.21 (Fabric)", "1.20.4 (Fabric)", "1.20.1 (Fabric)"}
// 	case "Forge":
// 		return []string{"1.20.1 (Forge)", "1.19.2 (Forge)", "1.12.2 (Forge)"}
// 	default:
// 		return []string{"1.21", "1.20.4", "1.19.4"}
// 	}
// }

// var ramOptions = []string{"2 GB", "4 GB", "6 GB", "8 GB", "12 GB", "16 GB"}
