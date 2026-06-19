package pages

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/constants"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
	"github.com/limelamp/osmium/internal/util"
)

type NextStepMsg struct{}
type PrevStepMsg struct{}

type ServerConfig struct {
	Category string
	Software string
	Version  string
	RAM      string
	Eula     bool
	AutoRun  bool
}

type CreateServerModel struct {
	layout core.Layout
	config *ServerConfig
	steps  []tea.Model
	active int
}

func NewCreateServerModel() CreateServerModel {
	cfg := &ServerConfig{
		Category: "Vanilla",
		Software: "Vanilla",
		Version:  "1.21",
		RAM:      "4 GB",
	}

	m := CreateServerModel{
		config: cfg,
	}
	m.initSteps()
	return m
}

func (m *CreateServerModel) initSteps() {
	m.steps = []tea.Model{
		NewLocationStep("Select Location", []string{"Here", "There", "Whereever"}, func(v string) {
			m.config.RAM = v
		}),

		CategoryEngineStep{config: m.config},

		NewSelectStep("Step 3: Select Version", []string{}, func(v string) {
			m.config.Version = v
		}),

		NewSelectStep("Step 4: Allocate System Memory", constants.RamOptions, func(v string) {
			m.config.RAM = v
		}),

		EulaStep{config: m.config},

		ConfirmStep{config: m.config},

		SuccessStep{config: m.config},
	}
}

func (m CreateServerModel) Init() tea.Cmd {
	return nil
}

func (m CreateServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "backspace" {
			if m.active == 1 {
				stepZero := m.steps[1].(CategoryEngineStep)
				if stepZero.selectingSub {
					var cmd tea.Cmd
					m.steps[1], cmd = m.steps[1].Update(msg)
					return m, cmd
				}
			}

			if m.active > 0 && m.active < len(m.steps)-1 {
				m.active--
				return m, nil
			}
			return m, core.RouteTo("Home")
		}

	case NextStepMsg:
		if m.active < len(m.steps)-1 {
			m.active++
			if m.active == 2 {
				versions, err := util.GetVersionStrings("release")
				if err != nil {
				}

				m.steps[2] = NewSelectStep(
					fmt.Sprintf("Step 2: Select Version for %s", m.config.Software),
					versions,
					func(v string) { m.config.Version = v },
				)
			}
		}
		return m, nil

	case PrevStepMsg:
		if m.active > 0 {
			m.active--
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.steps[m.active], cmd = m.steps[m.active].Update(msg)
	return m, cmd
}

func (m CreateServerModel) View() tea.View {
	if m.layout.Width == 0 {
		return tea.NewView("loading...")
	}

	progressBar := m.renderProgressBar()
	body := m.steps[m.active].View().Content

	helpText := "\n\n" + lipgloss.NewStyle().Foreground(theme.Inactive).Render("  [↑/↓] Navigate  •  [Enter] Select  •  [Backspace] Back")

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		progressBar,
		"",
		body,
		helpText,
	)

	return tea.NewView(
		styles.Container(
			m.layout.Width,
			m.layout.Height,
			true,
			"Setup New Server",
			view,
			true,
		),
	)
}

func (m CreateServerModel) renderProgressBar() string {
	steps := []string{"Location", "Engine", "Version", "Specs", "Finilazing", "Confirm"}
	var renderedSteps []string

	for i, name := range steps {
		stepIdx := i
		var s string
		if stepIdx == m.active {
			s = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(fmt.Sprintf("[%d. %s]", i+1, name))
		} else if stepIdx < m.active {
			s = lipgloss.NewStyle().Foreground(theme.Accent).Render(fmt.Sprintf("✓ %s", name))
		} else {
			s = lipgloss.NewStyle().Foreground(theme.Inactive).Render(fmt.Sprintf("%d. %s", i+1, name))
		}
		renderedSteps = append(renderedSteps, s)
	}

	return "  " + lipgloss.JoinHorizontal(0, strings.Join(renderedSteps, "  ➔  "))
}

func (m CreateServerModel) Title() string {
	return "Create Server"
}

func (m CreateServerModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l
	return m
}

