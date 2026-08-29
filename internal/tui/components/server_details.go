package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// ServerDetailsModel is an early sketch of the details panel, kept for the
// layout it establishes.
type ServerDetailsModel struct {
	layout core.Layout

	value   int
	isFocus bool
}

// NewServerDetailsModel returns the details sketch.
func NewServerDetailsModel() ServerDetailsModel {
	return ServerDetailsModel{}
}

func (m ServerDetailsModel) Init() tea.Cmd {
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
	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:    m.layout.Width,
		Height:   m.layout.Height,
		Focused:  m.isFocus,
		Title:    m.Title(),
		Content:  fmt.Sprint(m.value),
		ShowHelp: true,
	}))
}

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
