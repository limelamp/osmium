package actions

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// playersInterval is how often the panel re-reads the player files and asks the
// server who is connected.
const playersInterval = 3 * time.Second

// PlayersModel lists who plays on a server and what they are allowed to do, and
// changes it: op, kick, ban and pardon are console commands, so they work while
// the server is running and are refused when it is not.
type PlayersModel struct {
	base

	players []shared.Player
	table   table.Model
	max     int

	ticker core.Ticker
}

func NewPlayersModel() PlayersModel {
	return PlayersModel{
		base:   newBase(),
		table:  newPlayerTable(),
		ticker: core.NewTicker("players", playersInterval),
	}
}

// newPlayerTable builds the table in the app's colours.
func newPlayerTable() table.Model {
	built := table.New(
		table.WithColumns([]table.Column{
			{Title: "PLAYER", Width: 18},
			{Title: "STATUS", Width: 9},
			{Title: "ROLE", Width: 8},
			{Title: "LAST SEEN", Width: 12},
		}),
		table.WithFocused(true),
	)

	styling := table.DefaultStyles()
	styling.Header = styling.Header.Foreground(theme.Inactive).Bold(false)
	styling.Selected = styling.Selected.Foreground(theme.Primary).Bold(true)
	built.SetStyles(styling)

	return built
}

func (m PlayersModel) Init() tea.Cmd {
	return tea.Batch(core.Wake(), m.ask())
}

// Editing reports that the panel takes no typed input.
func (m PlayersModel) Editing() bool { return false }

func (m PlayersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.WakeMsg:
		ticker, cmd := m.ticker.Restart()
		m.ticker = ticker
		return m.reload(), cmd

	case core.TickMsg:
		ticker, ours := m.ticker.Accepts(msg)
		if !ours {
			return m, nil
		}

		armed, cmd := ticker.Arm()
		m.ticker = armed
		return m.reload(), tea.Batch(cmd, m.ask())

	case tea.KeyPressMsg:
		switch msg.String() {
		case "backspace":
			return m, core.CloseAction()

		case "o":
			return m.command("op", "%s is now an operator")
		case "d":
			return m.command("deop", "%s is no longer an operator")
		case "k":
			return m.command("kick", "kicked %s")
		case "b":
			return m.command("ban", "banned %s")
		case "p":
			return m.command("pardon", "pardoned %s")
		}
	}

	updated, cmd := m.table.Update(msg)
	m.table = updated
	return m, cmd
}

// reload re-reads the player files and who the server last said was online.
func (m PlayersModel) reload() PlayersModel {
	if m.needsServer() {
		return m
	}

	players, err := shared.Players(m.dir())
	if err != nil {
		m.err = err
		return m
	}

	m.max = shared.MaxPlayers(m.dir())
	if names, _, max, ok := onlineNow(m.dir()); ok {
		players = shared.MarkOnline(players, names)
		if max > 0 {
			m.max = max
		}
	}

	m.players = players
	m.table.SetRows(m.rows())
	return m
}

// ask sends "list" to a running server, whose answer the next reload reads.
func (m PlayersModel) ask() tea.Cmd {
	dir := m.dir()
	if !supervisor.Running(dir) {
		return nil
	}

	return func() tea.Msg {
		_ = supervisor.Exec(dir, "list")
		return nil
	}
}

// rows renders the players as table rows.
func (m PlayersModel) rows() []table.Row {
	rows := make([]table.Row, 0, len(m.players))

	for _, player := range m.players {
		status := "offline"
		switch {
		case player.Banned:
			status = "banned"
		case player.Online:
			status = "online"
		}

		role := "—"
		if player.Op {
			role = fmt.Sprintf("op %d", player.Level)
		}

		rows = append(rows, table.Row{player.Name, status, role, lastSeen(player)})
	}
	return rows
}

// command sends a console command for the player under the cursor.
func (m PlayersModel) command(verb, report string) (tea.Model, tea.Cmd) {
	player := m.selected()
	if player == "" {
		return m, nil
	}

	if !supervisor.Running(m.dir()) {
		return m.say("start the server to change players", true)
	}

	if err := supervisor.Exec(m.dir(), verb+" "+player); err != nil {
		return m.say(err.Error(), true)
	}

	return m.say(fmt.Sprintf(report, player), false)
}

// say puts a line in the panel's footer for a moment.
func (m PlayersModel) say(text string, failed bool) (tea.Model, tea.Cmd) {
	updated, cmd := m.base.report(text, failed)
	m.base = updated
	return m, cmd
}

// selected is the name in the highlighted row.
func (m PlayersModel) selected() string {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

// lastSeen renders when a player was last on, roughly.
func lastSeen(player shared.Player) string {
	switch {
	case player.Online:
		return "now"
	case player.LastSeen.IsZero():
		return "—"
	}

	// The user cache stores when the entry expires, a month after the player
	// was seen, so this is "about when" rather than exactly.
	elapsed := time.Since(player.LastSeen.AddDate(0, -1, 0))
	switch {
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", max(int(elapsed.Minutes()), 1))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours())/24)
	}
}

func (m PlayersModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	if len(m.players) == 0 {
		return m.emptyView(m.Title(),
			"Nobody has played on this server yet.",
			styles.Hint{Key: "backspace", Desc: "back"})
	}

	m.table.SetHeight(max(m.layout.ContentHeight()-2, 3))
	m.table.SetWidth(max(m.layout.ContentWidth(), 20))

	return m.panelView(m.title(), m.table.View(), m.Hints()...)
}

// title names the panel and how many players are on right now.
func (m PlayersModel) title() string {
	online := 0
	for _, player := range m.players {
		if player.Online {
			online++
		}
	}

	title := m.Title() + " · " + m.server.Name
	if m.max > 0 {
		title += fmt.Sprintf(" · %d/%d online", online, m.max)
	}
	return title
}

// Hints are the keys this panel responds to.
func (m PlayersModel) Hints() []styles.Hint {
	if !supervisor.Running(m.dir()) {
		return []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "backspace", Desc: "back"},
		}
	}

	return []styles.Hint{
		{Key: "o/d", Desc: "op / deop"},
		{Key: "k", Desc: "kick"},
		{Key: "b/p", Desc: "ban / pardon"},
		{Key: "backspace", Desc: "back"},
	}
}

// additional methods
func (m PlayersModel) Title() string {
	return "Players"
}

func (m PlayersModel) SetLayout(l core.Layout) core.Action {
	m.layout = l

	// The name column takes whatever the fixed columns leave.
	columns := m.table.Columns()
	if len(columns) == 4 {
		columns[0].Width = max(l.ContentWidth()-33, 10)
		m.table.SetColumns(columns)
	}
	return m
}

func (m PlayersModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m PlayersModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	return m.reload()
}