// ==========================================
// subcomponent LocationStep (Step 0)
// ==========================================
type LocationStep struct {
	title    string
	options  []string
	cursor   int
	onSelect func(string)
}

func NewLocationStep(title string, options []string, onSelect func(string)) LocationStep {
	return LocationStep{
		title:    title,
		options:  options,
		onSelect: onSelect,
	}
}

func (s LocationStep) Init() tea.Cmd { return nil }

func (s LocationStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}

		case "down", "j":
			if s.cursor < len(s.options)-1 {
				s.cursor++
			}

		case "enter", " ":
			if len(s.options) > 0 {
				s.onSelect(s.options[s.cursor])
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}
	}
	return s, nil
}

func (s LocationStep) View() tea.View {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(s.title))
	b.WriteString("\n\n")

	for i, opt := range s.options {
		if i == s.cursor {
			b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(
				fmt.Sprintf("  ➔ %s", opt)))
			b.WriteString("\n\n")
		} else {
			b.WriteString(fmt.Sprintf("    %s\n\n", opt))
		}
	}
	return tea.NewView(b.String())
}

// ==========================================
// subcomponent CategoryEngineStep (Steps 2)
// ==========================================

type CategoryEngineStep struct {
	config       *ServerConfig
	categoryIdx  int
	engineIdx    int
	selectingSub bool
}

func (s CategoryEngineStep) Init() tea.Cmd { return nil }

func (s CategoryEngineStep) getEnginesForCategory() []string {
	chosen := constants.Categories[s.categoryIdx]
	switch chosen {
	case "Plugin":
		return constants.PluginOptions

	case "Mod Loader":
		return constants.ModLoaderOptions
	}
	return nil
}

func (s CategoryEngineStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()
		if !s.selectingSub {
			switch key {
			case "up", "k":
				if s.categoryIdx > 0 {
					s.categoryIdx--
				}

			case "down", "j":
				if s.categoryIdx < len(constants.Categories)-1 {
					s.categoryIdx++
				}

			case "enter", " ", "right", "l":
				chosen := constants.Categories[s.categoryIdx]
				s.config.Category = chosen
				if chosen == "Vanilla" {
					s.config.Software = "Vanilla"
					return s, func() tea.Msg { return NextStepMsg{} }
				} else {
					s.selectingSub = true
					s.engineIdx = 0
				}
			}
		} else {
			engines := s.getEnginesForCategory()
			switch key {
			case "up", "k":
				if s.engineIdx > 0 {
					s.engineIdx--
				}

			case "down", "j":
				if s.engineIdx < len(engines)-1 {
					s.engineIdx++
				}

			case "left", "h", "esc", "backspace":
				// Collapse sub-selection back to categories panel
				s.selectingSub = false

			case "enter", " ":
				s.config.Software = engines[s.engineIdx]
				// Complete step and signal parent model to advance
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}
	}
	return s, nil
}

func (s CategoryEngineStep) View() tea.View {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Step 1: Choose Server Engine"))
	b.WriteString("\n\n")

	// Render Left Panel (Categories)
	var leftLines []string
	for i, cat := range constants.Categories {
		if i == s.categoryIdx {
			if s.selectingSub {
				// Category is active, but sub-menu has focus
				leftLines = append(leftLines, lipgloss.NewStyle().Foreground(theme.Accent).Render(fmt.Sprintf("  • %s", cat)))
			} else {
				// Focus is on Category list
				leftLines = append(leftLines, lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(fmt.Sprintf("  ➔ %s", cat)))
			}
		} else {
			leftLines = append(leftLines, fmt.Sprintf("    %s", cat))
		}
	}
	leftPanel := strings.Join(leftLines, "\n\n")

	// Render Right Panel (Sub-Engines)
	var rightPanel string
	chosenCat := constants.Categories[s.categoryIdx]
	if chosenCat != "Vanilla" {
		var rightLines []string
		rightLines = append(rightLines, lipgloss.NewStyle().Bold(true).Underline(true).Render("Available Software:"))
		rightLines = append(rightLines, "")

		engines := s.getEnginesForCategory()
		for i, eng := range engines {
			if s.selectingSub && i == s.engineIdx {
				rightLines = append(rightLines, lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(fmt.Sprintf("  ➔ %s", eng)))
			} else {
				rightLines = append(rightLines, fmt.Sprintf("    %s", eng))
			}
		}
		rightPanel = strings.Join(rightLines, "\n")
	}

	// Join both panels side-by-side
	var layout string
	if rightPanel != "" {
		layout = lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Width(30).Render(leftPanel),
			"      ", // gap spacing
			lipgloss.NewStyle().Render(rightPanel),
		)
	} else {
		layout = leftPanel
	}

	b.WriteString(layout)
	return tea.NewView(b.String())
}

