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

// maxLogLines bounds how much history the console keeps.
const maxLogLines = 5000

// Used to poll the buffer for new log lines automatically
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// serverProcess is the Minecraft server the Activity panel is attached to. It
// lives behind a pointer because the model is copied on every update while the
// process, its stdin pipe and its output buffer must not be.
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

// send writes one console command to the server's stdin.
func (p *serverProcess) send(command string) error {
	if p.stdin == nil {
		return fmt.Errorf("server is not running")
	}
	_, err := fmt.Fprintln(p.stdin, command)
	return err
}

// ActivityModel is the console of the Manage Servers page: it shows the
// selected server's output and application progress, and sends typed commands
// to the running process.
type ActivityModel struct {
	layout  core.Layout
	isFocus bool
	server  *storage.Server

	textInput textinput.Model
	viewport  viewport.Model

	lines     []string
	wrapped   int // rendered line count, for the scrollbar
	process   *serverProcess
	statusMsg string
	err       error
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
	}
}

func (m ActivityModel) Init() tea.Cmd {
	return nil
}

func (m ActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case core.ServerControlMsg:
		return m.control(msg)

	case core.LogMsg:
		return m.append(msg.Lines...), nil

	case core.ShutdownMsg:
		// Leaving the TUI must not orphan a server, so it is asked to shut
		// down gracefully on the way out.
		if m.process != nil && m.process.isRunning() {
			m.process.interrupt()
		}
		return m, nil

	case tickMsg:
		if m.process == nil {
			return m, nil
		}

		if output := m.process.drain(); output != "" {
			m = m.append(strings.Split(strings.TrimRight(output, "\n"), "\n")...)
		}

		if !m.process.isRunning() {
			m.statusMsg = "server stopped"
			m.process = nil
			return m.append("— server process exited —"), nil
		}
		return m, tickCmd()

	case tea.MouseMsg:
		mouse := msg.Mouse()
		if wheel, ok := msg.(tea.MouseWheelMsg); ok && m.layout.Hit(mouse.X, mouse.Y) {
			switch wheel.Button {
			case tea.MouseWheelUp:
				m.viewport.ScrollUp(core.WheelStep)
			case tea.MouseWheelDown:
				m.viewport.ScrollDown(core.WheelStep)
			}
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

		case "pgup", "pgdown":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// submit sends the typed line to the running server.
func (m ActivityModel) submit() (tea.Model, tea.Cmd) {
	command := m.textInput.Value()
	if command == "" {
		return m, nil
	}
	m.textInput.Reset()

	if m.process == nil {
		return m.append("no server running — start one from the Actions pane"), nil
	}

	if err := m.process.send(command); err != nil {
		return m.append("error: " + err.Error()), nil
	}
	return m.append("› " + command), nil
}

// control starts, stops or restarts the selected server.
func (m ActivityModel) control(msg core.ServerControlMsg) (tea.Model, tea.Cmd) {
	switch msg.Control {
	case core.ServerStop:
		return m.stop(), nil

	case core.ServerRestart:
		m = m.stop()
		return m.start(msg.Server)

	default:
		return m.start(msg.Server)
	}
}

// start launches the server process for the given server.
func (m ActivityModel) start(server *storage.Server) (tea.Model, tea.Cmd) {
	if server == nil || server.Path == "" {
		return m.append("select a server before starting it"), nil
	}
	if m.process != nil && m.process.isRunning() {
		return m.append("server is already running"), nil
	}

	dir := server.Path

	osmiumConf, err := config.ReadConfigIn(dir)
	if err != nil {
		return m.append("error: could not read osmium.json in " + dir), nil
	}

	// A live lock file means somebody else owns this server already.
	if pid, err := shared.ReadLockPIDIn(dir); err == nil {
		if shared.IsPIDRunning(pid) {
			return m.append(fmt.Sprintf("server already running (pid %d); stop it with 'osmium stop'", pid)), nil
		}
		if err := shared.RemoveLockFileIn(dir); err != nil {
			return m.append("error: " + err.Error()), nil
		}
	}

	javaPath, args := util.GetServerRunCommand(dir, osmiumConf.Loader, osmiumConf.AllocatedRAM)
	//                                          feed the appropriate loader and ram into the func

	process := &serverProcess{dir: dir, running: true}
	process.cmd = exec.Command(javaPath, args...)
	process.cmd.Dir = dir

	// Point both outputs to our buffer
	process.cmd.Stdout = process
	process.cmd.Stderr = process

	process.stdin, err = process.cmd.StdinPipe() // This is the "entrance"
	if err != nil {
		return m.append("error: " + err.Error()), nil
	}

	if err := process.cmd.Start(); err != nil {
		return m.append("error: " + err.Error()), nil
	}

	if err := shared.WriteLockPIDIn(dir, process.cmd.Process.Pid); err != nil {
		_ = process.cmd.Process.Kill()
		return m.append("error: " + err.Error()), nil
	}

	// // Start the socket listener in the background
	// go shared.StartBasicSocketServer(process.stdin)

	go func(p *serverProcess) {
		_ = p.cmd.Wait()
		p.finish()
		_ = shared.RemoveLockFileIn(p.dir)
	}(process)

	m.process = process
	m.statusMsg = "running"

	return m.append(fmt.Sprintf("— starting %s (%s %s) —", server.Name, osmiumConf.Loader, osmiumConf.Version)), tickCmd()
}

// stop terminates the running server process.
func (m ActivityModel) stop() ActivityModel {
	if m.process == nil || !m.process.isRunning() {
		return m.append("no server is running")
	}

	// "stop" lets the server save and shut down cleanly; killing it is the
	// fallback when it will not take the command.
	if err := m.process.send("stop"); err != nil {
		if m.process.cmd != nil && m.process.cmd.Process != nil {
			_ = m.process.cmd.Process.Kill()
		}
	}

	m.statusMsg = "stopping"
	return m.append("— stopping server —")
}

// append adds lines to the console and follows the tail when already at bottom.
func (m ActivityModel) append(lines ...string) ActivityModel {
	if len(lines) == 0 {
		return m
	}

	atBottom := m.viewport.AtBottom()

	m.lines = append(m.lines, lines...)
	if len(m.lines) > maxLogLines {
		m.lines = m.lines[len(m.lines)-maxLogLines:]
	}

	m = m.refresh()
	if atBottom {
		m.viewport.GotoBottom()
	}
	return m
}

// refresh re-wraps the log to the viewport width.
func (m ActivityModel) refresh() ActivityModel {
	// Using Lipgloss to force the text to wrap exactly at the edge of the viewport
	wrapped := lipgloss.NewStyle().
		Width(max(m.viewport.Width(), 1)).
		Render(strings.Join(m.lines, "\n"))

	m.viewport.SetContent(wrapped)
	m.wrapped = lipgloss.Height(wrapped)
	return m
}

// RunServer View
func (m ActivityModel) View() tea.View {
	separator := lipgloss.NewStyle().
		Foreground(theme.Inactive).
		Render(strings.Repeat("─", max(m.layout.ContentWidth(), 0)))

	logs := m.viewport.View()
	if m.err != nil {
		logs = styles.Error(m.err) + "\n" + logs
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

// title shows the pane name plus what the process is currently doing.
func (m ActivityModel) title() string {
	if m.statusMsg == "" {
		return m.Title()
	}
	return m.Title() + " · " + m.statusMsg
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

func (m ActivityModel) SetServer(server *storage.Server) ActivityModel {
	m.server = server
	return m
}
