package actions

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/commands"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// commandState is which of the panel's screens is showing.
type commandState int

const (
	commandList commandState = iota
	commandNaming
	commandWriting
	commandFilling
)

// CommandsModel is the library of saved server commands.
//
// The library is shared by every server: a command written for one is almost
// always right for the next, and keeping a copy per server would mean editing
// the same line in five places.
type CommandsModel struct {
	base

	library commands.Library
	filter  core.Filter
	state   commandState

	// name and line are what a command is being written as.
	name core.Field
	line core.Field

	// blanks is what a command with holes in it is waiting to be told.
	blanks  []string
	answers map[string]string
	answer  core.Field
	filling commands.Command
	blankAt int
}

// NewCommandsModel returns the saved-commands panel.
func NewCommandsModel() CommandsModel {
	m := CommandsModel{
		base:    newBase(),
		library: commands.Load(),
		filter:  core.NewFilter(),
		name:    core.NewField("what to call it…").WithPrompt("» ").WithLimit(40).SetWidth(30),
		line:    core.NewField("the command, with {blanks}…").WithPrompt("» ").WithLimit(300).SetWidth(40),
		answer:  core.NewField("").WithPrompt("» ").WithLimit(200).SetWidth(30),
	}

	m.base = m.setCount(len(m.library.Commands))
	return m
}

func (m CommandsModel) Init() tea.Cmd { return nil }

// Editing reports whether something is being typed.
func (m CommandsModel) Editing() bool {
	return m.filter.IsOpen() || m.name.IsOpen() || m.line.IsOpen() || m.answer.IsOpen()
}

func (m CommandsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.common(msg); handled {
		m.base = updated
		return m, cmd
	}

	switch msg := msg.(type) {
	case core.RefreshMsg:
		m.library = commands.Load()
		m.base = m.setCount(len(m.visible()))
		return m, nil

	case tea.MouseMsg:
		updated, activated := m.handleMouse(msg)
		m.base = updated

		if activated && m.state == commandList {
			return m.run()
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.key(msg)
	}

	return m, nil
}

// key handles a keypress for whichever screen is showing.
func (m CommandsModel) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case commandNaming:
		field, result, cmd := m.name.HandleKey(msg)
		m.name = field

		switch result {
		case core.FieldCommitted:
			if strings.TrimSpace(m.name.Value()) == "" {
				return m.back()
			}

			m.state = commandWriting

			opened, cmd := m.line.Open("")
			m.line = opened
			return m, cmd

		case core.FieldCancelled:
			return m.back()
		}
		return m, cmd

	case commandWriting:
		field, result, cmd := m.line.HandleKey(msg)
		m.line = field

		switch result {
		case core.FieldCommitted:
			return m.keep()
		case core.FieldCancelled:
			return m.back()
		}
		return m, cmd

	case commandFilling:
		return m.filling_(msg)
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

	case "n":
		m.state = commandNaming

		opened, cmd := m.name.Open("")
		m.name = opened
		return m, cmd

	case "d":
		return m.forget()

	case "enter":
		return m.run()

	case "backspace":
		if m.filter.Active() {
			m.filter = m.filter.Clear()
			m.base = m.setCount(len(m.visible()))
			return m, nil
		}
		return m, core.CloseAction()
	}

	return m, nil
}

// filling_ handles the answers to a command's blanks.
func (m CommandsModel) filling_(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	field, result, cmd := m.answer.HandleKey(msg)
	m.answer = field

	switch result {
	case core.FieldCommitted:
		m.answers[m.blanks[m.blankAt]] = strings.TrimSpace(m.answer.Value())
		m.blankAt++

		if m.blankAt < len(m.blanks) {
			opened, cmd := m.answer.WithValue("").Open("")
			m.answer = opened
			return m, cmd
		}

		return m.send(m.filling.Fill(m.answers))

	case core.FieldCancelled:
		return m.back()
	}

	return m, cmd
}

// run sends the chosen command, asking for its blanks first.
func (m CommandsModel) run() (tea.Model, tea.Cmd) {
	command, ok := m.chosen()
	if !ok {
		return m, nil
	}

	blanks := command.Blanks()
	if len(blanks) == 0 {
		return m.send(command.Line)
	}

	m.state = commandFilling
	m.filling, m.blanks, m.blankAt = command, blanks, 0
	m.answers = map[string]string{}

	opened, cmd := m.answer.WithValue("").Open("")
	m.answer = opened

	return m, cmd
}

// send gives the command to the server.
func (m CommandsModel) send(line string) (tea.Model, tea.Cmd) {
	dir := m.dir()

	updated, cmd := m.report("sent: "+line, false)
	m.base = updated

	return m.back2(), tea.Batch(cmd, core.SendCommand(dir, line))
}

