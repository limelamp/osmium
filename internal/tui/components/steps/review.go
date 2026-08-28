package steps

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// Buttons of the review step, in display order.
const (
	reviewBuild = iota
	reviewBack
	reviewCount
)

// ReviewStep shows everything the wizard collected and starts the build.
type ReviewStep struct {
	config *Config
	layout core.Layout

	list core.List
}

func NewReviewStep(config *Config) *ReviewStep {
	s := &ReviewStep{config: config}
	s.list = s.list.Resize(reviewCount, reviewCount)
	return s
}

func (s *ReviewStep) Init() tea.Cmd { return nil }

func (s *ReviewStep) Title() string { return "Review" }

func (s *ReviewStep) Subtitle() string {
	return "Check the summary, then build the server."
}

func (s *ReviewStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		// The buttons sit under the summary block, so clicks are offset by it.
		mouse := msg.Mouse()
		if _, isClick := msg.(tea.MouseClickMsg); !isClick || !s.layout.Hit(mouse.X, mouse.Y) {
			return s, nil
		}

		_, buttonRow := s.rows()
		index := mouse.Y - s.layout.Y - buttonRow
		if index < 0 || index >= reviewCount {
			return s, nil
		}
		if index == s.list.Cursor {
			return s.activate()
		}
		s.list = s.list.MoveTo(index)
		return s, nil

	case tea.KeyPressMsg:
		if list, handled := s.list.HandleKey(msg.String()); handled {
			s.list = list
			return s, nil
		}

		if msg.String() == "enter" || msg.String() == " " {
			return s.activate()
		}
	}

	return s, nil
}

// activate runs the highlighted button.
func (s *ReviewStep) activate() (tea.Model, tea.Cmd) {
	if s.list.Cursor == reviewBuild {
		return s, Next()
	}
	return s, Prev()
}

// rows renders the body and reports the display row the first button sits on.
// The summary is trimmed from the bottom when the panel is too short, because
// the buttons have to stay reachable even on a small terminal.
func (s *ReviewStep) rows() ([]string, int) {
	summary := []string{
		s.summaryRow("Name", s.config.Name),
		s.summaryRow("Folder", TargetDir(s.config)),
		s.summaryRow("Engine", fmt.Sprintf("%s (%s)", s.config.Software, s.config.Category)),
		s.summaryRow("Version", s.config.Version),
		s.summaryRow("Memory", s.config.RAM),
		s.summaryRow("Boot after build", yesNo(s.config.AutoRun)),
	}

	// Two buttons plus the prompt above them are what must always fit.
	if room := s.layout.Height - reviewCount - 1; room < len(summary) {
		summary = summary[:max(room, 0)]
	}

	rows := append(summary,
		lipgloss.NewStyle().Foreground(theme.Inactive).Render("Create this server?"),
		styles.ListRow(s.layout.Width, s.list.Cursor == reviewBuild, "[ Build server ]", ""),
		styles.ListRow(s.layout.Width, s.list.Cursor == reviewBack, "[ Back to options ]", ""),
	)

	return rows, len(summary) + 1
}

func (s *ReviewStep) View() tea.View {
	rows, _ := s.rows()
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// summaryRow renders one label/value line of the summary block.
func (s *ReviewStep) summaryRow(label, value string) string {
	return lipgloss.NewStyle().Foreground(theme.Inactive).Width(20).Render(label) +
		lipgloss.NewStyle().Foreground(theme.Accent).Render(value)
}

func (s *ReviewStep) SetLayout(l core.Layout) Step {
	s.layout = l
	return s
}

// TargetDir is the folder the server's files end up in: the chosen location
// with the server's own folder inside it.
func TargetDir(config *Config) string {
	if config.Dir == "" {
		return ""
	}
	return filepath.Join(config.Dir, slug(config.Name))
}

func yesNo(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

// Scroll reports no scrolling: the summary is trimmed to fit instead.
func (s *ReviewStep) Scroll() core.Scroll { return core.Scroll{} }
