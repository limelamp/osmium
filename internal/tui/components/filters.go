package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

type FiltersModel struct {
	layout core.Layout

	value   int
	count   int
	isFocus bool
}

func NewFiltersModel() FiltersModel {
	return FiltersModel{}
}

func (m FiltersModel) Init() tea.Cmd {
	m.count++

	return nil
}

func (m FiltersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m FiltersModel) View() tea.View {

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
func (m FiltersModel) Title() string {
	return "Filters"
}

func (m FiltersModel) SetLayout(l core.Layout) FiltersModel {
	m.layout = l
	return m
}

func (m FiltersModel) SetFocus(focused bool) FiltersModel {
	m.isFocus = focused
	return m
}
