package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/core"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/styles"
)

type ServerDetailsModel struct {
	layout core.Layout

	value   int
	count   int
	isFocus bool
}

func NewServerDetailsModel() ServerDetailsModel {
	return ServerDetailsModel{}
}

func (m ServerDetailsModel) Init() tea.Cmd {
	m.count++

	return nil
}

func (m ServerDetailsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ServerDetailsModel) View() tea.View {

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
func (m ServerDetailsModel) Title() string {
	return "Server Details"
}

func (m ServerDetailsModel) SetLayout(l core.Layout) ServerDetailsModel {
	m.layout = l
	return m
}

func (m ServerDetailsModel) SetFocus(focused bool) ServerDetailsModel {
	m.isFocus = focused
	return m
}
