package actions

import (
	"fmt"
	"sort"
	"strings"
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

// playersInterval is how often the panel re-reads who is on. It goes out to
// the server's status port rather than to its console, so it can be brisk
// without filling the log.
const playersInterval = 5 * time.Second

// liveMsg carries what a server said about itself.
type liveMsg struct {
	dir  string
	live shared.Live
}

// prompt is what a player action is waiting to be told.
type prompt struct {
	title   string
	verb    string
	choices []string
	command func(player, answer string) string
}

// The actions that can be taken against a player. Each is a key, a label and
// the command it turns into — which is all a moderation action is: a line the
// server already understands, sent for you with the name filled in.
var playerActions = []struct {
	key    string
	label  string
	needs  *prompt
	simple func(player string) string
	live   bool // needs the server to be running
	op     bool // acts on operator status
}{
	{key: "k", label: "Kick", live: true, needs: &prompt{
		title: "Kick", verb: "reason",
		command: func(player, reason string) string {
			return strings.TrimSpace("kick " + player + " " + reason)
		},
	}},
	{key: "b", label: "Ban", live: true, needs: &prompt{
		title: "Ban", verb: "reason",
		command: func(player, reason string) string {
			return strings.TrimSpace("ban " + player + " " + reason)
		},
	}},
	{
		key: "u", label: "Unban", live: true,
		simple: func(player string) string { return "pardon " + player },
	},

	{
		key: "o", label: "Op", live: true, op: true,
		simple: func(player string) string { return "op " + player },
	},
	{
		key: "w", label: "Whitelist", live: true,
		simple: func(player string) string { return "whitelist add " + player },
	},

	{key: "m", label: "Message", live: true, needs: &prompt{
		title: "Message", verb: "message",
		command: func(player, message string) string {
			return "tell " + player + " " + message
		},
	}},
	{key: "t", label: "Teleport", live: true, needs: &prompt{
		title: "Teleport", verb: "destination player",
		command: func(player, target string) string {
			return "tp " + player + " " + target
		},
	}},
	{key: "g", label: "Game mode", live: true, needs: &prompt{
		title: "Game mode", verb: "mode",
		choices: []string{"survival", "creative", "adventure", "spectator"},
		command: func(player, mode string) string {
			return "gamemode " + mode + " " + player
		},
	}},
}

// PlayersModel is who has played on a server: the roster down one side, and
// what is known about whoever is chosen beside it.
type PlayersModel struct {
	base

	players []shared.Player
	live    shared.Live

	filter core.Filter
	ticker core.Ticker

	// asking is the action waiting for something to be typed, if any.
	asking *prompt
	answer core.Field

	// choosing is the index into a prompt's choices when it has them.
	choosing int
}

// NewPlayersModel returns the players panel.
func NewPlayersModel() PlayersModel {
	return PlayersModel{
		base:   newBase(),
		filter: core.NewFilter(),
		ticker: core.NewTicker("players", playersInterval),
		answer: core.NewField("").WithPrompt("» ").WithLimit(200).SetWidth(30),
	}
}

func (m PlayersModel) Init() tea.Cmd {
	return tea.Batch(core.Wake(), m.ask())
}

// Editing reports whether something is being typed.
func (m PlayersModel) Editing() bool { return m.filter.IsOpen() || m.answer.IsOpen() }

// ask reads the server's live state in the background.
func (m PlayersModel) ask() tea.Cmd {
	dir := m.dir()
	if m.needsServer() || !supervisor.Running(dir) {
		return nil
	}

	return func() tea.Msg { return liveMsg{dir: dir, live: shared.Ask(dir)} }
}

func (m PlayersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.common(msg); handled {
		m.base = updated
		return m, cmd
	}

	switch msg := msg.(type) {
	case core.WakeMsg:
		ticker, cmd := m.ticker.Restart()
		m.ticker = ticker
		return m.reload(), tea.Batch(cmd, m.ask())

	case core.TickMsg:
		ticker, ours := m.ticker.Accepts(msg)
		if !ours {
			return m, nil
		}

		armed, cmd := ticker.Arm()
		m.ticker = armed

		return m.reload(), tea.Batch(cmd, m.ask())

	case liveMsg:
		if msg.dir == m.dir() {
			m.live = msg.live
		}
		return m.reload(), nil

	case core.RefreshMsg:
		return m.reload(), nil

	case tea.MouseMsg:
		updated, _ := m.handleMouse(msg)
		m.base = updated
		return m, nil

	case tea.KeyPressMsg:
		return m.key(msg)
	}

	return m, nil
}

// key handles a keypress.
func (m PlayersModel) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.asking != nil {
		return m.answering(msg)
	}

	if m.filter.IsOpen() {
		filter, cmd := m.filter.HandleKey(msg)
		m.filter = filter
		m.base = m.setCount(len(m.visible()))

		return m, cmd
	}

	if list, handled := m.list.HandleKey(msg.String()); handled {
		m.list = list
		return m, nil
	}

	switch msg.String() {
	case "/":
		filter, cmd := m.filter.Open()
		m.filter = filter
		return m, cmd

	case "backspace":
		if m.filter.Active() {
			m.filter = m.filter.Clear()
			m.base = m.setCount(len(m.visible()))
			return m, nil
		}
		return m, core.CloseAction()

	case "r":
		return m.reload(), m.ask()
	}

	return m.act(msg.String())
}

