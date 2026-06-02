package components

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/tui/config"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/util"
)

// RunServer Model
type ActivityModel struct {
	layout  core.Layout
	isFocus bool

	cursor        int
	textInput     textinput.Model
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

	return ActivityModel{
		cursor:    0,
		textInput: ti,
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

		javaPath, args := util.GetServerRunCommand(osmiumConf.Loader)

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
	}

	switch msg := msg.(type) {
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
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
		// case "backspace":
		// 	m.GoBack = true
		// 	return m, nil
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

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
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

	// Get everything in the bucket
	allLogs := m.output.String()

	// Split into lines
	lines := strings.Split(allLogs, "\n")

	for i := 0; i < len(lines); i++ {
		content += lines[i] + "\n"
	}

	content += "> " + m.textInput.View()
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
	return m
}

func (m ActivityModel) SetFocus(focused bool) ActivityModel {
	m.isFocus = focused
	return m
}
