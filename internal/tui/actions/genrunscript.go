package actions

import (
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// GenRunScriptModel writes a start script into the selected server's folder.
type GenRunScriptModel struct {
	base

	options []string
}

func NewGenRunScriptModel() GenRunScriptModel {
	m := GenRunScriptModel{
		base:    newBase(),
		options: []string{"Recommended settings", "Detailed"},
	}
	m.base = m.setCount(len(m.options))
	return m
}

func (m GenRunScriptModel) Init() tea.Cmd {
	return nil
}

func (m GenRunScriptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		updated, activated := m.handleMouse(msg)
		m.base = updated
		if activated {
			return m.generate()
		}
		return m, nil

	case tea.KeyPressMsg:
		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

		switch msg.String() {
		case "backspace":
			return m, core.CloseAction()

		case "enter":
			return m.generate()
		}
	}

	return m, nil
}

// generate writes the run script for the cursor's option.
func (m GenRunScriptModel) generate() (tea.Model, tea.Cmd) {
	if m.needsServer() {
		return m, nil
	}

	ram := "4G"
	if conf, err := config.ReadConfigIn(m.dir()); err == nil && conf.AllocatedRAM != "" {
		ram = conf.AllocatedRAM
	}

	command := "java -jar -Xms" + ram + " -Xmx" + ram + " server.jar nogui"

	var content []byte
	var outputFile string
	switch runtime.GOOS { // Create different files and contents for different OS
	case "windows":
		content = []byte(command)
		outputFile = "run_server.bat"
	default:
		content = []byte("#!/bin/bash\n\n" + command)
		outputFile = "run_server.sh"
	}

	path := m.scope(nil).Path(outputFile)
	if err := os.WriteFile(path, content, 0755); err != nil {
		m.err = err
		return m, nil
	}

	m.err = nil
	return m, core.Log("Created " + path)
}

func (m GenRunScriptModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	return m.listView(m.Title(), func(i int) string {
		return m.row(i, m.options[i], "")
	}, m.Hints()...)
}

// Hints are the keys this panel responds to.
func (m GenRunScriptModel) Hints() []styles.Hint {
	return []styles.Hint{
		{Key: "↑/↓", Desc: "move"},
		{Key: "enter", Desc: "generate"},
		{Key: "backspace", Desc: "back"},
	}
}

// additional methods
func (m GenRunScriptModel) Title() string {
	return "Generate Run Script"
}

func (m GenRunScriptModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(len(m.options))
	return m
}

func (m GenRunScriptModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m GenRunScriptModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	return m
}
