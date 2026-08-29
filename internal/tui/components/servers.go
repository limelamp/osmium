package components

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// ServersModel lists the managed servers and owns which one is selected. Every
// other panel on the page follows that selection.
type ServersModel struct {
	layout core.Layout
	store  *storage.ServerStore

	servers []storage.Server

	// running mirrors servers: probing a lock file on every render would mean
	// spawning a process per server per frame on Windows, so the states are
	// refreshed on a timer instead.
	running []bool

	list    core.List
	status  core.Ticker
	isFocus bool
	err     error

	// pending is a server path to highlight once the list has been read. The
	// list loads asynchronously, so a selection made before that arrives would
	// be overwritten the moment it does.
	pending string
}

// statusInterval is how often the running indicators are re-checked.
const statusInterval = 2 * time.Second

// NewServersModel now properly initializes the store field
func NewServersModel(store *storage.ServerStore) ServersModel {
	return ServersModel{
		store:  store,
		status: core.NewTicker("servers.status", statusInterval),
	}
}

// Init reloads the list and asks to be woken, which is what restarts the status
// probe. The restart happens on the wake message rather than here, because Init
// cannot keep the ticker it would have armed.
func (m ServersModel) Init() tea.Cmd {
	return tea.Batch(core.LoadServersCmd(m.store), core.Wake())
}

// refreshStatus re-probes which servers currently hold a live process lock.
func (m ServersModel) refreshStatus() ServersModel {
	running := make([]bool, len(m.servers))
	for i, server := range m.servers {
		running[i] = isServerRunning(server.Path)
	}

	m.running = running
	return m
}

// rows is how many server rows fit, leaving room for the hint line.
func (m ServersModel) rows() int {
	return max(0, m.layout.ContentHeight()-2)
}

// Selected returns the server under the cursor, or nil when the list is empty.
func (m ServersModel) Selected() *storage.Server {
	if len(m.servers) == 0 {
		return nil
	}
	server := m.servers[m.list.Cursor]
	return &server
}

func (m ServersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		before := m.list.Cursor
		list, _ := m.list.HandleMouse(msg, m.layout.Inner(), 0)
		m.list = list
		if m.list.Cursor != before {
			return m, m.selectCmd()
		}
		return m, nil

	case tea.KeyPressMsg:
		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, m.selectCmd()
		}

		if msg.String() == "r" {
			return m, core.LoadServersCmd(m.store)
		}

	case core.WakeMsg:
		status, cmd := m.status.Restart()
		m.status = status
		return m, cmd

	case core.ReloadServersMsg:
		return m, core.LoadServersCmd(m.store)

	case core.TickMsg:
		status, ours := m.status.Accepts(msg)
		if !ours {
			return m, nil
		}

		armed, cmd := status.Arm()
		m.status = armed
		return m.refreshStatus(), cmd

	case core.LoadedServersMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.err = nil
		m.servers = msg.Servers
		m.list = m.list.Resize(len(m.servers), m.rows())
		m = m.applyPending()

		// In order: the selection first, so the notice about what was pruned
		// lands in the console the user is now looking at rather than in
		// whichever one happened to be showing beforehand.
		return m.refreshStatus(), tea.Sequence(
			core.SelectServer(m.Selected()),
			reportMissing(msg.Missing),
		)

	case core.SavedServersMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, nil
	}

	return m, nil
}

// Select asks for a server to be highlighted as soon as the list has been read,
// which is how the interface opens straight onto the server named on the
// command line.
func (m ServersModel) Select(path string) ServersModel {
	m.pending = path
	return m
}

// applyPending moves the cursor onto the server that was asked for, if it is
// in the list that just arrived.
func (m ServersModel) applyPending() ServersModel {
	if m.pending == "" {
		return m
	}

	for i, server := range m.servers {
		if server.Path == m.pending {
			m.list = m.list.MoveTo(i)
			break
		}
	}

	m.pending = ""
	return m
}