// act runs the action a key stands for against the chosen player.
func (m PlayersModel) act(key string) (tea.Model, tea.Cmd) {
	player, ok := m.chosen()
	if !ok {
		return m, nil
	}

	for _, action := range playerActions {
		if action.key != key {
			continue
		}

		if action.live && !supervisor.Running(m.dir()) {
			updated, cmd := m.report("the server has to be running for that", true)
			m.base = updated
			return m, cmd
		}

		if action.needs != nil {
			m.asking = action.needs
			m.choosing = 0

			if len(action.needs.choices) > 0 {
				return m, nil
			}

			field, cmd := m.answer.Open("")
			m.answer = field
			return m, cmd
		}

		// Op and whitelist are toggles: the same key takes it back.
		command := action.simple(player.Name)
		switch {
		case action.op && player.Op:
			command = "deop " + player.Name
		case action.key == "w" && player.Whitelisted:
			command = "whitelist remove " + player.Name
		}

		return m.send(command)
	}

	return m, nil
}

// answering handles the keys while an action is waiting to be told something.
func (m PlayersModel) answering(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if len(m.asking.choices) > 0 {
		switch msg.String() {
		case "up", "k":
			m.choosing = (m.choosing - 1 + len(m.asking.choices)) % len(m.asking.choices)
			return m, nil
		case "down", "j":
			m.choosing = (m.choosing + 1) % len(m.asking.choices)
			return m, nil
		case "esc", "backspace":
			m.asking = nil
			return m, nil
		case "enter":
			player, ok := m.chosen()
			if !ok {
				m.asking = nil
				return m, nil
			}

			command := m.asking.command(player.Name, m.asking.choices[m.choosing])
			m.asking = nil

			return m.send(command)
		}
		return m, nil
	}

	field, result, cmd := m.answer.HandleKey(msg)
	m.answer = field

	switch result {
	case core.FieldCommitted:
		player, ok := m.chosen()
		if !ok {
			m.asking = nil
			return m, nil
		}

		command := m.asking.command(player.Name, strings.TrimSpace(m.answer.Value()))
		m.asking = nil
		m.answer = m.answer.Cancel().WithValue("")

		return m.send(command)

	case core.FieldCancelled:
		m.asking = nil
		m.answer = m.answer.Cancel().WithValue("")
		return m, nil
	}

	return m, cmd
}

