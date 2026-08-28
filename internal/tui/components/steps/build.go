package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
// or the final outcome. A single channel keeps the ordering obvious — the done
// event is always the last thing the build sends.
type buildEvent struct {
	line string
	done bool
	err  error
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

	done   bool
	err    error
	server *storage.Server
}

func NewBuildStep(config *Config, store *storage.ServerStore) *BuildStep {
	return &BuildStep{
		config:   config,
		store:    store,
		viewport: viewport.New(),
	}
}

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

	target := TargetDir(s.config)
	server := &storage.Server{
		ID:      slug(s.config.Name),
		Name:    s.config.Name,
		Path:    target,
		Version: s.config.Version,
		Type:    strings.ToLower(s.config.Software),
		Memory:  s.config.RAM,
	}

	go s.build(target, server)

	return waitForBuildEvent(s.events)
}

// waitForBuildEvent blocks on the next build event and re-arms itself for the
// one after, which is how the log streams in rather than arriving in one lump.
func waitForBuildEvent(events chan buildEvent) tea.Cmd {
	return func() tea.Msg {
		return buildEventMsg(<-events)
	}
}

// build performs the whole creation sequence and reports through the channel.
func (s *BuildStep) build(target string, server *storage.Server) {
	emit := func(line string) { s.events <- buildEvent{line: line} }
	fail := func(err error) { s.events <- buildEvent{done: true, err: err} }

	emit("Creating " + target)
	if err := os.MkdirAll(target, 0755); err != nil {
		fail(err)
		return
	}

	sc := scope.Scope{Dir: target, Log: emit}

	// 1. Download the server jar
	if err := util.DownloadJarInto(sc, s.config.Software, s.config.Version); err != nil {
		fail(err)
		return
	}

	// 2. Write Osmium config file
	emit("Writing osmium.json")
	osmiumConf := config.OsmiumConfig{
		Category:     s.config.Category,
		Loader:       s.config.Software,
		Version:      s.config.Version,
		AllocatedRAM: s.config.RAM,
		Mods:         make(map[string]config.Project),
		Plugins:      make(map[string]config.Project),
	}
	if err := config.WriteConfigIn(target, &osmiumConf); err != nil {
		fail(err)
		return
	}

	// 3. EULA Agreement creation
	if s.config.Eula {
		emit("Accepting the EULA")
		content := "#By changing the setting below to TRUE you are indicating your agreement to our EULA (https://aka.ms/MinecraftEULA).\n"
		content += "eula=true\n"

		if err := os.WriteFile(filepath.Join(target, "eula.txt"), []byte(content), 0644); err != nil {
			fail(err)
			return
		}
	}

	// 4. Register the server so it shows up in Manage Servers
	emit("Registering the server")
	if err := s.register(server); err != nil {
		fail(err)
		return
	}

	s.server = server
	s.events <- buildEvent{done: true}
}

// register appends the new server to servers.json, keeping its id unique.
func (s *BuildStep) register(server *storage.Server) error {
	servers, err := s.store.LoadAll()
	if err != nil {
		return err
	}

	taken := make(map[string]bool, len(servers))
	for _, existing := range servers {
		taken[existing.ID] = true
	}

	base := server.ID
	for i := 2; taken[server.ID]; i++ {
		server.ID = fmt.Sprintf("%s-%d", base, i)
	}

	return s.store.SaveAll(append(servers, *server))
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

// appendLog adds a line to the build log and keeps the view pinned to the tail.
func (s *BuildStep) appendLog(line string) tea.Cmd {
	s.logs = append(s.logs, line)
	s.refresh()
	s.viewport.GotoBottom()
	return nil
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
	}

	return tea.NewView(body)
}

func (s *BuildStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.viewport.SetWidth(max(l.Width, 1))
	s.viewport.SetHeight(max(l.Height-3, 1))
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
