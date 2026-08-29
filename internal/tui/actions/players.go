package actions

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	max     int

	ticker core.Ticker
}

// The widths of the columns after the name, which takes what is left.
const (
	statusWidth   = 9
	roleWidth     = 8
	lastSeenWidth = 10
	headerRows    = 1
)

// NewPlayersModel returns the panel that lists and manages players.
func NewPlayersModel() PlayersModel {
	return PlayersModel{
		base:   newBase(),
		ticker: core.NewTicker("players", playersInterval),
	}
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

	case tea.MouseMsg:
		// The header sits above the rows, so a click is one row further down
		// than it looks from the panel's edge.
		updated, _ := m.list.HandleMouse(msg, m.layout.Inner(), headerRows)
		m.list = updated
		return m, nil

	case tea.KeyPressMsg:
		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

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

	return m, nil
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

	// Who was online is read from the server's own answer in the log, which is
	// only true of a server that is still running.
	if supervisor.Running(m.dir()) {
		if names, limit, ok := onlineNow(m.dir()); ok {
			players = shared.MarkOnline(players, names)
			if limit > 0 {
				m.max = limit
			}
		}
	}

	m.players = players
	m.base = m.setCount(len(players))
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

// header names the columns, indented by the width of the cursor marker so the
// titles sit over the values they name.
func (m PlayersModel) header() string {
	return lipgloss.NewStyle().Foreground(theme.Inactive).Render("  " + m.columns(
		"PLAYER", "STATUS", "ROLE", "LAST SEEN"))
}

// row renders one player across the columns, with the cursor's marker.
func (m PlayersModel) playerRow(index int) string {
	player := m.players[index]

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

	marker := "  "
	style := lipgloss.NewStyle()
	if index == m.list.Cursor {
		marker = "➔ "
		style = style.Foreground(theme.Primary).Bold(true)
	}

	return style.Render(marker + m.columns(player.Name, status, role, lastSeen(player)))
}

// columns lays four values out in the panel's fixed columns, so the header and
// the rows line up whatever the values are.
func (m PlayersModel) columns(name, status, role, seen string) string {
	nameWidth := max(m.layout.ContentWidth()-statusWidth-roleWidth-lastSeenWidth-2, 8)

	cell := func(value string, width int) string {
		return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(value)
	}

	return cell(name, nameWidth) + cell(status, statusWidth) +
		cell(role, roleWidth) + cell(seen, lastSeenWidth)
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
	updated, cmd := m.report(text, failed)
	m.base = updated
	return m, cmd
}

// selected is the name in the highlighted row.
func (m PlayersModel) selected() string {
	if m.list.Cursor < 0 || m.list.Cursor >= len(m.players) {
		return ""
	}
	return m.players[m.list.Cursor].Name
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

	return m.panelView(m.title(),
		m.header()+"\n"+m.list.Render(m.playerRow), m.Hints()...)
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
		{Key: "↑/↓", Desc: "move"},
		{Key: "o/d", Desc: "op / deop"},
		{Key: "k", Desc: "kick"},
		{Key: "b/p", Desc: "ban / pardon"},
		{Key: "backspace", Desc: "back"},
	}
}

func (m PlayersModel) Title() string {
	return "Players"
}

func (m PlayersModel) SetLayout(l core.Layout) core.Action {
	m.layout = l

	// One row goes to the header, and the footer takes the rest.
	m.base = m.setCount(len(m.players))
	m.list = m.list.Resize(len(m.players), max(l.ContentHeight()-hintRows-headerRows, 1))

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
