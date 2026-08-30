package actions

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/expose"
	"github.com/limelamp/osmium/internal/properties"
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

	// live is what the server says about itself on its status port, and props
	// is what its own settings file says. Between them they answer nearly
	// everything this panel shows without a word being typed into the console.
	live  shared.Live
	props *properties.Properties

	// usage and previous are two samples of the process, which is what a
	// percentage of a processor is made from.
	usage    shared.Usage
	previous shared.Usage

	ticker core.Ticker
	scroll core.Scroll
}

// NewDetailsModel returns the panel that describes the selected server.
func NewDetailsModel() DetailsModel {
	m := DetailsModel{
		base:   newBase(),
		ticker: core.NewTicker("details", detailsInterval),
	}
	return m
}

func (m DetailsModel) Init() tea.Cmd {
	return tea.Batch(core.Wake(), m.askServer(), m.measure())
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

		if pid := supervisor.PID(m.dir()); pid > 0 {
			m.previous, m.usage = m.usage, shared.Sample(pid)
		}

		// Coming back to a panel that was already open does not run Init
		// again, so the measurement is started here instead - otherwise
		// switching servers leaves it saying "measuring…" for good.
		if !m.sized && !m.measuring {
			m.measuring = true
			return m.reload(), tea.Batch(cmd, m.measure(), m.askServer())
		}

		return m.reload(), tea.Batch(cmd, m.askServer())

	case liveMsg:
		if msg.dir == m.dir() {
			m.live = msg.live
		}
		return m, nil

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
			return m.reload(), tea.Batch(m.askServer(), m.measure())
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

	m.props, _ = properties.Read(m.dir())

	players, err := shared.Players(m.dir())
	if err == nil {
		// A stopped server has nobody on it, whatever was last heard.
		if supervisor.Running(m.dir()) && m.live.Named() {
			players = shared.MarkOnline(players, m.live.Names)
		}
		m.players = players
	}

	if m.live.Max > 0 {
		m.max = m.live.Max
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

// askServer reads the server's live state off its status port.
//
// This used to send "list" into the console and read the answer back out of
// the log, which meant a panel being open filled the log with Osmium's own
// questions. The port every Minecraft client uses to show a server in its list
// answers the same thing and leaves the console alone.
func (m DetailsModel) askServer() tea.Cmd {
	dir := m.dir()
	if !supervisor.Running(dir) {
		return nil
	}

	return func() tea.Msg { return liveMsg{dir: dir, live: shared.Ask(dir)} }
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

	if usage := m.usageRows(); len(usage) > 0 {
		rows = append(rows, m.block("RESOURCES", usage)...)
	}

	rows = append(rows, m.block("SERVER", m.serverRows())...)

	if world := m.worldRows(); len(world) > 0 {
		rows = append(rows, m.block("WORLD", world)...)
	}

	rows = append(rows, m.block("PLAYERS", m.playerRows())...)
	rows = append(rows, m.block("CONTENT", m.contentRows())...)
	rows = append(rows, m.block("CONNECT", m.connectRows())...)

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

	rows := []string{
		m.detail("state", state, uptime),
		m.detail("allocated", m.setting(func(c *config.OsmiumConfig) string { return c.AllocatedRAM }), ""),
	}

	// What the server itself says, which is the only account of it that comes
	// from the running thing rather than from files beside it.
	if m.live.Up {
		rows = append(rows,
			m.detail("running", m.live.Version, fmt.Sprintf("answered in %d ms", m.live.Latency.Milliseconds())))

		if m.live.MOTD != "" {
			rows = append(rows, m.detail("motd", m.live.MOTD, ""))
		}
	}

	return rows
}

// usageRows are what the process is actually using, as opposed to what it was
// promised.
func (m DetailsModel) usageRows() []string {
	if !m.usage.Known {
		return nil
	}

	rows := []string{m.detail("memory in use", styles.FormatBytes(m.usage.Memory), m.memoryBar())}

	if percent := m.usage.Percent(m.previous); percent > 0 {
		rows = append(rows, m.detail("processor", fmt.Sprintf("%.0f%%", percent), ""))
	}

	return rows
}

// memoryBar draws what is in use against what was allocated, when the
// allocation is known.
func (m DetailsModel) memoryBar() string {
	if m.conf == nil || m.conf.AllocatedRAM == "" {
		return ""
	}

	allocated := allocatedBytes(m.conf.AllocatedRAM)
	if allocated <= 0 {
		return ""
	}

	return fmt.Sprintf("of %s", styles.FormatBytes(allocated))
}

// worldRows are the server's own settings, as its settings file has them.
func (m DetailsModel) worldRows() []string {
	summary := m.props.Summary()
	if len(summary) == 0 {
		return nil
	}

	rows := make([]string, 0, len(summary))
	for _, row := range summary {
		rows = append(rows, m.detail(row[0], row[1], ""))
	}
	return rows
}

// connectRows are the addresses that reach this server.
func (m DetailsModel) connectRows() []string {
	port := 25565
	if m.props != nil {
		port = m.props.Port()
	}

	rows := []string{m.detail("this machine", fmt.Sprintf("localhost:%d", port), "")}

	if lan := expose.LocalAddress(); lan != "" {
		rows = append(rows, m.detail("same network", fmt.Sprintf("%s:%d", lan, port), ""))
	}

	rows = append(rows, m.detail("", lipgloss.NewStyle().Foreground(theme.Inactive).
		Render("osmium expose --open forwards the port"), ""))

	return rows
}

// allocatedBytes reads "4G" or "3072M" as a number of bytes.
func allocatedBytes(allocated string) int64 {
	value := strings.ToUpper(strings.TrimSpace(allocated))
	if value == "" {
		return 0
	}

	unit := value[len(value)-1]

	number, err := strconv.ParseInt(strings.TrimRight(value, "GM"), 10, 64)
	if err != nil {
		return 0
	}

	if unit == 'G' {
		return number * 1024 * 1024 * 1024
	}
	return number * 1024 * 1024
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
		return "-"
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

// TextPane reports this panel's own scroll: the details are a block of rows
// rather than a list, so the base's list offset would not describe them.
func (m DetailsModel) TextPane() core.Pane {
	return core.Pane{Bounds: m.layout.Text(), Offset: m.scroll.Offset}
}

// Hints are the keys this panel responds to.
func (m DetailsModel) Hints() []styles.Hint {
	return []styles.Hint{
		{Key: "↑/↓", Desc: "scroll"},
		{Key: "r", Desc: "refresh"},
		{Key: "backspace", Desc: "back"},
	}
}

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
