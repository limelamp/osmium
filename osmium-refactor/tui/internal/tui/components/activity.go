package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/core"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/styles"
)

type ActivityModel struct {
	layout core.Layout

	value   int
	count   int
	isFocus bool
}

func NewActivityModel() ActivityModel {
	return ActivityModel{}
}

func (m ActivityModel) Init() tea.Cmd {
	m.count++

	return nil
}

func (m ActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ActivityModel) View() tea.View {

	return tea.NewView(styles.Container(
		m.layout.Width,
		m.layout.Height,
		m.isFocus,
		m.Title(),
		fmt.Sprint(m.value),
		true,
	))
}

// additional methods
func (m ActivityModel) Title() string {
	return "Activity"
}

func (m ActivityModel) SetLayout(l core.Layout) ActivityModel {
	m.layout = l
	return m
}

func (m ActivityModel) SetFocus(focused bool) ActivityModel {
	m.isFocus = focused
	return m
}
