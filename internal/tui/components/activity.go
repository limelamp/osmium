package components

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/util"
)

// Used to poll the buffer for new log lines automatically
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// RunServer Model
type ActivityModel struct {
	layout  core.Layout
	isFocus bool

	cursor        int
	textInput     textinput.Model
	viewport      viewport.Model
	lastLogLength int
	firstRun      bool
	javaCMD       *exec.Cmd
	output        *bytes.Buffer // The "bucket" for logs
	inputPipe     io.WriteCloser
	statusMessage string
	GoBack        bool
	err           error
}

func NewActivityModel() ActivityModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter a command..."
	ti.Focus()     // Start with the cursor blinking inside it
	ti.Prompt = "" // Remove the ">" out of the way
	ti.CharLimit = 500
	ti.SetWidth(500)

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
		cursor:    0,
		textInput: ti,
		viewport:  vp,
		firstRun:  true,
		output:    &bytes.Buffer{},
		GoBack:    false,
	}
}

// RunServer State
func (m ActivityModel) Init() tea.Cmd {
	return nil
}

func (m ActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.firstRun {

		osmiumConf, err := config.ReadConfig()
		if err != nil {
			m.err = err
			return m, nil
		}

		if pid, err := shared.ReadLockPID(); err == nil {
			if shared.IsPIDRunning(pid) {
				m.err = fmt.Errorf("server already running (pid %d). stop it with 'osmium stop'", pid)
				m.firstRun = false
				return m, nil
			}

			if err := shared.RemoveLockFile(); err != nil {
				m.err = err
				return m, nil
			}
		}

		javaPath, args := util.GetServerRunCommand(osmiumConf.Loader, osmiumConf.AllocatedRAM)
		// 										   feed the appopriate loader and ram into the func

		m.javaCMD = exec.Command(javaPath, args...)

		m.javaCMD.Dir, _ = os.Getwd()

		// Point both outputs to our buffer
		m.javaCMD.Stdout = m.output
		m.javaCMD.Stderr = m.output

		m.inputPipe, err = m.javaCMD.StdinPipe() // This is the "entrance"
		if err != nil {
			m.err = err
			return m, nil
		}

		if err := m.javaCMD.Start(); err != nil {
			m.err = err
			return m, nil
		}

		if err := shared.WriteLockPID(m.javaCMD.Process.Pid); err != nil {
			m.err = err
			_ = m.javaCMD.Process.Kill()
			return m, nil
		}

		// Start the socket listener in the background
		go shared.StartBasicSocketServer(m.inputPipe)

		go func(cmd *exec.Cmd) {
			_ = cmd.Wait()
			_ = shared.RemoveLockFile()
		}(m.javaCMD)

		m.firstRun = false
		cmds = append(cmds, tickCmd())
	}

	switch msg := msg.(type) {
	case tickMsg:
		newLogs := m.output.String()         // Get java's logs
		if len(newLogs) != m.lastLogLength { // Check in order to not waste cpu usage by redrawing constantly
			isAtBottom := m.viewport.AtBottom()

			// Using Lipgloss to force the text to wrap exactly at the edge of the viewport
			wrappedLogs := lipgloss.NewStyle().Width(m.viewport.Width()).Render(newLogs)

			// Give the wrapped version to the viewport instead of the raw logs
			m.viewport.SetContent(wrappedLogs)
			// m.viewport.SetContent(newLogs) // Set the viewport's text to that of the java logs (no text wrapping, pure)

			if isAtBottom || m.lastLogLength == 0 {
				m.viewport.GotoBottom()
				// Basically, if the 'cursor' was at the bottom this forces it to stay at the bottom after lines get added
			}
			m.lastLogLength = len(newLogs)
		}
		cmds = append(cmds, tickCmd())

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// Make sure to kill the java process if ctrl-c is used.
			if m.javaCMD != nil && m.javaCMD.Process != nil {
				_ = m.javaCMD.Process.Kill()
			}

			// Remove the .lock file once the process is killed.
			if err := shared.RemoveLockFile(); err != nil {
				m.err = err
			}

			return m, tea.Quit

		case "enter":
			// 1. Get the command from the input
			command := m.textInput.Value()

			if m.inputPipe != nil && command != "" {
				// 2. Write it to the server with a newline
				fmt.Fprintln(m.inputPipe, command)
			}

			// 3. Reset the text input for the next command
			m.textInput.Reset()
		default:

		}
	}

	var tiCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	cmds = append(cmds, tiCmd)

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// RunServer View
func (m ActivityModel) View() tea.View {

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#eaff00ff")).
		Background(lipgloss.Color("#000000ff"))

	// content := headerStyle.Render(" OSMIUM - RUNNING SERVER ") + " Ctrl-C to Exit" + "\n\n"
	content := ""

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		content += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	// Replaced the old manual line splitting loop with the new shiny viewport
	content += m.viewport.View() + "\n"

	content += "> " + m.textInput.View() + "\n"
	content += statusStyle.Render(m.statusMessage)
	// s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'backspace' to go back.\n\n"

	return tea.NewView(styles.Container(
		m.layout.Width,
		m.layout.Height,
		m.isFocus,
		m.Title(),
		content,
		false,
	))
}

// additional methods
func (m ActivityModel) Title() string {
	return "Activity"
}

func (m ActivityModel) SetLayout(l core.Layout) ActivityModel {
	m.layout = l

	// Dynamically size viewport based on window size
	m.viewport.SetWidth(l.Width - 4)
	m.viewport.SetHeight(l.Height - 6)

	return m
}

func (m ActivityModel) SetFocus(focused bool) ActivityModel {
	m.isFocus = focused
	return m
}
