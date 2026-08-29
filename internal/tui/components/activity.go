package components

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// maxLogLines bounds how much history one server's console keeps in memory.
const maxLogLines = 5000

// attachHistory is how many lines of an already-running server's log are shown
// when Osmium attaches to it, so reopening the app lands in context rather than
// on an empty pane.
const attachHistory = 1000

// pollInterval is how often the attached log files are checked for new output.
const pollInterval = 300 * time.Millisecond

// statusEvery is how many polls apart the running-state checks are. Probing a
// process is far more expensive than stat-ing a file, so it happens on a much
// slower cadence than the log reading.
const statusEvery = 7

// console is everything the Activity panel keeps for one server: its output
// history, the tailer following that server's log, and where the user had
// scrolled to.
//
// One console per server is what keeps two servers from sharing a state: their
// logs stay apart, starting one does not report the other as already running,
// and switching selection returns to exactly the view you left.
type console struct {
	name   string
	dir    string // empty for the Osmium console, which belongs to no server
	lines  []supervisor.Line
	tail   *supervisor.Tailer
	status string

	// live is the progress of whatever is installing into this server: it is
	// redrawn in place while the operation runs, and kept as an ordinary line
	// once it finishes, so scrolling back shows what was downloaded.
	live      string
	liveLabel string

	// offset and follow remember this console's scroll position across
	// selection changes.
	offset int
	follow bool
}

// append adds lines to a console, trimming the oldest once it is full.
func (c *console) append(lines ...supervisor.Line) {
	c.lines = append(c.lines, lines...)
	if len(c.lines) > maxLogLines {
		c.lines = c.lines[len(c.lines)-maxLogLines:]
	}
}

// say adds one of Osmium's own notices to a console.
func (c *console) say(tag supervisor.Tag, text string) {
	c.append(supervisor.Line{Tag: tag, Text: text})
}

// attach starts following the server's log file, pulling in the history it
// already holds. A console with no server, or one already attached, is left
// alone.
func (c *console) attach() {
	if c.dir == "" || c.tail != nil {
		return
	}

	tail, history := supervisor.NewTailer(supervisor.LogPath(c.dir), attachHistory)
	c.tail = tail
	c.append(history...)
}

// ActivityModel is the console of the Manage Servers page. It shows the
// selected server's output and Osmium's own progress, and sends typed commands
// to that server's supervisor.
type ActivityModel struct {
	layout  core.Layout
	isFocus bool

	textInput textinput.Model
	viewport  viewport.Model

	// consoles is keyed by server path; current names the visible one. The
	// empty key is Osmium's own console, which is what is shown before a server
	// has been selected.
	consoles map[string]*console
	current  string

	poller  core.Ticker
	polls   int
	wrapped int // rendered line count, for the scrollbar
}

func NewActivityModel() ActivityModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter a command..."
	ti.Prompt = "› "
	ti.CharLimit = 500
	ti.SetStyles(core.FieldStyles())

	vp := viewport.New()

	// Disable the default horizontal scrolling of the viewport
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)

	// DISABLING VIM SCROLLING SHORTCUTS (annoying when used for typing):
	// Force Up and Down to ONLY use the arrow keys (removes 'k' and 'j')
	vp.KeyMap.Up.SetKeys("up")
	vp.KeyMap.Down.SetKeys("down")

	// Force PageUp and PageDown to ONLY use the actual PgUp/PgDn keys (removes 'b', 'f', and spacebar)
	vp.KeyMap.PageUp.SetKeys("pgup")
	vp.KeyMap.PageDown.SetKeys("pgdown")

	// Disable the page other jumping letters ('d', 'u')
	vp.KeyMap.HalfPageUp.SetEnabled(false)
	vp.KeyMap.HalfPageDown.SetEnabled(false)

	consoles := map[string]*console{
		"": {name: "Osmium", follow: true},
	}

	return ActivityModel{
		textInput: ti,
		viewport:  vp,
		consoles:  consoles,
		poller:    core.NewTicker("activity", pollInterval),
	}
}

// Init does nothing on its own: polling starts when a console is attached,
// which happens as soon as a server is selected or started.
func (m ActivityModel) Init() tea.Cmd { return nil }

// consoleFor returns the console for a server, creating and attaching it on
// first use.
func (m ActivityModel) consoleFor(server *storage.Server) *console {
	if server == nil || server.Path == "" {
		return m.consoles[""]
	}

	if existing, ok := m.consoles[server.Path]; ok {
		existing.name = server.Name
		return existing
	}

	created := &console{name: server.Name, dir: server.Path, follow: true}
	created.attach()
	m.consoles[server.Path] = created
	return created
}