// send gives the server a command and says so.
func (m PlayersModel) send(command string) (tea.Model, tea.Cmd) {
	dir := m.dir()

	updated, cmd := m.report("sent: "+command, false)
	m.base = updated

	return m, tea.Batch(cmd, core.SendCommand(dir, command), func() tea.Msg {
		// The rosters are files the server rewrites; give it a moment before
		// reading them again, or the panel shows the state from before.
		time.Sleep(400 * time.Millisecond)
		return core.RefreshMsg{}
	})
}

// reload rereads the roster.
func (m PlayersModel) reload() PlayersModel {
	if m.needsServer() {
		return m
	}

	players, err := shared.Players(m.dir())
	if err == nil {
		if supervisor.Running(m.dir()) && m.live.Named() {
			players = shared.MarkOnline(players, m.live.Names)
		}
		m.players = players
	}

	sort.SliceStable(m.players, func(a, b int) bool {
		if m.players[a].Online != m.players[b].Online {
			return m.players[a].Online
		}
		return strings.ToLower(m.players[a].Name) < strings.ToLower(m.players[b].Name)
	})

	m.base = m.setCount(len(m.visible()))
	return m
}

// visible is the roster as the filter leaves it.
func (m PlayersModel) visible() []shared.Player {
	if !m.filter.Active() {
		return m.players
	}

	kept := make([]shared.Player, 0, len(m.players))
	for _, player := range m.players {
		if m.filter.Matches(player.Name) {
			kept = append(kept, player)
		}
	}
	return kept
}

// chosen is the player the cursor is on.
func (m PlayersModel) chosen() (shared.Player, bool) {
	visible := m.visible()
	if len(visible) == 0 || m.list.Cursor >= len(visible) {
		return shared.Player{}, false
	}
	return visible[m.list.Cursor], true
}

func (m PlayersModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	if len(m.players) == 0 {
		return m.emptyView(m.title(), "Nobody has played on this server yet.",
			styles.Hint{Key: "backspace", Desc: "back"})
	}

	// The roster on the left, what is known about one player on the right.
	// The split is by proportion so a narrow pane still shows both.
	width := m.layout.ContentWidth()
	rosterWidth := max(min(width/2, 28), 14)
	detailWidth := max(width-rosterWidth-1, 10)

	roster := m.rosterView(rosterWidth)
	detail := m.detailView(detailWidth)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(rosterWidth).Render(roster),
		lipgloss.NewStyle().Foreground(theme.Inactive).Render("│"),
		lipgloss.NewStyle().Width(detailWidth).PaddingLeft(1).Render(detail),
	)

	return m.panelView(m.title(), body, m.Hints()...)
}

// rosterView is the list of players.
func (m PlayersModel) rosterView(width int) string {
	rows := m.list.Render(func(i int) string {
		visible := m.visible()
		if i >= len(visible) {
			return ""
		}

		player := visible[i]

		dot := lipgloss.NewStyle().Foreground(theme.Inactive).Render("○")
		if player.Online {
			dot = lipgloss.NewStyle().Foreground(theme.Accent).Render("●")
		}

		return styles.ListRow(width-1, i == m.list.Cursor, dot+" "+player.Name, "")
	})

	if line := m.filter.View(); line != "" {
		return line + "\n" + rows
	}
	return rows
}

