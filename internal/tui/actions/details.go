package actions

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// detailsInterval is how often the panel re-reads what it shows. It is a second
// so the uptime counts rather than jumps.
const detailsInterval = time.Second

// sizeReadyMsg carries the size of a server folder, which is measured in the
// background because a world can be gigabytes of small files.
type sizeReadyMsg struct {
	dir  string
	size int64
}

// DetailsModel shows everything Osmium knows about the selected server in one
// place: what it is running, where it lives, who is on it and what is installed.
type DetailsModel struct {
	base

	conf      *config.OsmiumConfig
	players   []shared.Player
	max       int
	size      int64
	sized     bool
	measuring bool

	ticker core.Ticker
	scroll core.Scroll
}

func NewDetailsModel() DetailsModel {
	m := DetailsModel{
		base:   newBase(),
		ticker: core.NewTicker("details", detailsInterval),
	}
	return m
}

func (m DetailsModel) Init() tea.Cmd {
	// The panel is opened, so it starts counting and asks the server who is on
	// it; the answer arrives in the log a moment later.
	return tea.Batch(core.Wake(), m.askWhoIsOnline(), m.measure())
}

// Editing reports that the panel takes no typed input.
func (m DetailsModel) Editing() bool { return false }

func (m DetailsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		// Coming back to a panel that was already open does not run Init
		// again, so the measurement is started here instead — otherwise
		// switching servers leaves it saying "measuring…" for good.
		if !m.sized && !m.measuring {
			m.measuring = true
			return m.reload(), tea.Batch(cmd, m.measure())
		}

		return m.reload(), cmd

	case sizeReadyMsg:
		m.measuring = false
		if msg.dir == m.dir() {
			m.size, m.sized = msg.size, true
		}
		return m, nil

	case tea.MouseMsg:
		if wheel, ok := msg.(tea.MouseWheelMsg); ok && m.layout.Hit(wheel.X, wheel.Y) {
			switch wheel.Button {
			case tea.MouseWheelUp:
				m.scroll = m.scroll.By(-core.WheelStep)
			case tea.MouseWheelDown:
				m.scroll = m.scroll.By(core.WheelStep)
			}
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "backspace":
			return m, core.CloseAction()
		case "up", "k":
			m.scroll = m.scroll.By(-1)
		case "down", "j":
			m.scroll = m.scroll.By(1)
		case "pgup":
			m.scroll = m.scroll.By(-m.scroll.Visible)
		case "pgdown":
			m.scroll = m.scroll.By(m.scroll.Visible)
		case "r":
			return m.reload(), tea.Batch(m.askWhoIsOnline(), m.measure())
		}
	}

	return m, nil
}

// reload re-reads the manifest, the player files and the running state.
func (m DetailsModel) reload() DetailsModel {
	if m.needsServer() {
		return m
	}

	m.conf, _ = config.ReadConfigIn(m.dir())
	m.max = shared.MaxPlayers(m.dir())

	players, err := shared.Players(m.dir())
	if err == nil {
		// A stopped server has nobody on it, whatever its log last said.
		if supervisor.Running(m.dir()) {
			if names, _, max, ok := onlineNow(m.dir()); ok {
				players = shared.MarkOnline(players, names)
				if max > 0 {
					m.max = max
				}
			}
		}
		m.players = players
	}

	return m.measured()
}

// measured records how tall the panel's content is now.
//
// The scroll has to be sized where the model is kept, not in View: View has a
// value receiver, so a size worked out there is thrown away, and the scroll
// then clamps every movement back to the top.
func (m DetailsModel) measured() DetailsModel {
	m.scroll = m.scroll.Resize(len(m.rows()), max(m.layout.ContentHeight()-1, 1))
	return m
}

// askWhoIsOnline sends "list" to a running server. Its reply goes to the log
// like everything else, which is where the panel reads it back from.
func (m DetailsModel) askWhoIsOnline() tea.Cmd {
	dir := m.dir()
	if !supervisor.Running(dir) {
		return nil
	}

	return func() tea.Msg {
		_ = supervisor.Exec(dir, "list")
		return nil
	}
}

// measure adds up the server folder in the background.
func (m DetailsModel) measure() tea.Cmd {
	dir := m.dir()
	if m.needsServer() {
		return nil
	}

	return func() tea.Msg {
		return sizeReadyMsg{dir: dir, size: shared.FolderSize(dir)}
	}
}

// onlineNow reads the newest answer to "list" out of the server's log.
func onlineNow(dir string) (names []string, online, max int, ok bool) {
	lines, err := supervisor.ReadLast(supervisor.LogPath(dir), 200)
	if err != nil {
		return nil, 0, 0, false
	}

	var text strings.Builder
	for _, line := range lines {
		text.WriteString(line.Text)
		text.WriteByte('\n')
	}

	return shared.OnlineFromLog(text.String())
}

func (m DetailsModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	rows := m.rows()

	start, end := m.scroll.Window()
	end = min(end, len(rows))
	return m.panelViewScrolled(m.Title()+" · "+m.server.Name,
		strings.Join(rows[start:end], "\n"), m.scroll, m.Hints()...)
}

