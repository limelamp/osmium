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

	// scroll is over the rendered rows. It follows the buttons, so they are
	// always reachable, and page-up/down moves it freely so the summary above
	// them can be read on a short terminal.
	scroll core.Scroll
	manual bool
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
		switch msg.String() {
		case "pgup":
			s.scroll, s.manual = s.scroll.By(-s.scroll.Visible), true
			return s, nil
		case "pgdown":
			s.scroll, s.manual = s.scroll.By(s.scroll.Visible), true
			return s, nil
		}

		if list, handled := s.list.HandleKey(msg.String()); handled {
			s.list, s.manual = list, false
			return s, nil
		}

		if msg.String() == "enter" || msg.String() == "space" {
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
func (s *ReviewStep) rows() ([]string, int) {
	summary := []string{
		s.summaryRow("Name", s.config.Name),
		s.summaryRow("Folder", TargetDir(s.config)),
		s.summaryRow("Engine", fmt.Sprintf("%s (%s)", s.config.Software, s.config.Category)),
		s.summaryRow("Version", s.config.Version),
		s.summaryRow("Memory", s.config.RAM),
		s.summaryRow("Boot after build", yesNo(s.config.AutoRun)),
	}

	rows := append(summary,
		lipgloss.NewStyle().Foreground(theme.Inactive).Render("Create this server?"),
		styles.ListRow(s.layout.Width, s.list.Cursor == reviewBuild, "[ Build server ]", ""),
		styles.ListRow(s.layout.Width, s.list.Cursor == reviewBack, "[ Back to options ]", ""),
	)

	return rows, len(summary) + 1
}

func (s *ReviewStep) View() tea.View {
	rows, buttonRow := s.rows()

	s.scroll = s.scroll.Resize(len(rows), s.layout.Height)
	if !s.manual {
		// The button under the cursor is what must stay on screen.
		s.scroll = s.scroll.Follow(buttonRow + s.list.Cursor)
	}

	start, end := s.scroll.Window()
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, rows[start:end]...))
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

// Scroll reports how much of the review is off screen.
func (s *ReviewStep) Scroll() core.Scroll { return s.scroll }

// Editing reports that the review step takes no typed input.
func (s *ReviewStep) Editing() bool { return false }
