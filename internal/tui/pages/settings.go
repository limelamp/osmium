package pages

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/config"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

type SettingsModel struct {
	layout core.Layout
	cursor int
}

func NewSettingsModel() SettingsModel {
	// Match cursor position to currently active global theme
	cursor := 0
	for i, name := range theme.ThemeNames {
		if name == theme.CurrentTheme {
			cursor = i
			break
		}
	}
	return SettingsModel{
		cursor: cursor,
	}
}

func (m SettingsModel) Init() tea.Cmd {
	return nil
}

func (m SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(theme.ThemeNames) - 1
			}
		case "down", "j":
			if m.cursor < len(theme.ThemeNames)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case "enter":
			// Update running styles
			selectedTheme := theme.ThemeNames[m.cursor]
			theme.SetTheme(selectedTheme)

			// Persist theme choice to file
			cfg, err := config.LoadConfig()
			if err == nil {
				cfg.Theme = selectedTheme
				_ = config.SaveConfig(cfg)
			}
		}
	}
	return m, nil
}

func (m SettingsModel) View() tea.View {
	var b strings.Builder
	b.WriteString("Select UI Color Scheme:\n\n")

	for i, name := range theme.ThemeNames {
		cursorStr := "  "
		if i == m.cursor {
			cursorStr = "> "
		}

		activeStr := ""
		if name == theme.CurrentTheme {
			activeStr = " [Active]"
		}

		// Rendering text representation
		b.WriteString(fmt.Sprintf("%s%s%s\n", cursorStr, name, activeStr))
	}

	b.WriteString("\nUse [Up/Down or j/k] to navigate, [Enter] to select.")

	return tea.NewView(
		styles.Container(
			m.layout.Width,
			m.layout.Height,
			true,
			"Settings",
			b.String(),
			true,
		),
	)
}

func (m SettingsModel) Title() string {
	return "Settings"
}

func (m SettingsModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l
	return m
}
