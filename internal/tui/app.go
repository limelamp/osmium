package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/components"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/pages"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
)

type Page interface {
	tea.Model

	SetLayout(core.Layout) tea.Model

	Title() string
}

type appModel struct {
	activePage Page        // active page
	layout     core.Layout // dimensions of the window

	dashboard     pages.DashboardModel
	home          pages.HomeModel
	createServer  pages.CreateServerModel
	manageServers pages.ManageServersModel
	settings      pages.SettingsModel

	// help modal overlay component
	help components.HelpModel
}

func NewAppModel(store *storage.ServerStore) appModel {
	home := pages.NewHomeModel()

	return appModel{
		activePage:    home,
		dashboard:     pages.NewDashboardModel(),
		home:          home,
		createServer:  pages.NewCreateServerModel(),
		manageServers: pages.NewManageServersModel(store),
		settings:      pages.NewSettingsModel(),
		help:          components.NewHelpModel(components.DefaultKeys),
	}
}

func (m appModel) getPageLayout(p Page) core.Layout {
	layout := m.layout

	if p != nil && p.Title() != "Home" {
		dummyTitle := styles.Title(layout.Width, "Measure", p.Title())
		titleHeight := lipgloss.Height(dummyTitle)
		layout.Height = max(0, layout.Height-titleHeight)
	}

	return layout
}

func (m appModel) switchPage(p Page) (tea.Model, tea.Cmd) {
	m.activePage = p.SetLayout(m.getPageLayout(p)).(Page) // settings window dimensions here, in case a page didn't receive the latest window layout

	return m, m.activePage.Init()
}

func (m appModel) Init() tea.Cmd {
	return m.activePage.Init()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// 1. Check and cache the visibility state of the help overlay before updating it.
	wasVisible := m.help.IsVisible()

	// Update the help component with the incoming message.
	var helpCmd tea.Cmd
	m.help, helpCmd = m.help.Update(msg)

	// 2. Implement the input-filtering hook.
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		// If help was visible beforehand, or has just been opened by this specific keypress (e.g., '?'),
		// we intercept the event here to swallow it, preventing unwanted side effects in background views.
		if wasVisible || m.help.IsVisible() {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, helpCmd
		}
	}

	// 3. Process normal system and application events
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = core.NewLayout(msg.Width, msg.Height)
		m.activePage = m.activePage.SetLayout(m.getPageLayout(m.activePage)).(Page)
		return m, helpCmd // handled locally, no need to forward to other pages

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "esc":
			return m, core.RouteTo("Home")
		}

	case core.ChangePageMsg:
		switch msg.Target {
		case "Dashboard":
			return m.switchPage(m.dashboard)

		case "Home":
			return m.switchPage(m.home)

		case "CreateServer":
			return m.switchPage(m.createServer) // page switch, return immediately

		case "ManageServers":
			return m.switchPage(m.manageServers)

		case "Settings":
			return m.switchPage(m.settings)
		}
	}

	// forwards the same incoming message to the currently active page model
	updated, cmd := m.activePage.Update(msg)
	m.activePage = updated.(Page)

	return m, tea.Batch(helpCmd, cmd)
}

func (m appModel) View() tea.View {
	pageView := m.activePage.View()

	var title string
	if m.activePage.Title() != "Home" {
		title = styles.Title(
			m.layout.Width,
			fmt.Sprintf("Osmium %d", m.layout.Width),
			m.activePage.Title(),
		)
	}

	baseContent := lipgloss.JoinVertical(
		0.5,
		title,
		pageView.Content,
	)

	// Delegate compositing and rendering logic entirely to the help component
	viewContent := m.help.Render(baseContent)

	view := tea.NewView(viewContent)
	view.AltScreen = true

	return view
}

// additional methods
func (m appModel) Title() string {
	return "App"
}

func (m appModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l
	return m
}