// visible returns the console currently on screen.
func (m ActivityModel) visible() *console {
	return m.consoles[m.current]
}

// Editing reports whether a command is half-typed, which is what tells the page
// to hand esc to this pane so it can clear the line instead of leaving.
func (m ActivityModel) Editing() bool {
	return m.textInput.Value() != ""
}

func (m ActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.ServerControlMsg:
		return m.control(msg)

	case core.LogMsg:
		return m.appendVisible(msg.Lines...), nil

	case core.RestartReadyMsg:
		// The server asked to restart has finished shutting down.
		if target, ok := m.consoles[msg.Dir]; ok {
			return m.start(target)
		}
		return m, nil

	case core.TickMsg:
		poller, ours := m.poller.Accepts(msg)
		if !ours {
			return m, nil
		}
		m.poller = poller
		return m.drain()

	case tea.MouseMsg:
		mouse := msg.Mouse()
		if wheel, ok := msg.(tea.MouseWheelMsg); ok && m.layout.Hit(mouse.X, mouse.Y) {
			switch wheel.Button {
			case tea.MouseWheelUp:
				m.viewport.ScrollUp(core.WheelStep)
			case tea.MouseWheelDown:
				m.viewport.ScrollDown(core.WheelStep)
			}
			m.rememberScroll()
		}
		return m, nil

	case tea.KeyPressMsg:
		// The console only listens while it holds focus, so typing a command
		// cannot leak into the panes behind it.
		if !m.isFocus {
			return m, nil
		}

		switch msg.String() {
		case "enter":
			return m.submit()

		// esc clears the half-typed command; the page only sends it here while
		// there is something to clear, so it never swallows a way out.
		case "esc":
			m.textInput.Reset()
			return m, nil

		// Jumping to the newest line resumes following, which is the way out
		// of a console that was scrolled away from the tail.
		case "end":
			m.viewport.GotoBottom()
			m.rememberScroll()
			return m, nil

		case "pgup", "pgdown", "up", "down":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			m.rememberScroll()
			return m, cmd
		}

		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// poll schedules the next log read, if anything is being followed or anything
// is installing.
func (m ActivityModel) poll() (ActivityModel, tea.Cmd) {
	if !m.following() && !m.installing() {
		return m, nil
	}

	poller, cmd := m.poller.Arm()
	m.poller = poller
	return m, cmd
}

// following reports whether any console has a log to read.
func (m ActivityModel) following() bool {
	for _, c := range m.consoles {
		if c.tail != nil {
			return true
		}
	}
	return false
}

// installing reports whether any console's server has an operation running,
// which is what keeps the poll going long enough to draw its bar.
func (m ActivityModel) installing() bool {
	for _, c := range m.consoles {
		if c.dir == "" {
			continue
		}
		if _, reported := core.Progress(c.dir); reported || c.liveLabel != "" {
			return true
		}
	}
	return false
}

// Wake restarts the poll loop after the pane has been off screen.
//
// A tick is only delivered to the page that is on screen, so leaving Manage
// Servers strands the loop: the tick goes to whatever page took over and the
// console never hears from it again — which is what left the logs frozen until
// the app was restarted. Waking starts a fresh generation, and the stranded
// tick is dropped when it turns up.
func (m ActivityModel) Wake() (ActivityModel, tea.Cmd) {
	if !m.following() {
		m.poller = m.poller.Stop()
		return m, nil
	}

	poller, cmd := m.poller.Restart()
	m.poller = poller
	return m, cmd
}

// drain moves whatever the followed servers have written since the last poll
// into their own consoles, so a server that is not on screen still collects its
// output.
func (m ActivityModel) drain() (tea.Model, tea.Cmd) {
	m.polls++
	checkStatus := m.polls%statusEvery == 0

	for _, c := range m.consoles {
		m.followProgress(c)

		if c.tail == nil {
			continue
		}

		c.append(c.tail.Read()...)

		if checkStatus {
			c.status = "stopped"
			if supervisor.Running(c.dir) {
				c.status = "running"
			}
		}
	}

	m = m.refresh()
	return m.poll()
}

// followProgress keeps a console's live line in step with the operation running
// in its folder, and commits the last one when the operation ends.
func (m ActivityModel) followProgress(c *console) {
	if c.dir == "" {
		return
	}

	operation, reported := core.Progress(c.dir)
	if !reported {
		return
	}

	c.liveLabel = operation.Label

	c.live = ""
	if operation.Total > 0 {
		c.live = styles.ProgressBar(max(m.viewport.Width()-12, 20),
			operation.Done, operation.Total)
	}

	if !operation.Finished {
		return
	}

	// It is over, and this is where it got to: the bar stops being redrawn and
	// becomes an ordinary line of the log, so scrolling back shows what was
	// downloaded and how big it was.
	if c.live != "" {
		c.say(supervisor.TagMod, c.liveLabel+"  "+c.live)
	}

	c.live, c.liveLabel = "", ""
	core.ClearProgress(c.dir)
}

// submit sends the typed line to the visible server's supervisor.
func (m ActivityModel) submit() (tea.Model, tea.Cmd) {
	command := m.textInput.Value()
	if command == "" {
		return m, nil
	}
	m.textInput.Reset()

	current := m.visible()
	switch {
	case current == nil || current.dir == "":
		return m.say(supervisor.TagOsmium, "select a server to send it commands"), nil

	case !supervisor.Running(current.dir):
		return m.say(supervisor.TagOsmium, "no server running — start one from the Actions pane"), nil
	}

	// The command is not echoed here: the supervisor writes it to the log, so
	// it appears once, in the same order the server saw it.
	if err := supervisor.Exec(current.dir, command); err != nil {
		return m.say(supervisor.TagError, err.Error()), nil
	}
	return m, nil
}

// control starts, stops or restarts the server the request names, which is not
// necessarily the one on screen.
func (m ActivityModel) control(msg core.ServerControlMsg) (tea.Model, tea.Cmd) {
	if msg.Server == nil || msg.Server.Path == "" {
		return m.say(supervisor.TagOsmium, "select a server first"), nil
	}

	target := m.consoleFor(msg.Server)

	switch msg.Control {
	case core.ServerStop:
		return m.stop(target)

	case core.ServerRestart:
		return m.restart(target)

	default:
		return m.start(target)
	}
}

// start asks a supervisor to take the server over, then follows its log.
func (m ActivityModel) start(target *console) (tea.Model, tea.Cmd) {
	switch err := supervisor.Start(target.dir, target.name); {
	case err == supervisor.ErrAlreadyRunning:
		target.say(supervisor.TagOsmium, fmt.Sprintf("%s is already running", target.name))

	case err != nil:
		target.say(supervisor.TagError, err.Error())

	default:
		target.status = "running"
	}

	target.attach()
	m = m.refresh()
	return m.poll()
}

// stop asks a running server to shut down.
func (m ActivityModel) stop(target *console) (tea.Model, tea.Cmd) {
	if err := supervisor.Stop(target.dir, false); err != nil {
		target.say(supervisor.TagOsmium, fmt.Sprintf("%s is not running", target.name))
		return m.refresh(), nil
	}

	target.status = "stopping"
	m = m.refresh()
	return m.poll()
}

// restart stops the server and starts it again once it has actually exited,
// which a Minecraft server can take a while to do.
func (m ActivityModel) restart(target *console) (tea.Model, tea.Cmd) {
	if !supervisor.Running(target.dir) {
		return m.start(target)
	}

	if err := supervisor.Stop(target.dir, false); err != nil {
		target.say(supervisor.TagError, err.Error())
		return m.refresh(), nil
	}

	target.status = "restarting"
	target.attach()
	m = m.refresh()

	dir := target.dir
	m, cmd := m.poll()

	// Waiting happens off the update loop so the pane keeps drawing the
	// server's shutdown log while it saves its world.
	return m, tea.Batch(cmd, func() tea.Msg {
		for range 120 { // up to a minute
			if !supervisor.Running(dir) {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		return core.RestartReadyMsg{Dir: dir}
	})
}

// say writes one of Osmium's own lines into the visible console.
func (m ActivityModel) say(tag supervisor.Tag, text string) ActivityModel {
	return m.appendVisible(supervisor.Line{Tag: tag, Text: text})
}

// appendVisible writes lines into the console currently on screen.
func (m ActivityModel) appendVisible(lines ...supervisor.Line) ActivityModel {
	return m.appendTo(m.visible(), lines...)
}

// appendTo writes lines into one console, re-rendering only if it is the one
// being shown.
func (m ActivityModel) appendTo(target *console, lines ...supervisor.Line) ActivityModel {
	if target == nil || len(lines) == 0 {
		return m
	}

	target.append(lines...)
	if target != m.visible() {
		return m
	}
	return m.refresh()
}

// rememberScroll stores where the user scrolled the visible console to, so
// coming back to it lands in the same place.
func (m ActivityModel) rememberScroll() {
	if current := m.visible(); current != nil {
		current.offset = m.viewport.YOffset()
		current.follow = m.viewport.AtBottom()
	}
}

// refresh re-renders the visible console into the viewport.
func (m ActivityModel) refresh() ActivityModel {
	current := m.visible()

	rows := make([]string, 0, len(current.lines)+1)
	for _, line := range current.lines {
		rows = append(rows, styles.LogRow(m.viewport.Width(), string(line.Tag), line.Text))
	}

	// The operation in progress is the newest line, redrawn where it stands
	// rather than added to the log over and over.
	if current.liveLabel != "" {
		rows = append(rows, styles.LogRow(m.viewport.Width(), string(supervisor.TagMod),
			strings.TrimSpace(current.liveLabel+"  "+current.live)))
	}

	rendered := strings.Join(rows, "\n")
	m.viewport.SetContent(rendered)
	m.wrapped = lipgloss.Height(rendered)

	if current.follow {
		m.viewport.GotoBottom()
		current.offset = m.viewport.YOffset()
	} else {
		m.viewport.SetYOffset(current.offset)
	}

	return m
}

// RunServer View
func (m ActivityModel) View() tea.View {
	separator := lipgloss.NewStyle().
		Foreground(theme.Inactive).
		Render(strings.Repeat("─", max(m.layout.ContentWidth(), 0)))

	logs := m.viewport.View()
	if current := m.visible(); len(current.lines) == 0 {
		logs = styles.Body(lipgloss.NewStyle().Foreground(theme.Inactive).
			Render(m.emptyMessage()), m.viewport.Width(), m.viewport.Height())
	}

	// The command line sits at the very bottom of the pane, under a rule, so it
	// reads as a prompt rather than as another log line.
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		logs,
		separator,
		m.textInput.View(),
	)

	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:         m.layout.Width,
		Height:        m.layout.Height,
		Focused:       m.isFocus,
		Title:         m.title(),
		Content:       content,
		ScrollOffset:  m.viewport.YOffset(),
		ScrollTotal:   m.wrapped,
		ScrollVisible: m.viewport.Height(),
	}))
}

