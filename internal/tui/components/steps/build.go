package steps

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
	"github.com/limelamp/osmium/internal/util"
)

// buildEvent is one message from the background build: either a progress line
// or the final outcome. A single channel keeps the ordering obvious - the done
// event is always the last thing the build sends.
type buildEvent struct {
	line string
	done bool
	err  error

	// A progress event reports a download rather than saying anything: the
	// label of the file, how much has arrived and how much there is.
	progress bool
	label    string
	received int64
	size     int64
}

// buildEventMsg carries a build event into the update loop.
type buildEventMsg buildEvent

// BuildStep downloads and assembles the server, streaming what it is doing into
// a log. The work happens in a goroutine so the interface stays responsive
// through what can be a minutes-long download.
type BuildStep struct {
	config *Config
	store  *storage.ServerStore
	layout core.Layout

	events   chan buildEvent
	logs     []string
	viewport viewport.Model

	// What the footer shows while the build runs: a spinner on its own frame
	// counter, and the download it is waiting on.
	spinner  core.Ticker
	frame    int
	label    string
	received int64
	size     int64

	done   bool
	err    error
	server *storage.Server
}

// NewBuildStep returns the step that creates the server.
func NewBuildStep(cfg *Config, store *storage.ServerStore) *BuildStep {
	return &BuildStep{
		config:   cfg,
		store:    store,
		viewport: viewport.New(),
		spinner:  core.NewTicker("build.spinner", spinnerInterval),
	}
}

// spinnerInterval is how often a busy indicator advances a frame.
const spinnerInterval = 100 * time.Millisecond

// Server is the record written to servers.json, once the build succeeded.
func (s *BuildStep) Server() *storage.Server { return s.server }

// Done reports whether the build has finished, successfully or not.
func (s *BuildStep) Done() bool { return s.done }

// Failed reports whether the build ended in an error.
func (s *BuildStep) Failed() bool { return s.err != nil }

func (s *BuildStep) Init() tea.Cmd {
	s.logs = nil
	s.done = false
	s.err = nil
	s.server = nil
	s.events = make(chan buildEvent, 64)

	go s.build()

	ticker, tick := s.spinner.Restart()
	s.spinner = ticker

	return tea.Batch(waitForBuildEvent(s.events), tick)
}

// waitForBuildEvent blocks on the next build event and re-arms itself for the
// one after, which is how the log streams in rather than arriving in one lump.
func waitForBuildEvent(events chan buildEvent) tea.Cmd {
	return func() tea.Msg {
		return buildEventMsg(<-events)
	}
}

// build runs the creation sequence and reports it through the channel.
func (s *BuildStep) build() {
	server, err := Build(s.config, s.store, scope.Scope{
		Log: func(line string) { s.events <- buildEvent{line: line} },
		Progress: func(label string, done, total int64) {
			s.events <- buildEvent{progress: true, label: label, received: done, size: total}
		},
	})
	if err != nil {
		s.events <- buildEvent{done: true, err: err}
		return
	}

	s.server = server
	s.events <- buildEvent{done: true}
}

// Build creates the server a Config describes: it downloads the jar, writes the
// manifest and the EULA, and registers the result. Progress is reported line by
// line through log.
//
// The wizard runs this in a goroutine and streams the lines into a panel; the
// CLI runs it in the foreground and prints them. Both get the same server built
// the same way, which is the point of it living here rather than in either.
func Build(cfg *Config, store *storage.ServerStore, sc scope.Scope) (*storage.Server, error) {
	emit := sc.Logf

	target := TargetDir(cfg)
	server := &storage.Server{
		ID:      slug(cfg.Name),
		Name:    cfg.Name,
		Path:    target,
		Version: cfg.Version,
		Type:    strings.ToLower(cfg.Software),
		Memory:  cfg.RAM,
	}

	emit("Creating %s", target)
	if err := os.MkdirAll(target, 0o750); err != nil {
		return nil, err
	}

	sc.Dir = target

	// 1. Download the server jar
	if err := util.DownloadJarInto(sc, cfg.Software, cfg.Version); err != nil {
		return nil, err
	}

	// 2. Write Osmium config file
	emit("Writing osmium.json")
	osmiumConf := config.OsmiumConfig{
		Category:     cfg.Category,
		Loader:       cfg.Software,
		Version:      cfg.Version,
		AllocatedRAM: cfg.RAM,
		Mods:         config.Wants{},
		Plugins:      config.Wants{},
	}
	if err := config.WriteConfigIn(target, &osmiumConf); err != nil {
		return nil, err
	}

	// 3. EULA Agreement creation
	if cfg.Eula {
		emit("Accepting the EULA")
		content := "#By changing the setting below to TRUE you are indicating your agreement to our EULA (https://aka.ms/MinecraftEULA).\n"
		content += "eula=true\n"

		if err := os.WriteFile(filepath.Join(target, "eula.txt"), []byte(content), 0o600); err != nil {
			return nil, err
		}
	}

	// 4. Register the server so it shows up in Manage Servers
	emit("Registering the server")
	if err := storage.Register(store, server); err != nil {
		return nil, err
	}

	return server, nil
}

func (s *BuildStep) Title() string {
	switch {
	case s.err != nil:
		return "Build Failed"
	case s.done:
		return "Server Created"
	default:
		return "Building"
	}
}

func (s *BuildStep) Subtitle() string {
	switch {
	case s.err != nil:
		return "Nothing was registered. Press backspace to change the options and retry."
	case s.done:
		return "Press enter to open Manage Servers."
	default:
		return "Downloading and assembling the server files…"
	}
}

func (s *BuildStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

// appendLog adds a line to the build log and keeps the view pinned to the tail.
func (s *BuildStep) appendLog(line string) {
	s.logs = append(s.logs, line)
	s.refresh()
	s.viewport.GotoBottom()
}

// refresh re-wraps the log to the current width.
func (s *BuildStep) refresh() {
	s.viewport.SetContent(lipgloss.NewStyle().
		Width(max(s.viewport.Width(), 1)).
		Render(strings.Join(s.logs, "\n")))
}

func (s *BuildStep) View() tea.View {
	body := s.viewport.View()

	switch {
	case s.err != nil:
		body += "\n\n" + styles.Error(s.err)
	case s.done:
		body += "\n\n" + lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).
			Render("✓ "+s.config.Name+" is ready.")
	default:
		body += "\n\n" + s.busyLine()
	}

	return tea.NewView(body)
}

// busyLine is what the build shows while it is working: the spinner, what it is
// doing, and how far a download has got.
func (s *BuildStep) busyLine() string {
	label := s.label
	if label == "" {
		label = "Working…"
	}

	line := styles.Spinner(s.frame) + " " +
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(label)

	if s.size > 0 || s.received > 0 {
		line += "\n" + styles.ProgressBar(s.layout.Width, s.received, s.size)
	}
	return line
}

func (s *BuildStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.viewport.SetWidth(max(l.Width, 1))
	// Three rows below the log: the blank, the busy line and its progress bar.
	s.viewport.SetHeight(max(l.Height-4, 1))
	s.refresh()
	return s
}

// Scroll reports the build log's position.
func (s *BuildStep) Scroll() core.Scroll {
	return core.Scroll{
		Offset:  s.viewport.YOffset(),
		Total:   lipgloss.Height(s.viewport.GetContent()),
		Visible: s.viewport.Height(),
	}
}

// Editing reports that the build log takes no typed input.
func (s *BuildStep) Editing() bool { return false }
