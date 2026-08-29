package steps

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/constants"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// EngineStep picks the server software, laid out as one column per category.
type EngineStep struct {
	config *Config
	layout core.Layout

	grid Grid
}

// NewEngineStep returns the step that picks the server software.
func NewEngineStep(cfg *Config) *EngineStep {
	return &EngineStep{
		config: cfg,
		grid: NewGrid([]GridColumn{
			{Title: "Vanilla", Items: []string{"Vanilla"}},
			{Title: "Plugin", Items: constants.PluginOptions},
			{Title: "Mod Loader", Items: constants.ModLoaderOptions},
		}),
	}
}

func (s *EngineStep) Init() tea.Cmd { return nil }

func (s *EngineStep) Title() string { return "Choose Server Engine" }

func (s *EngineStep) Subtitle() string {
	return "Pick the software your server runs on."
}

func (s *EngineStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	grid, activated := s.grid.Update(msg)
	s.grid = grid

	if !activated {
		return s, nil
	}

	category, software := s.grid.Selected()
	if software == "" {
		return s, nil
	}

	s.config.Category = category
	s.config.Software = software
	return s, Next()
}

func (s *EngineStep) View() tea.View {
	_, software := s.grid.Selected()

	description := lipgloss.NewStyle().
		Foreground(theme.Inactive).
		Render(constants.SoftwareDesc[software])

	return tea.NewView(lipgloss.JoinVertical(
		lipgloss.Left,
		s.grid.View(),
		"",
		description,
	))
}

func (s *EngineStep) SetLayout(l core.Layout) Step {
	s.layout = l
	// The description line sits under the grid, so the grid gets the rest.
	s.grid = s.grid.SetLayout(l.Inset(0, 0, 0, -2))
	return s
}

// Scroll reports the engine grid's position.
func (s *EngineStep) Scroll() core.Scroll { return s.grid.Scroll() }

// Editing reports that the engine grid takes no typed input.
func (s *EngineStep) Editing() bool { return false }
