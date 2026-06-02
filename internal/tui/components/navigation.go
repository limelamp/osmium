package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/assets"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
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

func (m NavigationModel) Update(msg tea.Msg) (NavigationModel, tea.Cmd) {
	switch msg := msg.(type) {
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
			selected := m.choices[m.cursor]

			//? returns a message to be catched by a parent tea.Model
			return m, func() tea.Msg {
				return MenuSelectionMsg{Selection: selected}
			}
		}
	}

	return m, nil
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