// Hints are the keys this pane responds to.
func (m ActivityModel) Hints() []styles.Hint {
	hints := []styles.Hint{
		{Key: "type", Desc: "server command"},
		{Key: "enter", Desc: "send"},
		{Key: "esc", Desc: "clear line"},
		{Key: "↑/↓", Desc: "scroll"},
	}

	if current := m.visible(); current != nil && !current.follow {
		hints = append(hints, styles.Hint{Key: "end", Desc: "resume following"})
	}

	return append(hints, styles.Hint{Key: "tab", Desc: "switch pane"})
}

// emptyMessage explains an empty console.
func (m ActivityModel) emptyMessage() string {
	if m.visible().dir == "" {
		return "Select a server to see its console."
	}
	return "Nothing logged yet. Start the server from the Actions pane."
}

// title names the pane, the server it is showing and what that server is doing.
func (m ActivityModel) title() string {
	current := m.visible()
	if current.dir == "" {
		return m.Title()
	}

	title := m.Title() + " · " + current.name
	if current.status != "" {
		title += " · " + current.status
	}

	// Scrolling away from the tail stops the view following new output. That
	// is useful — you are reading something — but it looks exactly like a
	// stalled console unless it says so.
	if !current.follow {
		title += " · paused"
	}
	return title
}

