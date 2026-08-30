package actions

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/expose"
	"github.com/limelamp/osmium/internal/properties"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// networkWait is how long the router and the checks are given before the panel
// gives up on them. Nothing here blocks the interface — it all runs as
// commands — but a router that never answers should not leave a spinner
// turning for ever either.
const networkWait = 20 * time.Second

// addressesMsg carries what was found out about how to reach a server.
type addressesMsg struct {
	dir       string
	addresses []expose.Address
}

// mappedMsg carries what the router said.
type mappedMsg struct {
	mapping expose.Mapping
	closed  bool
	err     error
}

// The rows of the panel, in order.
const (
	networkCheck = iota
	networkOpen
	networkClose
	networkCount
)

var networkActions = []string{
	"Check whether anyone can reach it",
	"Ask the router to open the port",
	"Ask the router to close it again",
}

// NetworkModel is how a server is reached: the addresses that lead to it,
// whether anything answers on them, and the router's part in it.
type NetworkModel struct {
	base

	port      int
	addresses []expose.Address
	mapped    *expose.Mapping
}

// NewNetworkModel returns the panel that exposes a server.
func NewNetworkModel() NetworkModel {
	m := NetworkModel{base: newBase()}
	m.base = m.setCount(networkCount)

	return m
}

func (m NetworkModel) Init() tea.Cmd { return m.look(false) }

// Editing reports that this panel takes no typed input.
func (m NetworkModel) Editing() bool { return false }

// look works out the addresses, and optionally whether they answer.
func (m NetworkModel) look(check bool) tea.Cmd {
	dir := m.dir()
	if m.needsServer() {
		return nil
	}

	port := m.port
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), networkWait)
		defer cancel()

		addresses := expose.Addresses(ctx, port)
		if check {
			addresses = expose.Check(ctx, addresses)
		}

		return addressesMsg{dir: dir, addresses: addresses}
	}
}

func (m NetworkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.common(msg); handled {
		m.base = updated
		return m, cmd
	}

	switch msg := msg.(type) {
	case addressesMsg:
		m.busyRow, m.busyLabel = -1, ""

		if msg.dir == m.dir() {
			m.addresses = msg.addresses
		}
		return m, nil

	case mappedMsg:
		m.busyRow, m.busyLabel = -1, ""

		if msg.err != nil {
			updated, cmd := m.report(msg.err.Error(), true)
			m.base = updated
			return m, cmd
		}

		if msg.closed {
			m.mapped = nil

			updated, cmd := m.report("the router is no longer forwarding the port", false)
			m.base = updated
			return m, cmd
		}

		mapping := msg.mapping
		m.mapped = &mapping

		updated, cmd := m.report("the router is forwarding the port", false)
		m.base = updated

		return m, tea.Batch(cmd, m.look(true))

	case core.RefreshMsg:
		return m.reload(), m.look(false)

	case tea.MouseMsg:
		updated, activated := m.handleMouse(msg)
		m.base = updated

		if activated {
			return m.activate()
		}
		return m, nil

	case tea.KeyPressMsg:
		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

		switch msg.String() {
		case "enter":
			return m.activate()
		case "backspace":
			return m, core.CloseAction()
		}
	}

	return m, nil
}

// activate runs the row under the cursor.
func (m NetworkModel) activate() (tea.Model, tea.Cmd) {
	if m.needsServer() {
		return m, nil
	}

	name := "this server"
	if m.server != nil {
		name = m.server.Name
	}

	port := m.port

	switch m.list.Cursor {
	case networkCheck:
		updated, tick := m.working(networkCheck, "asking whether anyone answers")
		m.base = updated

		return m, tea.Batch(tick, m.look(true))

	case networkOpen:
		updated, tick := m.working(networkOpen, "asking the router")
		m.base = updated

		return m, tea.Batch(tick, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), networkWait)
			defer cancel()

			mapping, err := expose.Open(ctx, port, name)
			return mappedMsg{mapping: mapping, err: err}
		})

	case networkClose:
		updated, tick := m.working(networkClose, "asking the router")
		m.base = updated

		return m, tea.Batch(tick, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), networkWait)
			defer cancel()

			return mappedMsg{closed: true, err: expose.Close(ctx, port)}
		})
	}

	return m, nil
}

// reload rereads the port the server is set to listen on.
func (m NetworkModel) reload() NetworkModel {
	m.port = 25565
	if props, err := properties.Read(m.dir()); err == nil {
		m.port = props.Port()
	}

	m.base = m.setCount(networkCount)
	return m
}

func (m NetworkModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	rows := []string{muted.Render("WAYS IN")}

	if len(m.addresses) == 0 {
		rows = append(rows, muted.Render("  looking…"))
	}

	for _, address := range m.addresses {
		line := fmt.Sprintf("  %-14s %s", address.Where.Name(), address.String())
		if address.Host == "" {
			line = fmt.Sprintf("  %-14s %s", address.Where.Name(), muted.Render("unknown"))
		}

		switch {
		case address.Checked && address.Answers:
			line += "  " + lipgloss.NewStyle().Foreground(theme.Accent).Render("✓ answers")
		case address.Checked:
			line += "  " + lipgloss.NewStyle().Foreground(styles.ErrorColor).Render("✕ no answer")
		}

		rows = append(rows, lipgloss.NewStyle().MaxWidth(max(m.layout.ContentWidth(), 1)).Render(line))

		if address.Note != "" {
			rows = append(rows, muted.Render("    "+address.Note))
		}
	}

	if m.mapped != nil {
		rows = append(rows, "", muted.Render("THE ROUTER"),
			fmt.Sprintf("  forwarding %d until %s", m.mapped.Port, m.mapped.Until.Format("15:04")))
	}

	rows = append(rows,
		"",
		muted.Render("DO"),
		m.list.Render(func(i int) string {
			return m.row(i, m.rowMarker(i)+networkActions[i], "")
		}),
		"",
		muted.Render("opening a port changes the router, not this machine, and can be closed again"),
	)

	return m.panelView(m.Title(), lipgloss.JoinVertical(lipgloss.Left, rows...), m.Hints()...)
}

// Hints are the keys this panel responds to.
func (m NetworkModel) Hints() []styles.Hint {
	return []styles.Hint{
		{Key: "↑/↓", Desc: "move"},
		{Key: "enter", Desc: "do it"},
		{Key: "backspace", Desc: "back"},
	}
}

func (m NetworkModel) Title() string { return "Expose Server" }

func (m NetworkModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(networkCount)
	return m
}

func (m NetworkModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m NetworkModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.addresses, m.mapped = nil, nil

	return m.reload()
}
