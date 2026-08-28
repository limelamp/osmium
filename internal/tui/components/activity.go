package components

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
	"github.com/limelamp/osmium/internal/util"
)

// maxLogLines bounds how much history one server's console keeps.
const maxLogLines = 5000

// Used to poll the buffers for new log lines automatically
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// serverProcess is a running Minecraft server. It lives behind a pointer
// because the model is copied on every update while the process, its stdin pipe
// and its output buffer must not be.
type serverProcess struct {
	mu     sync.Mutex
	buffer bytes.Buffer

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	dir     string
	running bool
}

// Write collects the process output. The process writes from its own goroutine,
// so every touch of the buffer is behind the mutex.
func (p *serverProcess) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buffer.Write(b)
}

// drain takes everything written since the last call.
func (p *serverProcess) drain() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := p.buffer.String()
	p.buffer.Reset()
	return out
}

// isRunning reports whether the process is still alive.
func (p *serverProcess) isRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// finish records that the process has exited.
func (p *serverProcess) finish() {
	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
}

// send writes one console command to the server's stdin.
func (p *serverProcess) send(command string) error {
	if p.stdin == nil {
		return fmt.Errorf("server is not running")
	}
	_, err := fmt.Fprintln(p.stdin, command)
	return err
}

// interrupt asks the process to shut down the way "osmium stop" does, letting
// the server save its world instead of losing it to a kill.
func (p *serverProcess) interrupt() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}

	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		_ = p.cmd.Process.Kill()
	}
	_ = shared.RemoveLockFileIn(p.dir)
}

// console is everything the Activity panel keeps for one server: its output
// history, the process producing it, and where the user had scrolled to.
//
// One console per server is what keeps two servers from sharing a state: their
// logs stay apart, starting one does not report the other as already running,
// and switching selection returns to exactly the view you left.
type console struct {
	name    string
	lines   []string
	process *serverProcess
	status  string

	// offset and follow remember this console's scroll position across
	// selection changes.
	offset int
	follow bool
}

// append adds lines to a console, trimming the oldest once it is full.
func (c *console) append(lines ...string) {
	c.lines = append(c.lines, lines...)
	if len(c.lines) > maxLogLines {
		c.lines = c.lines[len(c.lines)-maxLogLines:]
	}
}

// isRunning reports whether this console owns a live process.
func (c *console) isRunning() bool {
	return c.process != nil && c.process.isRunning()
}

// ActivityModel is the console of the Manage Servers page. It shows the
// selected server's output and application progress, and sends typed commands
// to that server's process.
type ActivityModel struct {
	layout  core.Layout
	isFocus bool

	textInput textinput.Model
	viewport  viewport.Model

	// consoles is keyed by server path; current names the visible one.
	consoles map[string]*console
	current  string

	wrapped int // rendered line count, for the scrollbar
}

func NewActivityModel() ActivityModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter a command..."
	ti.Prompt = "› "
	ti.CharLimit = 500

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

	return ActivityModel{
		textInput: ti,
		viewport:  vp,
		consoles:  make(map[string]*console),
	}
}

func (m ActivityModel) Init() tea.Cmd {
	return nil
}

// consoleFor returns the console for a server, creating it on first use.
func (m ActivityModel) consoleFor(server *storage.Server) *console {
	if server == nil || server.Path == "" {
		return nil
	}

	if existing, ok := m.consoles[server.Path]; ok {
		existing.name = server.Name
		return existing
	}

	created := &console{name: server.Name, follow: true}
	m.consoles[server.Path] = created
	return created
}

// visible returns the console currently on screen, if any.
func (m ActivityModel) visible() *console {
	return m.consoles[m.current]
}

