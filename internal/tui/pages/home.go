package pages

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/assets"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/components"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// logoGap is the blank space between the logo and the menu.
const logoGap = 1

// homeStatusInterval is how often the home page re-checks which servers are up,
// which is also what keeps the uptimes moving while you sit on it.
const homeStatusInterval = 5 * time.Second

// HomeModel is the app's front page: the menu, and the servers it manages.
//
// The servers are here because they are the reason the app is open. Seeing
// which ones exist and which are running before choosing anything is worth more
// than an empty half-screen.
type HomeModel struct {
	layout core.Layout
	store  *storage.ServerStore

	nav components.NavigationModel

	servers []storage.Server
	running []bool
	status  core.Ticker
}

func NewHomeModel(store *storage.ServerStore) HomeModel {
	return HomeModel{
		store:  store,
		status: core.NewTicker("home.status", homeStatusInterval),
		nav: components.NewNavigationModel([]components.Choice{
			{Page: "Create server", Key: "c"},
			{Page: "Manage servers", Key: "m"},
			{Page: "Migrate server", Key: "g"},
			{Page: "Settings", Key: "s"},
			{Page: "Quit", Key: "q"},
		}),
	}
}

func (m HomeModel) Init() tea.Cmd {
	// Waking is a message rather than something done here: Init has a value
	// receiver, so a ticker restarted in it would be armed on a copy that is
	// thrown away, and its beat would never be recognised.
	return tea.Batch(core.LoadServersCmd(m.store), core.Wake())
}

func (m HomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Intercept the selection message from the navigation child component
	case components.MenuSelectionMsg:
		switch msg.Selection.Page {
		case "Quit":
			return m, tea.Quit

		case "Create server":
			// Ask the parent to switch to the CreateServer page
			return m, core.RouteTo("CreateServer")

		case "Manage servers":
			return m, core.RouteTo("ManageServers")

		case "Migrate server":
			return m, core.RouteTo("Migrate")

		case "Settings":
			return m, core.RouteTo("Settings")
		}

	case core.WakeMsg:
		status, cmd := m.status.Restart()
		m.status = status
		return m.refreshStatus(), cmd

	case core.LoadedServersMsg:
		if msg.Err == nil {
			m.servers = msg.Servers
		}

		// The servers box changes how tall the page is, and the page is
		// centred: everything below the logo has just moved.
		return m.refreshStatus().relayout(), nil

	case core.TickMsg:
		status, ours := m.status.Accepts(msg)
		if !ours {
			return m, nil
		}

		armed, cmd := status.Arm()
		m.status = armed
		return m.refreshStatus(), cmd

	case tea.KeyPressMsg:
		switch msg.String() {
		case "c":
			return m, core.RouteTo("CreateServer")

		case "m":
			return m, core.RouteTo("ManageServers")

		case "g":
			return m, core.RouteTo("Migrate")

		case "s":
			return m, core.RouteTo("Settings")
		}
	}

	var cmd tea.Cmd
	m.nav, cmd = m.nav.Update(msg)
	return m, cmd
}

// refreshStatus re-probes which servers are up.
func (m HomeModel) refreshStatus() HomeModel {
	running := make([]bool, len(m.servers))
	for i, server := range m.servers {
		running[i] = supervisor.Running(server.Path)
	}

	m.running = running
	return m
}

func (m HomeModel) View() tea.View {
	return tea.NewView(lipgloss.Place(
		m.layout.Width, m.layout.Height,
		lipgloss.Center, lipgloss.Center,
		m.content(),
	))
}

// content is the whole page as one block: the logo, the menu, the servers and
// the hints.
//
// View draws it and SetLayout measures it, so where the menu lands on screen
// and where its clicks are answered are worked out from the same rows. Any
// other arrangement drifts the moment the page grows a line.
func (m HomeModel) content() string {
	logo, width := styles.Logo(assets.Logo2)

	blocks := []string{
		logo,
		"",
		styles.Container(styles.ContainerOpts{
			Width:   width,
			Height:  m.nav.Height() + 2,
			Focused: true,
			Title:   "Menu",
			Content: m.nav.View().Content,
		}),
	}

	if servers := m.serversPane(width); servers != "" {
		blocks = append(blocks, servers)
	}

	blocks = append(blocks, styles.Hints(width,
		styles.Hint{Key: "↑/↓", Desc: "move"},
		styles.Hint{Key: "enter", Desc: "open"},
		styles.Hint{Key: "?", Desc: "help"},
		styles.Hint{Key: "q", Desc: "quit"},
	))

	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

// serversPane lists the servers under the menu, as many as there is room for.
// It is left out entirely when there are none, so a first run is not greeted by
// an empty box.
func (m HomeModel) serversPane(width int) string {
	if len(m.servers) == 0 {
		return ""
	}

	rows := make([]string, 0, len(m.servers))
	for i, server := range m.servers[:min(len(m.servers), m.serverRows())] {
		rows = append(rows, styles.ListRow(width-4, false,
			components.ServerName(server, i < len(m.running) && m.running[i]),
			components.ServerDetails(server),
		))
	}

	if hidden := len(m.servers) - len(rows); hidden > 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(theme.Inactive).
			Render("  and "+plural(hidden, "more server")))
	}

	return styles.Container(styles.ContainerOpts{
		Width:   width,
		Height:  len(rows) + 2,
		Title:   "Servers",
		Content: lipgloss.JoinVertical(lipgloss.Left, rows...),
	})
}

// serverRows is how many servers fit under the logo, the menu and the hints.
func (m HomeModel) serverRows() int {
	logo, _ := styles.Logo(assets.Logo2)

	// The logo, its blank, the menu box, the hint line, and this box's frame.
	used := lipgloss.Height(logo) + logoGap + m.nav.Height() + 2 + 1 + 2
	return max(m.layout.Height-used, 0)
}

// relayout re-measures the page after something changed its height.
func (m HomeModel) relayout() HomeModel {
	return m.SetLayout(m.layout).(HomeModel)
}

// plural renders a count with its noun.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return lipgloss.NewStyle().Render(noun) + "s"
}

// Help lists the menu's keys, including the letter shortcuts that jump
// straight to a page.
func (m HomeModel) Help() []core.HelpSection {
	return []core.HelpSection{{
		Title: "Home",
		Hints: []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "enter", Desc: "open"},
			{Key: "c", Desc: "create server"},
			{Key: "m", Desc: "manage servers"},
			{Key: "g", Desc: "migrate server"},
			{Key: "s", Desc: "settings"},
			{Key: "q", Desc: "quit"},
		},
	}}
}

// additional methods
func (m HomeModel) Title() string {
	return "Home"
}

func (m HomeModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l

	// The page is centred in both directions, so the menu sits below whatever
	// blank rows the centring put above the logo. Telling it exactly where its
	// rows land is what lets it answer clicks.
	logo, width := styles.Logo(assets.Logo2)
	top := max((l.Height-lipgloss.Height(m.content()))/2, 0)

	menu := core.Layout{
		X:      l.X + max(l.Width-width, 0)/2,
		Y:      l.Y + top + lipgloss.Height(logo) + logoGap,
		Width:  width,
		Height: m.nav.Height() + 2,
	}

	m.nav = m.nav.SetLayout(menu.Inner().Inset(1, 0, -2, 0))
	return m
}
