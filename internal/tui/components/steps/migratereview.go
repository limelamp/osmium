package steps

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// Buttons of the migrate review step, in display order.
const (
	migrateStart = iota
	migrateBack
	migrateCount
)

// MigrateReviewStep shows what the migration will do and starts it.
//
// A migration replaces the server jar and every mod on the server, so it gets
// the same last look the create wizard gives a build - and both wizards, in the
// interface and on the command line, show this same screen.
type MigrateReviewStep struct {
	config *Config
	layout core.Layout

	list core.List
}

// NewMigrateReviewStep returns the last look before a migration starts.
func NewMigrateReviewStep(cfg *Config) *MigrateReviewStep {
	s := &MigrateReviewStep{config: cfg}
	s.list = s.list.Resize(migrateCount, migrateCount)
	return s
}

func (s *MigrateReviewStep) Init() tea.Cmd { return nil }

func (s *MigrateReviewStep) Title() string { return "Review" }

func (s *MigrateReviewStep) Subtitle() string {
	return "Anything with no build for the target is moved to migration_backup."
}

func (s *MigrateReviewStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		// The buttons sit under the summary block, so clicks are offset by it.
		mouse := msg.Mouse()
		if _, isClick := msg.(tea.MouseClickMsg); !isClick || !s.layout.Hit(mouse.X, mouse.Y) {
			return s, nil
		}

		_, buttonRow := s.rows()
		index := mouse.Y - s.layout.Y - buttonRow
		if index < 0 || index >= migrateCount {
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

		if msg.String() == "enter" || msg.String() == "space" {
			return s.activate()
		}
	}

	return s, nil
}

// activate runs the highlighted button.
func (s *MigrateReviewStep) activate() (tea.Model, tea.Cmd) {
	if s.list.Cursor == migrateStart {
		return s, Next()
	}
	return s, Prev()
}

// rows renders the body and reports the display row the first button sits on.
func (s *MigrateReviewStep) rows() (rows []string, buttonRow int) {
	from := "unknown"
	if conf, err := config.ReadConfigIn(s.config.Dir); err == nil {
		from = conf.Loader + " " + conf.Version
	}

	summary := []string{
		s.summaryRow("Server", s.config.Name),
		s.summaryRow("Folder", s.config.Dir),
		s.summaryRow("Currently", from),
		s.summaryRow("Migrating to", s.config.Software+" "+s.config.Version),
	}

	// The buttons and the prompt above them are what must always fit.
	if room := s.layout.Height - migrateCount - 1; room < len(summary) {
		summary = summary[:max(room, 0)]
	}

	rows = make([]string, 0, len(summary)+3)
	rows = append(rows, summary...)
	rows = append(rows,
		lipgloss.NewStyle().Foreground(theme.Inactive).Render("Migrate this server?"),
		styles.ListRow(s.layout.Width, s.list.Cursor == migrateStart, "[ Migrate now ]", ""),
		styles.ListRow(s.layout.Width, s.list.Cursor == migrateBack, "[ Back to options ]", ""),
	)

	return rows, len(summary) + 1
}

func (s *MigrateReviewStep) View() tea.View {
	rows, _ := s.rows()
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// summaryRow renders one label/value line of the summary block.
func (s *MigrateReviewStep) summaryRow(label, value string) string {
	return lipgloss.NewStyle().Foreground(theme.Inactive).Width(20).Render(label) +
		lipgloss.NewStyle().Foreground(theme.Accent).Render(value)
}

func (s *MigrateReviewStep) SetLayout(l core.Layout) Step {
	s.layout = l
	return s
}

// Scroll reports no scrolling: the summary is trimmed to fit instead.
func (s *MigrateReviewStep) Scroll() core.Scroll { return core.Scroll{} }

// Editing reports that the review step takes no typed input.
func (s *MigrateReviewStep) Editing() bool { return false }