// keep saves what has been written.
func (m CommandsModel) keep() (tea.Model, tea.Cmd) {
	command := commands.Command{
		Name: strings.TrimSpace(m.name.Value()),
		Line: strings.TrimSpace(m.line.Value()),
	}

	if command.Name == "" || command.Line == "" {
		return m.back()
	}

	m.library = m.library.Add(command)

	updated, cmd := m.report("saved "+command.Name, m.library.Save() != nil)
	m.base = updated

	next := m.back2()
	next.base = next.setCount(len(next.visible()))

	return next, cmd
}

// forget removes the chosen command.
func (m CommandsModel) forget() (tea.Model, tea.Cmd) {
	command, ok := m.chosen()
	if !ok {
		return m, nil
	}

	m.library = m.library.Remove(command.Name)

	updated, cmd := m.report("removed "+command.Name, m.library.Save() != nil)
	m.base = updated
	m.base = m.setCount(len(m.visible()))

	return m, cmd
}

// back returns to the list, abandoning whatever was being typed.
func (m CommandsModel) back() (tea.Model, tea.Cmd) { return m.back2(), nil }

// back2 is back, for the callers that have their own command to return.
func (m CommandsModel) back2() CommandsModel {
	m.state = commandList
	m.name = m.name.Cancel().WithValue("")
	m.line = m.line.Cancel().WithValue("")
	m.answer = m.answer.Cancel().WithValue("")
	m.blanks, m.answers, m.blankAt = nil, nil, 0

	return m
}

// visible is the library as the filter leaves it.
func (m CommandsModel) visible() []commands.Command {
	if !m.filter.Active() {
		return m.library.Commands
	}

	kept := make([]commands.Command, 0, len(m.library.Commands))
	for _, command := range m.library.Commands {
		if m.filter.Matches(command.Name, command.Line) {
			kept = append(kept, command)
		}
	}
	return kept
}

// chosen is the command under the cursor.
func (m CommandsModel) chosen() (commands.Command, bool) {
	visible := m.visible()
	if len(visible) == 0 || m.list.Cursor >= len(visible) {
		return commands.Command{}, false
	}
	return visible[m.list.Cursor], true
}

func (m CommandsModel) View() tea.View {
	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	switch m.state {
	case commandNaming:
		return m.panelView(m.Title()+" · New",
			lipgloss.JoinVertical(lipgloss.Left,
				muted.Render("A short name for it:"), "", m.name.View()),
			m.Hints()...)

	case commandWriting:
		return m.panelView(m.Title()+" · New",
			lipgloss.JoinVertical(lipgloss.Left,
				muted.Render("The command. Anything in {braces} is asked for when it runs:"),
				"", m.line.View(), "",
				muted.Render("for example: tell @a[distance=..10] {message}")),
			m.Hints()...)

	case commandFilling:
		return m.panelView(m.Title()+" · "+m.filling.Name,
			lipgloss.JoinVertical(lipgloss.Left,
				muted.Render(m.filling.Line), "",
				muted.Render(m.blanks[m.blankAt]+":"), m.answer.View()),
			m.Hints()...)
	}

	if len(m.visible()) == 0 {
		return m.emptyView(m.Title(), "No saved commands yet. Press n to write one.",
			styles.Hint{Key: "n", Desc: "new"}, styles.Hint{Key: "backspace", Desc: "back"})
	}

	body := m.list.Render(func(i int) string {
		visible := m.visible()
		if i >= len(visible) {
			return ""
		}

		return m.row(i, m.rowMarker(i)+visible[i].Name, visible[i].Line)
	})

	if line := m.filter.View(); line != "" {
		body = line + "\n" + body
	}

	return m.panelView(m.Title(), body, m.Hints()...)
}

// Hints are the keys this panel responds to.
func (m CommandsModel) Hints() []styles.Hint {
	switch m.state {
	case commandNaming, commandWriting, commandFilling:
		return []styles.Hint{
			{Key: "enter", Desc: "next"},
			{Key: "esc", Desc: "cancel"},
		}
	}

	return []styles.Hint{
		{Key: "enter", Desc: "run"},
		{Key: "n", Desc: "new"},
		{Key: "d", Desc: "delete"},
		{Key: "/", Desc: "filter"},
		{Key: "backspace", Desc: "back"},
	}
}

func (m CommandsModel) Title() string { return "Commands" }

func (m CommandsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(len(m.visible()))
	return m
}

func (m CommandsModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m CommandsModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.library = commands.Load()
	m.base = m.setCount(len(m.visible()))

	return m
}