// ==========================================
// subcomponent SelectStep (Steps 3, 4)
// ==========================================

type SelectStep struct {
	title    string
	options  []string
	cursor   int
	onSelect func(string)

	offset  int // index of first visible item
	visible int // how many items to show at once
}

func NewSelectStep(title string, options []string, onSelect func(string)) SelectStep {
	return SelectStep{
		title:    title,
		options:  options,
		onSelect: onSelect,
		visible:  10,
	}
}

func (s SelectStep) Init() tea.Cmd { return nil }

func (s SelectStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
				if s.cursor < s.offset {
					s.offset--
				}
			}

		case "down", "j":
			if s.cursor < len(s.options)-1 {
				s.cursor++
				if s.cursor >= s.offset+s.visible {
					s.offset++
				}
			}

		case "enter", " ":
			if len(s.options) > 0 {
				s.onSelect(s.options[s.cursor])
				return s, func() tea.Msg { return NextStepMsg{} }
			}
		}
	}
	return s, nil
}

func (s SelectStep) View() tea.View {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(s.title))
	b.WriteString("\n\n")

	end := s.offset + s.visible
	if end > len(s.options) {
		end = len(s.options)
	}

	for i, opt := range s.options[s.offset:end] {
		actualIdx := s.offset + i
		if actualIdx == s.cursor {
			b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(
				fmt.Sprintf("  ➔ %s", opt)))
			b.WriteString("\n\n")
		} else {
			b.WriteString(fmt.Sprintf("    %s\n\n", opt))
		}
	}
	return tea.NewView(b.String())
}

// ==========================================
// subcomponent EulaStep (Step 5)
// ==========================================

type EulaStep struct {
	config *ServerConfig
	cursor int
	warn   string
}

func (e EulaStep) Init() tea.Cmd { return nil }

func (e EulaStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if e.cursor > 0 {
				e.cursor--
			}

		case "down", "j":
			if e.cursor < 2 {
				e.cursor++
			}

		case "enter", " ":
			switch e.cursor {
			case 0:
				e.config.Eula = !e.config.Eula
				e.warn = ""
			case 1:
				e.config.AutoRun = !e.config.AutoRun
			case 2:
				if e.config.Eula {
					return e, func() tea.Msg { return NextStepMsg{} }
				} else {
					e.warn = "⚠️ You must accept the Mojang EULA to proceed."
				}
			}
		}
	}
	return e, nil
}

func (e EulaStep) View() tea.View {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Step 5: Agreements & Configurations"))
	b.WriteString("\n\n")

	eulaCheck := "[ ]"
	if e.config.Eula {
		eulaCheck = "[X]"
	}
	if e.cursor == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(fmt.Sprintf("  ➔ %s Accept Minecraft EULA License Agreement", eulaCheck)))
		b.WriteString("\n")
	} else {
		b.WriteString(fmt.Sprintf("    %s Accept Minecraft EULA License Agreement\n", eulaCheck))
	}
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Inactive).Render("      (https://account.mojang.com/documents/minecraft_eula)"))
	b.WriteString("\n\n")

	startCheck := "[ ]"
	if e.config.AutoRun {
		startCheck = "[X]"
	}
	if e.cursor == 1 {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(fmt.Sprintf("  ➔ %s Boot server instantly on setup", startCheck)))
		b.WriteString("\n\n")
	} else {
		b.WriteString(fmt.Sprintf("    %s Boot server instantly on setup\n\n", startCheck))
	}

	var btn string
	if e.cursor == 2 {
		btn = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("  ➔ [ Continue ]")
	} else {
		btn = "    [ Continue ]"
	}
	b.WriteString(btn)
	b.WriteString("\n")

	if e.warn != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("  " + e.warn))
		b.WriteString("\n")
	}

	return tea.NewView(b.String())
}