// rows is the whole panel, block by block.
func (m DetailsModel) rows() []string {
	var rows []string

	rows = append(rows, m.block("STATUS", m.statusRows())...)
	rows = append(rows, m.block("SERVER", m.serverRows())...)
	rows = append(rows, m.block("PLAYERS", m.playerRows())...)
	rows = append(rows, m.block("CONTENT", m.contentRows())...)

	return rows[:len(rows)-1] // no trailing blank
}

// block renders a heading and its rows, followed by a blank.
func (m DetailsModel) block(title string, rows []string) []string {
	heading := lipgloss.NewStyle().Foreground(theme.Inactive).Render(title)
	return append(append([]string{heading}, rows...), "")
}

func (m DetailsModel) statusRows() []string {
	state := lipgloss.NewStyle().Foreground(theme.Inactive).Render("stopped")
	uptime := ""

	if supervisor.Running(m.dir()) {
		state = lipgloss.NewStyle().Foreground(theme.Accent).Render("running")
		uptime = fmt.Sprintf("%s · pid %d", supervisor.Uptime(m.dir()), supervisor.PID(m.dir()))
	}

	return []string{
		m.detail("state", state, uptime),
		m.detail("memory", m.setting(func(c *config.OsmiumConfig) string { return c.AllocatedRAM }), ""),
	}
}

func (m DetailsModel) serverRows() []string {
	size := "measuring…"
	if m.sized {
		size = styles.FormatBytes(m.size)
	}

	engine := m.setting(func(c *config.OsmiumConfig) string { return c.Loader + " " + c.Version })

	return []string{
		m.detail("engine", engine, ""),
		m.detail("folder", shortenPath(m.dir()), ""),
		m.detail("size on disk", size, ""),
	}
}

func (m DetailsModel) playerRows() []string {
	var online, ops, banned, whitelisted []string
	for _, player := range m.players {
		switch {
		case player.Online:
			online = append(online, player.Name)
		}
		if player.Op {
			ops = append(ops, player.Name)
		}
		if player.Banned {
			banned = append(banned, player.Name)
		}
		if player.Whitelisted {
			whitelisted = append(whitelisted, player.Name)
		}
	}

	count := fmt.Sprintf("%d", len(online))
	if m.max > 0 {
		count += fmt.Sprintf(" / %d", m.max)
	}

	rows := []string{
		m.detail("online", count, strings.Join(online, ", ")),
		m.detail("ops", names(ops), ""),
		m.detail("banned", names(banned), ""),
	}

	if len(whitelisted) > 0 {
		rows = append(rows, m.detail("whitelisted", names(whitelisted), ""))
	}
	if !supervisor.Running(m.dir()) {
		rows = append(rows, m.detail("", lipgloss.NewStyle().Foreground(theme.Inactive).
			Render("start the server to see who is on it"), ""))
	}
	return rows
}

func (m DetailsModel) contentRows() []string {
	mods, plugins := "0", "0"
	if m.conf != nil {
		mods, plugins = fmt.Sprint(len(m.conf.Mods)), fmt.Sprint(len(m.conf.Plugins))
	}

	return []string{
		m.detail("mods", mods, ""),
		m.detail("plugins", plugins, ""),
	}
}

// detail renders one label/value row, with an optional muted note after it.
func (m DetailsModel) detail(label, value, note string) string {
	rendered := lipgloss.NewStyle().Foreground(theme.Inactive).Width(16).Render("  "+label) +
		lipgloss.NewStyle().Foreground(theme.Primary).Render(value)

	if note != "" {
		rendered += lipgloss.NewStyle().Foreground(theme.Inactive).Render("   " + note)
	}

	return lipgloss.NewStyle().MaxWidth(max(m.layout.ContentWidth(), 1)).Render(rendered)
}

// setting reads one value out of the manifest, or says it is missing.
func (m DetailsModel) setting(read func(*config.OsmiumConfig) string) string {
	if m.conf == nil {
		return "unknown"
	}
	return read(m.conf)
}

// names renders a list of players, or a dash when there are none.
func names(list []string) string {
	if len(list) == 0 {
		return "—"
	}
	return strings.Join(list, ", ")
}

// shortenPath writes a path under the home directory the way a shell would.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(path, home) {
		return path
	}
	return "~" + strings.TrimPrefix(path, home)
}

// Hints are the keys this panel responds to.
func (m DetailsModel) Hints() []styles.Hint {
	return []styles.Hint{
		{Key: "↑/↓", Desc: "scroll"},
		{Key: "r", Desc: "refresh"},
		{Key: "backspace", Desc: "back"},
	}
}

// additional methods
func (m DetailsModel) Title() string {
	return "Server Details"
}

func (m DetailsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	return m.measured()
}

func (m DetailsModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m DetailsModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.sized, m.size, m.measuring = false, 0, false
	m.scroll = core.Scroll{}

	return m.reload()
}
