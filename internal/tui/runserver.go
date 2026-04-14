package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/util"
)

// RunServer Model
type RunServerModel struct {
	cursor        int
	options       []string
	textInput     textinput.Model
	firstRun      bool
	javaCMD       *exec.Cmd
	output        *bytes.Buffer // The "bucket" for logs
	inputPipe     io.WriteCloser
	statusMessage string
	GoBack        bool
	err           error
}

func NewRunServerModel() RunServerModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter a command..."
	ti.Focus()     // Start with the cursor blinking inside it
	ti.Prompt = "" // Remove the ">" out of the way
	ti.CharLimit = 500
	ti.Width = 500

	return RunServerModel{
		cursor:    0,
		options:   []string{"Recommended settings", "Detailed"},
		textInput: ti,
		firstRun:  true,
		output:    &bytes.Buffer{},
		GoBack:    false,
	}
}

// RunServer State
func (m RunServerModel) Init() tea.Cmd {
	return nil
}

func (m RunServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.firstRun {
		if len(m.options) == 0 {
			m.options = []string{""}
		}

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
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
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
func (m RunServerModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#d56509ff")).
		Padding(0, 1)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#eaff00ff")).
		Background(lipgloss.Color("#000000ff"))
		// Background(lipgloss.Color("#eaff00ff")).
		// Foreground(lipgloss.Color("#000000ff"))

	s := headerStyle.Render(" OSMIUM - RUNNING SERVER ") + " Ctrl-C to Exit" + "\n\n"

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		s += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	// Get everything in the bucket
	allLogs := m.output.String()

	// Split into lines
	lines := strings.Split(allLogs, "\n")

	for i := 0; i < len(lines); i++ {
		s += lines[i] + "\n"
	}

	s += "> " + m.textInput.View()
	s += statusStyle.Render(m.statusMessage)
	// s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'backspace' to go back.\n\n"
	return s
}