// ==========================================
// subcomponent ConfirmStep (Step 5)
// ==========================================

type ConfirmStep struct {
	config *ServerConfig
	cursor int
}

func (c ConfirmStep) Init() tea.Cmd { return nil }

func (c ConfirmStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k", "down", "j":
			c.cursor = (c.cursor + 1) % 2

		case "enter", " ":
			if c.cursor == 0 {
				return c, func() tea.Msg { return NextStepMsg{} }
			}
			return c, func() tea.Msg { return PrevStepMsg{} }
		}
	}
	return c, nil
}

func (c ConfirmStep) View() tea.View {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Step 5: Review Server Details"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  Engine:   %s\n", lipgloss.NewStyle().Foreground(theme.Accent).Render(c.config.Software)))
	b.WriteString(fmt.Sprintf("  Version:  %s\n", lipgloss.NewStyle().Foreground(theme.Accent).Render(c.config.Version)))
	b.WriteString(fmt.Sprintf("  Memory:   %s\n", lipgloss.NewStyle().Foreground(theme.Accent).Render(c.config.RAM)))

	startStatus := "No"
	if c.config.AutoRun {
		startStatus = "Yes"
	}
	b.WriteString(fmt.Sprintf("  Boot:     %s\n\n", lipgloss.NewStyle().Foreground(theme.Accent).Render(startStatus)))

	b.WriteString("Confirm creation of the Minecraft server setup?\n\n")

	var confirmBtn, cancelBtn string
	if c.cursor == 0 {
		confirmBtn = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("  ➔ [ Build Server ]")
		cancelBtn = "    [ Back to Options ]"
	} else {
		confirmBtn = "    [ Build Server ]"
		cancelBtn = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("  ➔ [ Back to Options ]")
	}

	b.WriteString(confirmBtn)
	b.WriteString("\n")
	b.WriteString(cancelBtn)
	return tea.NewView(b.String())
}

// ==========================================
// subcomponent SuccessStep (Step 6)
// ==========================================

type SuccessStep struct {
	config *ServerConfig
}

func (s SuccessStep) Init() tea.Cmd { return nil }

func (s SuccessStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "enter" {
			return s, core.RouteTo("ManageServers")
		}
	}
	return s, nil
}

func (s SuccessStep) View() tea.View {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render("✓ Server Created Successfully!"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Your %s %s instance is ready.\n", s.config.Software, s.config.Version))
	b.WriteString(fmt.Sprintf("Allocated Resource Cap: %s RAM\n\n", s.config.RAM))

	if s.config.AutoRun {
		b.WriteString("Spinning up instances now in the background...\n\n")
	}

	b.WriteString("Press ")
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("[Enter]"))
	b.WriteString(" to manage servers.")
	return tea.NewView(b.String())
}

// // create_server.go
// package pages

// import (
// 	"fmt"
// 	"strings"

// 	tea "charm.land/bubbletea/v2"
// 	"charm.land/lipgloss/v2"
// 	"github.com/limelamp/osmium/internal/tui/core"
// 	"github.com/limelamp/osmium/internal/tui/styles"
// )

// // Define step transition signals
// type NextStepMsg struct{}
// type PrevStepMsg struct{}

// // ServerConfig holds the unified configuration across steps
// type ServerConfig struct {
// 	Software string
// 	Version  string
// 	RAM      string
// 	Eula     bool
// 	AutoRun  bool
// }

// type CreateServerModel struct {
// 	layout core.Layout
// 	config *ServerConfig
// 	steps  []tea.Model
// 	active int
// }

// func NewCreateServerModel() CreateServerModel {
// 	cfg := &ServerConfig{
// 		Software: "Vanilla",
// 		Version:  "1.21",
// 		RAM:      "4 GB",
// 	}

