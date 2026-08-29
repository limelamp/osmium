package components

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// The menu is drawn like every other list in the app: one row per entry, no
// spacers. That is also what makes a click land on the row it was aimed at.

// Choice is one entry of the home menu: what it opens, and the key that opens
// it.
type Choice struct {
	Page string
	Key  string
}

// MenuSelectionMsg announces the entry the user chose.
type MenuSelectionMsg struct {
	Selection Choice
}

// NavigationModel is the home menu: a list of pages and the keys to reach
// them.
type NavigationModel struct {
	layout core.Layout

	choices []Choice
	cursor  int
}

// NewNavigationModel returns the menu for a set of choices.
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

	row := y - m.layout.Y
	if row < 0 || row >= len(m.choices) {
		return -1
	}
	return row
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
			m.cursor = (m.cursor - 1 + len(m.choices)) % len(m.choices)

		case "down", "j":
			m.cursor = (m.cursor + 1) % len(m.choices)

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
	return len(m.choices)
}

func (m NavigationModel) View() tea.View {
	rows := make([]string, 0, len(m.choices))
	for i, choice := range m.choices {
		rows = append(rows, styles.ListRow(m.layout.Width, i == m.cursor, choice.Page, choice.Key))
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m NavigationModel) SetLayout(l core.Layout) NavigationModel {
	m.layout = l
	return m
}
