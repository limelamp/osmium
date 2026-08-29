package steps

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// ServerStep picks which registered server the wizard acts on. It is only shown
// when the page was opened without one — coming from the Actions pane, the
// server is already known and the step is left out.
type ServerStep struct {
	config *Config
	store  *storage.ServerStore
	layout core.Layout

	servers []storage.Server
	list    core.List
	err     error
}

// NewServerStep returns the step that picks which server to act on.
func NewServerStep(cfg *Config, store *storage.ServerStore) *ServerStep {
	return &ServerStep{config: cfg, store: store}
}

func (s *ServerStep) Init() tea.Cmd {
	return core.LoadServersCmd(s.store)
}

func (s *ServerStep) Title() string { return "Choose Server" }

func (s *ServerStep) Subtitle() string {
	return "Which server is being migrated."
}

func (s *ServerStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.LoadedServersMsg:
		s.err = msg.Err
		s.servers = msg.Servers
		s.list = s.list.Resize(len(s.servers), max(s.layout.Height, 1))
		return s, nil

	case tea.MouseMsg:
		list, activated := s.list.HandleMouse(msg, s.layout, 0)
		s.list = list
		if activated {
			cmd := s.choose()
			return s, cmd
		}
		return s, nil

	case tea.KeyPressMsg:
		if list, handled := s.list.HandleKey(msg.String()); handled {
			s.list = list
			return s, nil
		}

		switch msg.String() {
		case "enter", "space", "right", "l":
			cmd := s.choose()
			return s, cmd
		}
	}

	return s, nil
}

// choose records the highlighted server and moves on.
func (s *ServerStep) choose() tea.Cmd {
	if len(s.servers) == 0 {
		return nil
	}

	server := s.servers[s.list.Cursor]
	s.config.Dir = server.Path
	s.config.Name = server.Name
	return Next()
}

func (s *ServerStep) View() tea.View {
	if s.err != nil {
		return tea.NewView(styles.Error(s.err))
	}

	if len(s.servers) == 0 {
		return tea.NewView(lipgloss.NewStyle().Foreground(theme.Inactive).
			Render("No servers yet. Create one first."))
	}

	return tea.NewView(s.list.Render(func(i int) string {
		return styles.ListRow(s.layout.Width, i == s.list.Cursor,
			s.servers[i].Name, s.engineOf(s.servers[i]))
	}))
}

// engineOf describes what a server currently runs, which is what a migration
// changes.
func (s *ServerStep) engineOf(server storage.Server) string {
	conf, err := config.ReadConfigIn(server.Path)
	if err != nil {
		return server.Type + " " + server.Version
	}
	return conf.Loader + " " + conf.Version
}

func (s *ServerStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.list = s.list.Resize(len(s.servers), max(l.Height, 1))
	return s
}

// Scroll reports the server list's position.
func (s *ServerStep) Scroll() core.Scroll { return s.list.Scroll }

// Editing reports that the server list takes no typed input.
func (s *ServerStep) Editing() bool { return false }
