package pages

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium-refactor/tui/internal/assets"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/components"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/core"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/styles"
)

type HomeModel struct {
	layout core.Layout

	nav components.NavigationModel
}

func NewHomeModel() HomeModel {
	return HomeModel{
		nav: components.NewNavigationModel([]components.Choice{
			{Page: "Create server", Key: "c"},
			{Page: "Manage servers", Key: "m"},
			{Page: "Settings", Key: "s"},
			{Page: "Quit", Key: "q"},
		}),
	}
}

func (m HomeModel) Init() tea.Cmd {

	return nil
}

func (m HomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Intercept the selection message from the navigation child component
	case components.MenuSelectionMsg:
		switch msg.Selection.Page {
		case "Quit":
			return m, tea.Quit

		case "Create server":
			// Ask the parent to switch to the CreateServer page
			return m, core.RouteTo("CreateServer")

		case "Manage servers":
			return m, core.RouteTo("ManageServers")

		case "Settings":
			return m, core.RouteTo("Settings")
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		case "d":
			return m, core.RouteTo("Dashboard") // send a ChangePageMsg

		// case "h", "backspace":
		// 	return m, core.RouteTo("Home")

		case "c":
			return m, core.RouteTo("CreateServer")

		case "m":
			return m, core.RouteTo("ManageServers")

		case "s":
			return m, core.RouteTo("Settings")
		}
	}

	var cmd tea.Cmd
	m.nav, cmd = m.nav.Update(msg)
	return m, cmd
}

func (m HomeModel) View() tea.View {
	logo, _ := styles.Logo(assets.Logo2)

	content := lipgloss.JoinVertical(
		0.5,
		logo,
		"\n",
		m.nav.View().Content,
	)

	return tea.NewView(lipgloss.PlaceHorizontal(m.layout.Width, lipgloss.Center, content))
}

// additional methods
func (m HomeModel) Title() string {
	return "Home"
}

func (m HomeModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l
	return m
}
