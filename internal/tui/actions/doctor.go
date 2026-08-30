package actions

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/health"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// checkedMsg carries what a look at a server found.
type checkedMsg struct {
	dir      string
	findings []health.Finding
}

// DoctorModel looks a server over for the things that stop it starting: the
// wrong Java, memory it has not got, a port in use, an unaccepted licence,
// files the lock names that are not there.
type DoctorModel struct {
	base

	findings []health.Finding
}

// NewDoctorModel returns the panel that checks a server.
func NewDoctorModel() DoctorModel {
	return DoctorModel{base: newBase()}
}

func (m DoctorModel) Init() tea.Cmd { return m.check() }

// Editing reports that this panel takes no typed input.
func (m DoctorModel) Editing() bool { return false }

// check looks the server over in the background: it runs java and dials a
// port, neither of which belongs on the thread that draws.
func (m DoctorModel) check() tea.Cmd {
	dir := m.dir()
	if m.needsServer() {
		return nil
	}

	return func() tea.Msg { return checkedMsg{dir: dir, findings: health.Check(dir)} }
}

func (m DoctorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.common(msg); handled {
		m.base = updated
		return m, cmd
	}

	switch msg := msg.(type) {
	case checkedMsg:
		m.busyRow, m.busyLabel = -1, ""

		if msg.dir == m.dir() {
			m.findings = msg.findings
			m.base = m.setCount(len(m.findings))
		}
		return m, nil

	case core.RefreshMsg:
		return m, m.check()

	case tea.KeyPressMsg:
		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

		switch msg.String() {
		case "r", "enter":
			updated, tick := m.working(0, "looking")
			m.base = updated

			return m, tea.Batch(tick, m.check())

		case "backspace":
			return m, core.CloseAction()
		}
	}

	return m, nil
}

func (m DoctorModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	if len(m.findings) == 0 {
		return m.emptyView(m.Title(), "Looking…", styles.Hint{Key: "backspace", Desc: "back"})
	}

	rows := make([]string, 0, len(m.findings)+2)
	for _, finding := range m.findings {
		colour := theme.Accent
		switch finding.Level {
		case health.Problem:
			colour = styles.ErrorColor
		case health.Note:
			colour = theme.Primary
		}

		rows = append(rows, lipgloss.NewStyle().MaxWidth(max(m.layout.ContentWidth(), 1)).Render(
			lipgloss.NewStyle().Foreground(colour).Render(finding.Level.Mark())+" "+
				lipgloss.NewStyle().Foreground(theme.Primary).Width(20).Render(finding.Title)+
				lipgloss.NewStyle().Foreground(theme.Inactive).Render(finding.Detail)))
	}

	verdict := "Nothing here would stop the server starting."
	if health.Worst(m.findings) == health.Problem {
		verdict = "The marked problems will stop this server working."
	}

	rows = append(rows, "", lipgloss.NewStyle().Foreground(theme.Inactive).Render(verdict))

	return m.panelView(m.Title(), lipgloss.JoinVertical(lipgloss.Left, rows...), m.Hints()...)
}

// Hints are the keys this panel responds to.
func (m DoctorModel) Hints() []styles.Hint {
	return []styles.Hint{
		{Key: "r", Desc: "look again"},
		{Key: "backspace", Desc: "back"},
	}
}

func (m DoctorModel) Title() string { return "Check Server" }

func (m DoctorModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(len(m.findings))
	return m
}

func (m DoctorModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m DoctorModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.findings = nil

	return m
}
