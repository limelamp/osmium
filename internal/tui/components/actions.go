package components

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/actions"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

type ActionsModel struct {
	layout core.Layout

	count   int
	isFocus bool

	cursor  int
	options []string
}

func NewActionsModel() ActionsModel {
	return ActionsModel{
		cursor:  0,
		options: []string{"Remove files", "Generate Run Script", "Manage Configs", "Mod Management", "Plugin Management"},
	}
}

func (m ActionsModel) Init() tea.Cmd {
	m.count++

	return nil
}

func (m ActionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			switch m.cursor {
			case 0:
				return m, func() tea.Msg {
					return core.SwitchActionMsg{
						NewAction: actions.NewRemoveFilesModel(),
					}
				}
				// core.SwitchAction(actions.NewRemoveFilesModel())

			case 1:
				return m, func() tea.Msg {
					return core.SwitchActionMsg{
						NewAction: actions.NewGenRunScriptModel(),
					}
				}
			case 2:
				return m, func() tea.Msg {
					return core.SwitchActionMsg{
						NewAction: actions.NewManageConfigsModel(),
					}
				}
			case 3:
				return m, func() tea.Msg {
					return core.SwitchActionMsg{
						NewAction: actions.NewModManagementModel(),
					}
				}
			case 4:
				return m, func() tea.Msg {
					return core.SwitchActionMsg{
						NewAction: actions.NewPluginManagementModel(),
					}
				}
			}

		}
	}

	return m, nil
}

func (m ActionsModel) View() tea.View {
	content := ""

	// Create a simple list
	for i := 0; i < len(m.options); i++ {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}

		content += "\n" + cursor + m.options[i]
	}

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
func (m ActionsModel) Title() string {
	return "Actions"
}

func (m ActionsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	return m
}

func (m ActionsModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}
