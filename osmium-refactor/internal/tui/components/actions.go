package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/core"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/styles"
)

type ActionsModel struct {
	layout core.Layout

	value   int
	count   int
	isFocus bool
}

func NewActionsModel() ActionsModel {
	return ActionsModel{}
}

func (m ActionsModel) Init() tea.Cmd {
	m.count++

	return nil
}

func (m ActionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "e":
			m.value++

		case "t":
			m.value++
		}
	}

	return m, nil
}

func (m ActionsModel) View() tea.View {

	return tea.NewView(styles.Container(
		m.layout.Width,
		m.layout.Height,
		m.isFocus,
		m.Title(),
		fmt.Sprint(m.value),
		false,
	))
}

// additional methods
func (m ActionsModel) Title() string {
	return "Actions"
}

func (m ActionsModel) SetLayout(l core.Layout) ActionsModel {
	m.layout = l
	return m
}

func (m ActionsModel) SetFocus(focused bool) ActionsModel {
	m.isFocus = focused
	return m
}
