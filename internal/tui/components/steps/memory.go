package steps

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/constants"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// MemoryStep allocates the JVM heap the server runs with.
type MemoryStep struct {
	config *Config
	layout core.Layout

	list core.List
}

func NewMemoryStep(config *Config) *MemoryStep {
	s := &MemoryStep{config: config}
	s.list = s.list.Resize(len(constants.RamOptions), len(constants.RamOptions))

	// Start on whatever the configuration already carries, so the wizard's
	// default from Settings is the highlighted row.
	for i, option := range constants.RamOptions {
		if option == config.RAM {
			s.list.Cursor = i
			break
		}
	}
	return s
}

func (s *MemoryStep) Init() tea.Cmd { return nil }

func (s *MemoryStep) Title() string { return "Allocate Memory" }

func (s *MemoryStep) Subtitle() string {
	return "How much RAM the server may use. 4G suits most small servers."
}

func (s *MemoryStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		list, activated := s.list.HandleMouse(msg, s.layout, 0)
		s.list = list
		if activated {
			return s, s.confirm()
		}
		return s, nil

	case tea.KeyPressMsg:
		if list, handled := s.list.HandleKey(msg.String()); handled {
			s.list = list
			return s, nil
		}

		if msg.String() == "enter" || msg.String() == " " {
			return s, s.confirm()
		}
	}

	return s, nil
}

// confirm records the highlighted allocation.
func (s *MemoryStep) confirm() tea.Cmd {
	s.config.RAM = constants.RamOptions[s.list.Cursor]
	return Next()
}

func (s *MemoryStep) View() tea.View {
	start, end := s.list.Window()

	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rows = append(rows, styles.ListRow(s.layout.Width, i == s.list.Cursor,
			constants.RamOptions[i], ""))
	}

	note := lipgloss.NewStyle().Foreground(theme.Inactive).
		Render("Both -Xms and -Xmx are set to this value.")

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinVertical(lipgloss.Left, rows...), "", note))
}

func (s *MemoryStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.list = s.list.Resize(len(constants.RamOptions), max(l.Height-2, 1))
	return s
}

// Scroll reports the allocation list's position.
func (s *MemoryStep) Scroll() core.Scroll { return s.list.Scroll }