// 	m := CreateServerModel{
// 		config: cfg,
// 	}
// 	m.initSteps()
// 	return m
// }

// func (m *CreateServerModel) initSteps() {
// 	m.steps = []tea.Model{
// 		// Step 0: Software Selection
// 		NewSelectStep("Step 1: Choose Server Engine", softwares, func(v string) {
// 			m.config.Software = v
// 		}),
// 		// Step 1: Version Selection (Placeholder, populated dynamically on transition)
// 		NewSelectStep("Step 2: Select Version", []string{}, func(v string) {
// 			m.config.Version = v
// 		}),
// 		// Step 2: Specs/RAM Selection
// 		NewSelectStep("Step 3: Allocate System Memory", ramOptions, func(v string) {
// 			m.config.RAM = v
// 		}),
// 		// Step 3: Rules/Agreements
// 		EulaStep{config: m.config},
// 		// Step 4: Confirmation Screen
// 		ConfirmStep{config: m.config},
// 		// Step 5: Success Screen
// 		SuccessStep{config: m.config},
// 	}
// }

// func (m CreateServerModel) Init() tea.Cmd {
// 	return nil
// }

// func (m CreateServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
// 	switch msg := msg.(type) {
// 	case tea.KeyPressMsg:
// 		if msg.String() == "backspace" {
// 			// If we're past the first step, go back. Otherwise, go to Home.
// 			if m.active > 0 && m.active < len(m.steps)-1 {
// 				m.active--
// 				return m, nil
// 			}
// 			return m, core.RouteTo("Home")
// 		}

// 	case NextStepMsg:
// 		if m.active < len(m.steps)-1 {
// 			m.active++
// 			// Dynamically filter Version list based on Software choice before opening Step 1
// 			if m.active == 1 {
// 				m.steps[1] = NewSelectStep(
// 					fmt.Sprintf("Step 2: Select Version for %s", m.config.Software),
// 					getVersions(m.config.Software),
// 					func(v string) { m.config.Version = v },
// 				)
// 			}
// 		}
// 		return m, nil

// 	case PrevStepMsg:
// 		if m.active > 0 {
// 			m.active--
// 		}
// 		return m, nil
// 	}

// 	// Forward message processing to the active step component
// 	var cmd tea.Cmd
// 	m.steps[m.active], cmd = m.steps[m.active].Update(msg)
// 	return m, cmd
// }

// func (m CreateServerModel) View() tea.View {
// 	if m.layout.Width == 0 {
// 		return tea.NewView("loading...")
// 	}

// 	progressBar := m.renderProgressBar()
// 	body := m.steps[m.active].View().Content

// 	helpText := "\n\n" + lipgloss.NewStyle().Foreground(theme.Inactive).Render("  [↑/↓] Navigate  •  [Enter] Select  •  [Esc] Back")

// 	view := lipgloss.JoinVertical(
// 		lipgloss.Left,
// 		progressBar,
// 		"",
// 		body,
// 		helpText,
// 	)

// 	return tea.NewView(
// 		styles.Container(
// 			m.layout.Width,
// 			m.layout.Height,
// 			true,
// 			"Setup New Server",
// 			view,
// 			true,
// 		),
// 	)
// }

// func (m CreateServerModel) renderProgressBar() string {
// 	steps := []string{"Software", "Version", "Specs", "Rules", "Confirm"}
// 	var renderedSteps []string

// 	for i, name := range steps {
// 		stepIdx := i
// 		var s string
// 		if stepIdx == m.active {
// 			s = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(fmt.Sprintf("[%d. %s]", i+1, name))
// 		} else if stepIdx < m.active {
// 			s = lipgloss.NewStyle().Foreground(theme.Accent).Render(fmt.Sprintf("✓ %s", name))
// 		} else {
// 			s = lipgloss.NewStyle().Foreground(theme.Inactive).Render(fmt.Sprintf("%d. %s", i+1, name))
// 		}
// 		renderedSteps = append(renderedSteps, s)
// 	}

// 	return "  " + lipgloss.JoinHorizontal(0, strings.Join(renderedSteps, "  ➔  "))
// }