// selectCmd announces the highlighted server to the rest of the page. A server
// whose folder has been deleted since the list was read is dropped instead:
// reloading prunes it, which is the same thing that happens at startup.
func (m ServersModel) selectCmd() tea.Cmd {
	server := m.Selected()
	if server != nil && !server.Exists() {
		return core.LoadServersCmd(m.store)
	}
	return core.SelectServer(server)
}

// reportMissing tells the console which stale entries were dropped, so servers
// never disappear from the list without a word.
func reportMissing(missing []string) tea.Cmd {
	if len(missing) == 0 {
		return nil
	}

	lines := make([]string, 0, len(missing))
	for _, name := range missing {
		lines = append(lines, "removed "+name+" from the list — its folder is gone")
	}
	return core.Log(lines...)
}

func (m ServersModel) View() tea.View {
	var body string

	switch {
	case m.err != nil:
		body = styles.Error(m.err)

	case len(m.servers) == 0:
		body = lipgloss.NewStyle().Foreground(theme.Inactive).
			Render("No servers yet.\n\nCreate one from the home page to see it here.")

	default:
		body = m.list.Render(m.serverRow)
	}

	body = styles.Body(body, m.layout.ContentWidth(), m.layout.ContentHeight()-1)

	hints := styles.Hints(m.layout.ContentWidth(), m.Hints()...)

	// 3. Render the list inside the layout container style
	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:         m.layout.Width,
		Height:        m.layout.Height,
		Focused:       m.isFocus,
		Title:         m.Title(),
		Content:       body + "\n" + hints,
		ScrollOffset:  m.list.Scroll.Offset,
		ScrollTotal:   m.list.Scroll.Total,
		ScrollVisible: m.list.Scroll.Visible,
	}))
}

// TextPane is where the list's text sits and how far it has scrolled.
func (m ServersModel) TextPane() core.Pane {
	return core.Pane{Bounds: m.layout.Text(), Offset: m.list.Scroll.Offset}
}

// Hints are the keys this panel responds to.
func (m ServersModel) Hints() []styles.Hint {
	return []styles.Hint{
		{Key: "↑/↓", Desc: "select"},
		{Key: "r", Desc: "refresh"},
		{Key: "ctrl+b", Desc: "hide pane"},
		{Key: "tab", Desc: "switch pane"},
	}
}

// serverRow renders one server: its name with a running indicator, and what it
// runs on the right.
func (m ServersModel) serverRow(i int) string {
	running := i < len(m.running) && m.running[i]

	return styles.ListRow(m.layout.ContentWidth(), i == m.list.Cursor,
		ServerName(m.servers[i], running),
		ServerDetails(m.servers[i]),
	)
}

// ServerName is a server's name behind a lit or unlit dot. The home page shows
// servers too, and both should say the same thing the same way.
func ServerName(server storage.Server, running bool) string {
	dot := lipgloss.NewStyle().Foreground(theme.Inactive).Render("○")
	if running {
		dot = lipgloss.NewStyle().Foreground(theme.Accent).Render("●")
	}
	return dot + " " + server.Name
}

// ServerDetails is how long a server has been up, and nothing when it is not
// running. What a server is made of belongs on its details panel; a list of
// servers is for seeing which of them are up.
func ServerDetails(server storage.Server) string {
	return supervisor.Uptime(server.Path)
}

// isServerRunning reports whether a server directory has a live supervisor.
func isServerRunning(dir string) bool {
	return supervisor.Running(dir)
}

func (m ServersModel) Title() string {
	return "Servers"
}

func (m ServersModel) SetLayout(l core.Layout) ServersModel {
	m.layout = l
	m.list = m.list.Resize(len(m.servers), m.rows())
	return m
}

func (m ServersModel) SetFocus(focused bool) ServersModel {
	m.isFocus = focused
	return m
}