// additional methods
func (m ActivityModel) Title() string {
	return "Activity"
}

func (m ActivityModel) SetLayout(l core.Layout) ActivityModel {
	m.layout = l

	// Dynamically size viewport based on window size, leaving the bottom two
	// rows for the separator and the command input.
	m.viewport.SetWidth(max(l.ContentWidth(), 1))
	m.viewport.SetHeight(max(l.ContentHeight()-2, 1))
	m.textInput.SetWidth(max(l.ContentWidth()-2, 10))

	return m.refresh()
}

func (m ActivityModel) SetFocus(focused bool) ActivityModel {
	m.isFocus = focused
	if focused {
		m.textInput.Focus()
	} else {
		m.textInput.Blur()
	}
	return m
}

// SetServer switches the panel to that server's own console, keeping the one it
// was showing exactly as it was.
func (m ActivityModel) SetServer(server *storage.Server) (ActivityModel, tea.Cmd) {
	m.rememberScroll()

	target := m.consoleFor(server)
	m.current = target.dir

	m = m.refresh()
	return m.poll()
}

// Running reports whether any followed server still has a live supervisor.
func (m ActivityModel) Running() bool {
	for _, c := range m.consoles {
		if c.dir != "" && supervisor.Running(c.dir) {
			return true
		}
	}
	return false
}