func (m ActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.ServerControlMsg:
		return m.control(msg)

	case core.LogMsg:
		return m.appendVisible(msg.Lines...), nil

	case core.ShutdownMsg:
		// Leaving the TUI must not orphan a server, so every one that is still
		// running is asked to shut down gracefully on the way out.
		for _, c := range m.consoles {
			if c.isRunning() {
				c.process.interrupt()
			}
		}
		return m, nil

	case tickMsg:
		return m.drainAll()

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

// drainAll moves any new process output into its own console, so a server that
// is not currently on screen still collects its logs.
func (m ActivityModel) drainAll() (tea.Model, tea.Cmd) {
	anyRunning := false

	for _, c := range m.consoles {
		if c.process == nil {
			continue
		}

		if output := c.process.drain(); output != "" {
			c.append(strings.Split(strings.TrimRight(output, "\n"), "\n")...)
		}

		if c.process.isRunning() {
			anyRunning = true
			continue
		}

		c.append("— server process exited —")
		c.process = nil
		c.status = "stopped"
	}

	m = m.refresh()
	if !anyRunning {
		return m, nil
	}
	return m, tickCmd()
}

// submit sends the typed line to the visible server's process.
func (m ActivityModel) submit() (tea.Model, tea.Cmd) {
	command := m.textInput.Value()
	if command == "" {
		return m, nil
	}
	m.textInput.Reset()

	current := m.visible()
	switch {
	case current == nil:
		return m, nil
	case !current.isRunning():
		return m.appendVisible("no server running — start one from the Actions pane"), nil
	}

	if err := current.process.send(command); err != nil {
		return m.appendVisible("error: " + err.Error()), nil
	}
	return m.appendVisible("› " + command), nil
}

// control starts, stops or restarts the server the request names, which is not
// necessarily the one on screen.
func (m ActivityModel) control(msg core.ServerControlMsg) (tea.Model, tea.Cmd) {
	target := m.consoleFor(msg.Server)
	if target == nil {
		return m.appendVisible("select a server first"), nil
	}

	switch msg.Control {
	case core.ServerStop:
		return m.stop(target), nil

	case core.ServerRestart:
		m = m.stop(target)
		return m.start(msg.Server, target)

	default:
		return m.start(msg.Server, target)
	}
}

// start launches the process for one server into its own console.
func (m ActivityModel) start(server *storage.Server, target *console) (tea.Model, tea.Cmd) {
	dir := server.Path

	if target.isRunning() {
		return m.appendTo(target, fmt.Sprintf("%s is already running", server.Name)), nil
	}

	osmiumConf, err := config.ReadConfigIn(dir)
	if err != nil {
		return m.appendTo(target, "error: could not read osmium.json in "+dir), nil
	}

	// A live lock file means somebody else owns this server already.
	if pid, err := shared.ReadLockPIDIn(dir); err == nil {
		if shared.IsPIDRunning(pid) {
			return m.appendTo(target, fmt.Sprintf(
				"%s is already running (pid %d); stop it with 'osmium stop'", server.Name, pid)), nil
		}
		if err := shared.RemoveLockFileIn(dir); err != nil {
			return m.appendTo(target, "error: "+err.Error()), nil
		}
	}

	javaPath, args := util.GetServerRunCommand(dir, osmiumConf.Loader, osmiumConf.AllocatedRAM)
	//                                          feed the appropriate loader and ram into the func

	process := &serverProcess{dir: dir, running: true}
	process.cmd = exec.Command(javaPath, args...)
	process.cmd.Dir = dir

	// Point both outputs to this server's own buffer
	process.cmd.Stdout = process
	process.cmd.Stderr = process

	process.stdin, err = process.cmd.StdinPipe() // This is the "entrance"
	if err != nil {
		return m.appendTo(target, "error: "+err.Error()), nil
	}

	if err := process.cmd.Start(); err != nil {
		return m.appendTo(target, "error: "+err.Error()), nil
	}

	if err := shared.WriteLockPIDIn(dir, process.cmd.Process.Pid); err != nil {
		_ = process.cmd.Process.Kill()
		return m.appendTo(target, "error: "+err.Error()), nil
	}

	// // Start the socket listener in the background
	// go shared.StartBasicSocketServer(process.stdin)

	go func(p *serverProcess) {
		_ = p.cmd.Wait()
		p.finish()
		_ = shared.RemoveLockFileIn(p.dir)
	}(process)

	target.process = process
	target.status = "running"

	return m.appendTo(target, fmt.Sprintf("— starting %s (%s %s) —",
		server.Name, osmiumConf.Loader, osmiumConf.Version)), tickCmd()
}

// stop terminates one server's process.
func (m ActivityModel) stop(target *console) ActivityModel {
	if !target.isRunning() {
		return m.appendTo(target, fmt.Sprintf("%s is not running", target.name))
	}

	// "stop" lets the server save and shut down cleanly; killing it is the
	// fallback when it will not take the command.
	if err := target.process.send("stop"); err != nil {
		if target.process.cmd != nil && target.process.cmd.Process != nil {
			_ = target.process.cmd.Process.Kill()
		}
	}

	target.status = "stopping"
	return m.appendTo(target, "— stopping "+target.name+" —")
}

// appendVisible writes lines into the console currently on screen.
func (m ActivityModel) appendVisible(lines ...string) ActivityModel {
	return m.appendTo(m.visible(), lines...)
}

// appendTo writes lines into one console, re-rendering only if it is the one
// being shown.
func (m ActivityModel) appendTo(target *console, lines ...string) ActivityModel {
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

	text := ""
	if current != nil {
		text = strings.Join(current.lines, "\n")
	}

	// Using Lipgloss to force the text to wrap exactly at the edge of the viewport
	wrapped := lipgloss.NewStyle().
		Width(max(m.viewport.Width(), 1)).
		Render(text)

	m.viewport.SetContent(wrapped)
	m.wrapped = lipgloss.Height(wrapped)

	switch {
	case current == nil:
		m.viewport.SetYOffset(0)
	case current.follow:
		m.viewport.GotoBottom()
		current.offset = m.viewport.YOffset()
	default:
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
	if m.visible() == nil {
		logs = styles.Body(lipgloss.NewStyle().Foreground(theme.Inactive).
			Render("Select a server to see its console."), m.viewport.Height())
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

// title names the pane, the server it is showing and what that server is doing.
func (m ActivityModel) title() string {
	current := m.visible()
	if current == nil {
		return m.Title()
	}

	title := m.Title() + " · " + current.name
	if current.status != "" {
		title += " · " + current.status
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
func (m ActivityModel) SetServer(server *storage.Server) ActivityModel {
	m.rememberScroll()

	if server == nil || server.Path == "" {
		m.current = ""
		return m.refresh()
	}

	m.consoleFor(server)
	m.current = server.Path
	return m.refresh()
}

// Running reports whether any server still has a live process, which is what
// keeps the poll ticking after the page is rebuilt.
func (m ActivityModel) Running() bool {
	for _, c := range m.consoles {
		if c.isRunning() {
			return true
		}
	}
	return false
}