// func (m CreateServerModel) Title() string {
// 	return "Create Server"
// }

// func (m CreateServerModel) SetLayout(l core.Layout) tea.Model {
// 	m.layout = l
// 	return m
// }

// // ==========================================
// // REUSABLE SUB-COMPONENT: SelectStep (Steps 1, 2, 3)
// // ==========================================

// type SelectStep struct {
// 	title    string
// 	options  []string
// 	cursor   int
// 	onSelect func(string)
// }

// func NewSelectStep(title string, options []string, onSelect func(string)) SelectStep {
// 	return SelectStep{
// 		title:    title,
// 		options:  options,
// 		onSelect: onSelect,
// 	}
// }

// func (s SelectStep) Init() tea.Cmd { return nil }

// func (s SelectStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
// 	switch msg := msg.(type) {
// 	case tea.KeyPressMsg:
// 		switch msg.String() {
//
// case "up", "k":
// 			if s.cursor > 0 {
// 				s.cursor--
// 			}
//
// case "down", "j":
// 			if s.cursor < len(s.options)-1 {
// 				s.cursor++
// 			}
//
// case "enter", " ":
// 			if len(s.options) > 0 {
// 				s.onSelect(s.options[s.cursor])
// 				return s, func() tea.Msg { return NextStepMsg{} }
// 			}
// 		}
// 	}
// 	return s, nil
// }

// func (s SelectStep) View() tea.View {
// 	var b strings.Builder
// 	b.WriteString(lipgloss.NewStyle().Bold(true).Render(s.title) + "\n\n")

// 	for i, opt := range s.options {
// 		if i == s.cursor {
// 			b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(fmt.Sprintf("  ➔ %s", opt)) + "\n\n")
// 		} else {
// 			b.WriteString(fmt.Sprintf("    %s\n\n", opt))
// 		}
// 	}
// 	return tea.NewView(b.String())
// }

// // ==========================================
// // SPECIFIC SUB-COMPONENT: EulaStep (Step 4)
// // ==========================================

// type EulaStep struct {
// 	config *ServerConfig
// 	cursor int
// 	warn   string
// }

// func (e EulaStep) Init() tea.Cmd { return nil }

// func (e EulaStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
// 	switch msg := msg.(type) {
// 	case tea.KeyPressMsg:
// 		switch msg.String() {
//
// case "up", "k":
// 			if e.cursor > 0 {
// 				e.cursor--
// 			}
//
// case "down", "j":
// 			if e.cursor < 2 {
// 				e.cursor++
// 			}
//
// case "enter", " ":
// 			switch e.cursor {
// 			case 0:
// 				e.config.Eula = !e.config.Eula
// 				e.warn = ""
// 			case 1:
// 				e.config.AutoRun = !e.config.AutoRun
// 			case 2:
// 				if e.config.Eula {
// 					return e, func() tea.Msg { return NextStepMsg{} }
// 				} else {
// 					e.warn = "⚠️ You must accept the Mojang EULA to proceed."
// 				}
// 			}
// 		}
// 	}
// 	return e, nil
// }

// func (e EulaStep) View() tea.View {
// 	var b strings.Builder
// 	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Step 4: Agreements & Configurations") + "\n\n")

// 	eulaCheck := "[ ]"
// 	if e.config.Eula {
// 		eulaCheck = "[X]"
// 	}
// 	if e.cursor == 0 {
// 		b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(fmt.Sprintf("  ➔ %s Accept Minecraft EULA License Agreement", eulaCheck)) + "\n")
// 	} else {
// 		b.WriteString(fmt.Sprintf("    %s Accept Minecraft EULA License Agreement\n", eulaCheck))
// 	}
// 	b.WriteString(lipgloss.NewStyle().Foreground(theme.Inactive).Render("      (https://account.mojang.com/documents/minecraft_eula)") + "\n\n")

