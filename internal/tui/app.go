package tui

import (
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
		home:          home,
		createServer:  pages.NewCreateServerModel(store),
		manageServers: pages.NewManageServersModel(store),
		settings:      pages.NewSettingsModel(),
		help:          components.NewHelpModel(components.DefaultKeys),
	}
}

// titleHeight is how many rows the page title bar takes, which every page below
// it has to be shifted down by for mouse coordinates to line up.
func (m appModel) titleHeight(p Page) int {
	if p == nil || p.Title() == "Home" {
		return 0
	}
	return lipgloss.Height(styles.Title(m.layout.Width, "Measure", p.Title()))
}

func (m appModel) getPageLayout(p Page) core.Layout {
	height := m.titleHeight(p)

	return core.Layout{
		X:      m.layout.X,
		Y:      m.layout.Y + height,
		Width:  m.layout.Width,
		Height: max(0, m.layout.Height-height),
	}
}

func (m appModel) switchPage(p Page) (tea.Model, tea.Cmd) {
	m.activePage = p.SetLayout(m.getPageLayout(p)).(Page) // settings window dimensions here, in case a page didn't receive the latest window layout

	return m, m.activePage.Init()
}

// quit shuts down anything the pages own before leaving.
func (m appModel) quit() tea.Cmd {
	return tea.Sequence(core.Shutdown(), tea.Quit)
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
			if msg.String() == "ctrl+c" {
				return m, m.quit()
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
		case "ctrl+c":
			return m, m.quit()

		// Only the home page can quit on a bare "q"; elsewhere it is an
		// ordinary character that panels with text inputs need to receive.
		case "q":
			if m.activePage.Title() == "Home" {
				return m, m.quit()
			}
		}

	// Manage Servers owns the running server process, so it is told to shut
	// down wherever the user happens to be when they quit.
	case core.ShutdownMsg:
		updated, _ := m.manageServers.Update(msg)
		m.manageServers = updated.(pages.ManageServersModel)
		return m, nil

	case core.ChangePageMsg:
		switch msg.Target {
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

	// Keep the stored copy of the page in step with the active one so state
	// survives navigating away and back.
	m.retain(m.activePage)

	return m, tea.Batch(helpCmd, cmd)
}

// retain writes the active page back into its slot, so a page keeps its state
// across navigation instead of being reset to the copy made at startup.
func (m *appModel) retain(page Page) {
	switch p := page.(type) {
	case pages.HomeModel:
		m.home = p
	case pages.CreateServerModel:
		m.createServer = p
	case pages.ManageServersModel:
		m.manageServers = p
	case pages.SettingsModel:
		m.settings = p
	}
}

func (m appModel) View() tea.View {
	pageView := m.activePage.View()

	var title string
	if m.activePage.Title() != "Home" {
		title = styles.Title(m.layout.Width, "Osmium", m.activePage.Title())
	}

	baseContent := lipgloss.JoinVertical(
		0,
		title,
		pageView.Content,
	)

	// Delegate compositing and rendering logic entirely to the help component
	viewContent := m.help.Render(baseContent)

	view := tea.NewView(viewContent)
	view.AltScreen = true

	// Cell motion reports clicks and wheel events with coordinates, which is
	// what the panels hit-test against.
	view.MouseMode = tea.MouseModeCellMotion

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