// detailView is everything known about the chosen player.
func (m PlayersModel) detailView(width int) string {
	if m.asking != nil {
		return m.promptView()
	}

	player, ok := m.chosen()
	if !ok {
		return lipgloss.NewStyle().Foreground(theme.Inactive).Render("Nobody matches.")
	}

	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	state := muted.Render("offline")
	if player.Online {
		state = lipgloss.NewStyle().Foreground(theme.Accent).Render("online")
	}

	rows := []string{
		lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(player.Name),
		state,
		"",
	}

	rows = append(rows, detailRow("operator", yesNo(player.Op), width))
	if player.Op && player.Level > 0 {
		rows = append(rows, detailRow("level", fmt.Sprint(player.Level), width))
	}

	rows = append(rows,
		detailRow("whitelisted", yesNo(player.Whitelisted), width),
		detailRow("banned", yesNo(player.Banned), width),
	)

	if player.Banned && player.Reason != "" {
		rows = append(rows, detailRow("reason", player.Reason, width))
	}

	if !player.LastSeen.IsZero() {
		rows = append(rows, detailRow("last seen", ago(player.LastSeen), width))
	}
	if player.UUID != "" {
		rows = append(rows, detailRow("uuid", player.UUID, width))
	}

	rows = append(rows, "", muted.Render(m.actionsLine()))

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// promptView is the question an action is waiting on.
func (m PlayersModel) promptView() string {
	player, _ := m.chosen()

	muted := lipgloss.NewStyle().Foreground(theme.Inactive)
	rows := []string{
		lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).
			Render(m.asking.title + " " + player.Name),
		"",
	}

	if len(m.asking.choices) > 0 {
		for i, choice := range m.asking.choices {
			rows = append(rows, styles.ListRow(20, i == m.choosing, choice, ""))
		}
		rows = append(rows, "", muted.Render("[enter] do it   [esc] cancel"))

		return lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	rows = append(rows,
		muted.Render(m.asking.verb+":"),
		m.answer.View(),
		"",
		muted.Render("[enter] do it   [esc] cancel"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// actionsLine names the keys that act on the chosen player.
func (m PlayersModel) actionsLine() string {
	parts := make([]string, 0, len(playerActions))
	for _, action := range playerActions {
		parts = append(parts, "["+action.key+"] "+strings.ToLower(action.label))
	}

	return strings.Join(parts, "  ")
}

// detailRow is one label and value of the detail side.
//
// A narrow pane puts the value under its label rather than squeezing both onto
// a line neither of them fits on.
func detailRow(label, value string, width int) string {
	muted := lipgloss.NewStyle().Foreground(theme.Inactive)
	primary := lipgloss.NewStyle().Foreground(theme.Primary)

	labelWidth := 13
	if width < labelWidth+8 {
		return muted.Render(label) + "\n  " + primary.MaxWidth(max(width-2, 4)).Render(value)
	}

	return muted.Width(labelWidth).Render(label) +
		primary.MaxWidth(max(width-labelWidth, 4)).Render(value)
}

// yesNo renders a flag.
func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

// ago renders a time as how long ago it was.
func ago(when time.Time) string {
	since := time.Since(when)

	switch {
	case since < 0:
		return "recently"
	case since < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(since.Minutes()))
	case since < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(since.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(since.Hours()/24))
	}
}

// title names the panel and how many players are on right now.
func (m PlayersModel) title() string {
	title := m.Title()

	switch {
	case !supervisor.Running(m.dir()):
		return title + " · " + fmt.Sprint(len(m.players))
	case m.live.Up:
		return fmt.Sprintf("%s · %d of %d online", title, m.live.Online, m.live.Max)
	default:
		return title + " · " + fmt.Sprint(len(m.players))
	}
}

// Hints are the keys this panel responds to.
func (m PlayersModel) Hints() []styles.Hint {
	if m.asking != nil {
		return []styles.Hint{
			{Key: "enter", Desc: "do it"},
			{Key: "esc", Desc: "cancel"},
		}
	}

	return []styles.Hint{
		{Key: "↑/↓", Desc: "player"},
		{Key: "/", Desc: "filter"},
		{Key: "k/b/o/w", Desc: "kick, ban, op, whitelist"},
		{Key: "backspace", Desc: "back"},
	}
}

func (m PlayersModel) Title() string { return "Players" }

func (m PlayersModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(len(m.visible()))
	return m
}

func (m PlayersModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m PlayersModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.live = shared.Live{}
	m.list = core.List{}

	return m.reload()
}

// TextPane reports the roster's scroll, so a selection follows it.
func (m PlayersModel) TextPane() core.Pane {
	return core.Pane{Bounds: m.layout.Text(), Offset: m.list.Scroll.Offset}
}