// 	startCheck := "[ ]"
// 	if e.config.AutoRun {
// 		startCheck = "[X]"
// 	}
// 	if e.cursor == 1 {
// 		b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(fmt.Sprintf("  ➔ %s Boot server instantly on setup", startCheck)) + "\n\n")
// 	} else {
// 		b.WriteString(fmt.Sprintf("    %s Boot server instantly on setup\n\n", startCheck))
// 	}

// 	var btn string
// 	if e.cursor == 2 {
// 		btn = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("  ➔ [ Continue ]")
// 	} else {
// 		btn = "    [ Continue ]"
// 	}
// 	b.WriteString(btn + "\n")

// 	if e.warn != "" {
// 		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("  "+e.warn) + "\n")
// 	}

// 	return tea.NewView(b.String())
// }

// // ==========================================
// // SPECIFIC SUB-COMPONENT: ConfirmStep (Step 5)
// // ==========================================

// type ConfirmStep struct {
// 	config *ServerConfig
// 	cursor int
// }

// func (c ConfirmStep) Init() tea.Cmd { return nil }

// func (c ConfirmStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
// 	switch msg := msg.(type) {
// 	case tea.KeyPressMsg:
// 		switch msg.String() {
//
// case "up", "k", "down", "j":
// 			c.cursor = (c.cursor + 1) % 2
//
// case "enter", " ":
// 			if c.cursor == 0 {
// 				return c, func() tea.Msg { return NextStepMsg{} }
// 			}
// 			return c, func() tea.Msg { return PrevStepMsg{} }
// 		}
// 	}
// 	return c, nil
// }

// func (c ConfirmStep) View() tea.View {
// 	var b strings.Builder
// 	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Step 5: Review Server Details") + "\n\n")

// 	b.WriteString(fmt.Sprintf("  Engine:   %s\n", lipgloss.NewStyle().Foreground(theme.Accent).Render(c.config.Software)))
// 	b.WriteString(fmt.Sprintf("  Version:  %s\n", lipgloss.NewStyle().Foreground(theme.Accent).Render(c.config.Version)))
// 	b.WriteString(fmt.Sprintf("  Memory:   %s\n", lipgloss.NewStyle().Foreground(theme.Accent).Render(c.config.RAM)))

// 	startStatus := "No"
// 	if c.config.AutoRun {
// 		startStatus = "Yes"
// 	}
// 	b.WriteString(fmt.Sprintf("  Boot:     %s\n\n", lipgloss.NewStyle().Foreground(theme.Accent).Render(startStatus)))

// 	b.WriteString("Confirm creation of the Minecraft server setup?\n\n")

// 	var confirmBtn, cancelBtn string
// 	if c.cursor == 0 {
// 		confirmBtn = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("  ➔ [ Build Server ]")
// 		cancelBtn = "    [ Back to Options ]"
// 	} else {
// 		confirmBtn = "    [ Build Server ]"
// 		cancelBtn = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("  ➔ [ Back to Options ]")
// 	}

// 	b.WriteString(confirmBtn + "\n" + cancelBtn)
// 	return tea.NewView(b.String())
// }

// // ==========================================
// // SPECIFIC SUB-COMPONENT: SuccessStep (Step 6)
// // ==========================================

// type SuccessStep struct {
// 	config *ServerConfig
// }

// func (s SuccessStep) Init() tea.Cmd { return nil }

// func (s SuccessStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
// 	switch msg := msg.(type) {
// 	case tea.KeyPressMsg:
// 		if msg.String() == "enter" {
// 			return s, core.RouteTo("ManageServers")
// 		}
// 	}
// 	return s, nil
// }

// func (s SuccessStep) View() tea.View {
// 	var b strings.Builder
// 	b.WriteString(lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render("✓ Server Created Successfully!") + "\n\n")
// 	b.WriteString(fmt.Sprintf("Your %s %s instance is ready.\n", s.config.Software, s.config.Version))
// 	b.WriteString(fmt.Sprintf("Allocated Resource Cap: %s RAM\n\n", s.config.RAM))

// 	if s.config.AutoRun {
// 		b.WriteString("Spinning up instances now in the background...\n\n")
// 	}

// 	b.WriteString("Press " + lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render("[Enter]") + " to manage servers.")
// 	return tea.NewView(b.String())
// }
