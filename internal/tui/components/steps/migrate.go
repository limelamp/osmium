package steps

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// MigrateStep moves a server to another loader and version, streaming what it
// is doing into a log.
//
// It is the migration's equivalent of BuildStep and works the same way: the
// work happens in a goroutine and reports through a channel, because replacing
// every mod on a server takes as long as downloading them all.
type MigrateStep struct {
	config *Config
	layout core.Layout

	events   chan buildEvent
	logs     []string
	viewport viewport.Model

	started bool
	done    bool
	err     error
}

func NewMigrateStep(config *Config) *MigrateStep {
	return &MigrateStep{config: config, viewport: viewport.New()}
}

// Done reports whether the migration has finished, successfully or not.
func (s *MigrateStep) Done() bool { return s.done }

// Failed reports whether the migration ended in an error.
func (s *MigrateStep) Failed() bool { return s.err != nil }

func (s *MigrateStep) Init() tea.Cmd {
	// Coming back to a finished migration must not run it a second time: the
	// mods have already been replaced.
	if s.started {
		return nil
	}

	s.started = true
	s.events = make(chan buildEvent, 64)

	go s.migrate()
	return waitForBuildEvent(s.events)
}

// migrate runs the migration and reports it through the channel.
func (s *MigrateStep) migrate() {
	sc := scope.Scope{Dir: s.config.Dir, Log: func(line string) {
		s.events <- buildEvent{line: line}
	}}

	err := shared.MigrateServer(sc, s.config.Software, s.config.Version)
	s.events <- buildEvent{done: true, err: err}
}

func (s *MigrateStep) Title() string {
	switch {
	case s.err != nil:
		return "Migration Failed"
	case s.done:
		return "Migration Complete"
	default:
		return "Migrating"
	}
}

func (s *MigrateStep) Subtitle() string {
	switch {
	case s.err != nil:
		return "The server was left as it was found. Press backspace to change the options."
	case s.done:
		return "Anything with no compatible build was moved to migration_backup."
	default:
		return "Replacing the server jar and every mod and plugin that has a build for it…"
	}
}

func (s *MigrateStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case buildEventMsg:
		if msg.done {
			s.done = true
			s.err = msg.err
			if msg.err != nil {
				return s, s.appendLog("error: " + msg.err.Error())
			}
			return s, s.appendLog("✓ Done.")
		}
		return s, tea.Batch(s.appendLog(msg.line), waitForBuildEvent(s.events))

	case tea.MouseMsg:
		mouse := msg.Mouse()
		if wheel, ok := msg.(tea.MouseWheelMsg); ok && s.layout.Hit(mouse.X, mouse.Y) {
			switch wheel.Button {
			case tea.MouseWheelUp:
				s.viewport.ScrollUp(core.WheelStep)
			case tea.MouseWheelDown:
				s.viewport.ScrollDown(core.WheelStep)
			}
		}
		return s, nil
	}

	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// appendLog adds a line to the migration log and keeps the view on the tail.
func (s *MigrateStep) appendLog(line string) tea.Cmd {
	for _, one := range strings.Split(line, "\n") {
		s.logs = append(s.logs, one)
	}
	s.refresh()
	return nil
}

// refresh re-renders the log into the viewport.
func (s *MigrateStep) refresh() {
	s.viewport.SetContent(lipgloss.NewStyle().
		Width(max(s.viewport.Width(), 1)).
		Render(strings.Join(s.logs, "\n")))
	s.viewport.GotoBottom()
}

func (s *MigrateStep) View() tea.View {
	if s.err != nil {
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
			s.viewport.View(), "", styles.Error(s.err)))
	}
	return tea.NewView(s.viewport.View())
}

func (s *MigrateStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.viewport.SetWidth(max(l.Width, 1))
	s.viewport.SetHeight(max(l.Height-1, 1))
	s.refresh()
	return s
}

// Scroll reports the log's position.
func (s *MigrateStep) Scroll() core.Scroll {
	return core.Scroll{
		Offset:  s.viewport.YOffset(),
		Total:   len(s.logs),
		Visible: s.viewport.Height(),
	}
}

// Editing reports that the log takes no typed input.
func (s *MigrateStep) Editing() bool { return false }

// Summary is the one-line description of what the migration will do, shown
// before it starts.
func Summary(cfg *Config) string {
	conf, err := config.ReadConfigIn(cfg.Dir)
	if err != nil {
		return cfg.Name
	}

	from := lipgloss.NewStyle().Foreground(theme.Inactive).
		Render(conf.Loader + " " + conf.Version)
	to := lipgloss.NewStyle().Foreground(theme.Accent).
		Render(cfg.Software + " " + cfg.Version)

	return cfg.Name + ": " + from + " → " + to
}
