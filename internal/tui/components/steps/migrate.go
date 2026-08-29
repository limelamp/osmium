package steps

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/supervisor"
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

	// The footer while it runs, exactly as the build step shows it.
	spinner  core.Ticker
	frame    int
	label    string
	received int64
	size     int64

	started bool
	done    bool
	err     error
}

// NewMigrateStep returns the step that carries out a migration.
func NewMigrateStep(cfg *Config) *MigrateStep {
	return &MigrateStep{
		config:   cfg,
		viewport: viewport.New(),
		spinner:  core.NewTicker("migrate.spinner", spinnerInterval),
	}
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

	// Replacing the jar under a running server would leave it running one
	// version and configured for another.
	if supervisor.Running(s.config.Dir) {
		s.started, s.done = true, true
		s.err = fmt.Errorf("%s is running — stop it before migrating it", s.config.Name)
		return nil
	}

	s.started = true
	s.events = make(chan buildEvent, 64)

	go s.migrate()

	ticker, tick := s.spinner.Restart()
	s.spinner = ticker

	return tea.Batch(waitForBuildEvent(s.events), tick)
}

// migrate runs the migration and reports it through the channel.
func (s *MigrateStep) migrate() {
	sc := scope.Scope{
		Dir: s.config.Dir,
		Log: func(line string) { s.events <- buildEvent{line: line} },
		Progress: func(label string, done, total int64) {
			s.events <- buildEvent{progress: true, label: label, received: done, size: total}
		},
	}

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
	case core.TickMsg:
		ticker, ours := s.spinner.Accepts(msg)
		if !ours || s.done {
			return s, nil
		}

		s.frame++
		ticker, tick := ticker.Arm()
		s.spinner = ticker
		return s, tick

	case buildEventMsg:
		if msg.progress {
			s.label, s.received, s.size = msg.label, msg.received, msg.size
			return s, waitForBuildEvent(s.events)
		}

		if msg.done {
			s.done = true
			s.spinner = s.spinner.Stop()
			s.label = ""
			s.err = msg.err
			if msg.err != nil {
				s.appendLog("error: " + msg.err.Error())
				return s, nil
			}
			s.appendLog("✓ Done.")
			return s, nil
		}
		s.appendLog(msg.line)
		return s, waitForBuildEvent(s.events)

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
func (s *MigrateStep) appendLog(line string) {
	s.logs = append(s.logs, strings.Split(line, "\n")...)
	s.refresh()
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

	if s.done {
		return tea.NewView(s.viewport.View())
	}

	label := s.label
	if label == "" {
		label = "Working…"
	}

	busy := styles.Spinner(s.frame) + " " +
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(label)
	if s.size > 0 || s.received > 0 {
		busy += "\n" + styles.ProgressBar(s.layout.Width, s.received, s.size)
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, s.viewport.View(), "", busy))
}

func (s *MigrateStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.viewport.SetWidth(max(l.Width, 1))
	s.viewport.SetHeight(max(l.Height-4, 1))
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
