package pages

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/assets"
	"github.com/limelamp/osmium/internal/tui/components"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// logoGap is the blank space between the logo and the menu.
const logoGap = 1

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

	// The menu is centred under the logo; telling it exactly where it lands is
	// what lets it respond to clicks.
	logo, logoWidth := styles.Logo(assets.Logo2)

	m.nav = m.nav.SetLayout(core.Layout{
		X:      l.X + max(l.Width-logoWidth, 0)/2,
		Y:      l.Y + lipgloss.Height(logo) + logoGap,
		Width:  logoWidth,
		Height: m.nav.Height(),
	})

	return m
}
