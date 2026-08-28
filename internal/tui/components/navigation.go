package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/assets"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// Each menu entry is drawn with a blank row above it, so one choice occupies
// two terminal rows and starts on the second of them.
const (
	choiceStride = 2
	choiceOffset = 1
)

type Choice struct {
	Page string
	Key  string
}

type MenuSelectionMsg struct {
	Selection Choice
}

type NavigationModel struct {
	layout core.Layout

	choices []Choice
	cursor  int
}

func NewNavigationModel(choices []Choice) NavigationModel {
	return NavigationModel{
		choices: choices,
		cursor:  0,
	}
}

func (m NavigationModel) Init() tea.Cmd {
	return nil
}

// choiceAt maps an absolute terminal coordinate to a menu entry.
func (m NavigationModel) choiceAt(x, y int) int {
	if !m.layout.Hit(x, y) {
		return -1
	}

	row := y - m.layout.Y - choiceOffset
	if row < 0 || row%choiceStride != 0 {
		return -1
	}

	index := row / choiceStride
	if index >= len(m.choices) {
		return -1
	}
	return index
}

func (m NavigationModel) Update(msg tea.Msg) (NavigationModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		index := m.choiceAt(msg.X, msg.Y)
		if index < 0 || msg.Button != tea.MouseLeft {
			return m, nil
		}

		// Clicking the highlighted entry opens it; any other entry is selected
		// first, matching how the lists elsewhere in the app behave.
		if index == m.cursor {
			return m, m.selectCmd()
		}
		m.cursor = index
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.choices) - 1
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}

		case "enter":
			return m, m.selectCmd()
		}
	}

	return m, nil
}

// selectCmd announces the highlighted entry to the parent page.
func (m NavigationModel) selectCmd() tea.Cmd {
	selected := m.choices[m.cursor]

	//? returns a message to be catched by a parent tea.Model
	return func() tea.Msg {
		return MenuSelectionMsg{Selection: selected}
	}
}

// Height is how many rows the menu occupies.
func (m NavigationModel) Height() int {
	return len(m.choices) * choiceStride
}

func (m NavigationModel) View() tea.View {
	var s strings.Builder

	_, logoWidth := styles.Logo(assets.Logo2)

	for i, choice := range m.choices {
		isSelected := i == m.cursor
		s.WriteString(styles.Selection(logoWidth, isSelected, choice.Page, choice.Key))
		s.WriteByte('\n')
	}

	return tea.NewView(lipgloss.NewStyle().Width(logoWidth).Render(s.String()))
}

func (m NavigationModel) SetLayout(l core.Layout) NavigationModel {
	m.layout = l
	return m
}
