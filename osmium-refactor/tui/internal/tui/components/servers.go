package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/core"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/storage"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/styles"
)

type ServersModel struct {
	layout core.Layout
	store  *storage.ServerStore

	servers []storage.Server
	value   int
	count   int
	isFocus bool
	err     error
}

// NewServersModel now properly initializes the store field
func NewServersModel(store *storage.ServerStore) ServersModel {
	return ServersModel{
		store: store,
	}
}

func (m ServersModel) Init() tea.Cmd {
	return core.LoadServersCmd(m.store)
}

func (m ServersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "e":
			m.value++

		case "t":
			m.value++
		}

	case core.LoadedServersMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.servers = msg.Servers
		return m, nil

	case core.SavedServersMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, nil
	}

	return m, nil
}

func (m ServersModel) View() tea.View {
	// 1. If an error occurred while loading/saving, display it
	if m.err != nil {
		return tea.NewView(styles.Container(
			m.layout.Width,
			m.layout.Height,
			m.isFocus,
			m.Title(),
			fmt.Sprintf("Error loading servers: %v", m.err),
			false,
		))
	}

	// 2. Build the server list representation
	var content string
	if len(m.servers) == 0 {
		content = "No servers found. Add one to get started."
	} else {
		var serversList strings.Builder
		for _, server := range m.servers {
			// Formats each server line with bullet points and basic information
			fmt.Fprintf(&serversList, "• %s [%s %s] (%s)\n",
				server.Name,
				strings.ToUpper(server.Type),
				server.Version,
				server.Memory)
		}
		content = serversList.String()
	}

	// 3. Render the list inside the layout container style
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
func (m ServersModel) Title() string {
	return "Servers"
}

func (m ServersModel) SetLayout(l core.Layout) ServersModel {
	m.layout = l
	return m
}

func (m ServersModel) SetFocus(focused bool) ServersModel {
	m.isFocus = focused
	return m
}
