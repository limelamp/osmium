// Package tui is the application itself: the model that owns the pages,
// routes between them, and draws the help overlay over whichever one is
// showing.
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

// Page is one screen of the app: a model that can be told the rectangle it
// has and can name itself for the title bar.
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
	migrate       pages.MigrateModel
	settings      pages.SettingsModel

	// help modal overlay component
	help components.HelpModel

	// What the mouse has selected, and what the last copy had to say for
	// itself.
	selection core.Selection
	copied    string

	// drags counts the drags there have been, which names the current one so
	// that the beats of an abandoned drag can be told apart from its own.
	// scrolling says one of those beats is already on its way.
	drags     int
	scrolling bool
}

// NewAppModel returns the application, opened on its home page.
func NewAppModel(store *storage.ServerStore) appModel {
	home := pages.NewHomeModel(store)

	return appModel{
		activePage:    home,
		home:          home,
		createServer:  pages.NewCreateServerModel(store),
		manageServers: pages.NewManageServersModel(store),
		migrate:       pages.NewMigrateModel(store),
		settings:      pages.NewSettingsModel(),
		help:          components.NewHelpModel(),
	}
}

// NewAppModelFor returns the app already open on a server's Manage Servers
// page, which is what "osmium --server survival" and running Osmium inside a
// server folder ask for. A nil server opens the app as usual, on Home.
func NewAppModelFor(store *storage.ServerStore, server *storage.Server) appModel {
	m := NewAppModel(store)
	if server == nil {
		return m
	}

	m.manageServers = m.manageServers.OpenServer(server)
	m.activePage = m.manageServers
	return m
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
	// The page is given the window before it is shown, in case it never
	// received the latest size while it was in the background.
	m.activePage = asPage(p.SetLayout(m.getPageLayout(p)))

	return m, m.activePage.Init()
}

// asPage converts a model back into the page it is. Every page returns itself
// from SetLayout, so a failure here is a programming mistake rather than
// something to recover from - but it should say which type broke the contract.
func asPage(model tea.Model) Page {
	page, ok := model.(Page)
	if !ok {
		panic(fmt.Sprintf("osmium: %T is not a page", model))
	}
	return page
}

// quit leaves the app. Running servers are deliberately left alone: each one
// belongs to its own supervisor process, so quitting Osmium is not the same as
// stopping a server, and a server started here is still running - and still
// takes commands - the next time Osmium is opened.
func (m appModel) quit() tea.Cmd {
	return tea.Quit
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

	// The click that dismissed the help modal is spent on dismissing it: it
	// must not also land on whatever it was covering.
	if _, ok := msg.(tea.MouseClickMsg); ok && wasVisible {
		return m, helpCmd
	}

	// Selecting text with the mouse takes precedence over what a click would
	// otherwise do, and a click only reaches the page once the button comes up
	// where it went down.
	if mouse, ok := msg.(tea.MouseMsg); ok {
		updated, cmd, handled := m.handleMouse(mouse)
		if handled {
			return updated, tea.Batch(helpCmd, cmd)
		}

		if model, isApp := updated.(appModel); isApp {
			m = model
		}
		if release, isRelease := msg.(tea.MouseReleaseMsg); isRelease {
			// The press was held back so that a drag could claim it; now that
			// it turned out to be a click, the page gets it.
			msg = tea.MouseClickMsg(release)
		}
	}

	// 3. Process normal system and application events
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = core.NewLayout(msg.Width, msg.Height)
		m.activePage = asPage(m.activePage.SetLayout(m.getPageLayout(m.activePage)))
		return m, helpCmd // handled locally, no need to forward to other pages

	case tea.KeyPressMsg:
		// Typing anything means the selection is done with.
		m.selection = m.selection.Clear()

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

	// The console lives on the Manage Servers page, but an operation started
	// there can finish while the user is somewhere else. Its output is handed
	// to that page wherever they are, instead of to whatever page happens to be
	// on screen, which would drop it.
	case core.LogMsg, core.OperationDoneMsg:
		if m.activePage.Title() != m.manageServers.Title() {
			updated, cmd := m.manageServers.Update(msg)
			if page, ok := updated.(pages.ManageServersModel); ok {
				m.manageServers = page
			}
			return m, cmd
		}

	case dragScrollMsg:
		model, cmd := m.dragScroll(msg)
		return model, cmd

	case core.ThemeChangedMsg:
		// A new theme has to reach every page, not only the one in front: a
		// panel that keeps text it has already rendered, or a component that
		// was handed its styles when it was built, has to be told to build
		// them again, and it may well be behind another page when it is.
		return m.everyPage(msg)

	case copiedMsg:
		m.copied = copiedReport(msg)
		return m, clearCopied()

	case clearCopiedMsg:
		m.copied = ""
		return m, nil

	case core.ChangePageMsg:
		switch msg.Target {
		case "Home":
			return m.switchPage(m.home)

		case "CreateServer":
			return m.switchPage(m.createServer) // page switch, return immediately

		case "ManageServers":
			return m.switchPage(m.manageServers)

		case "Migrate":
			// The routing request carries the server when there is one, so the
			// page skips its own picker.
			m.migrate = m.migrate.SetServer(msg.Server)
			return m.switchPage(m.migrate)

		case "Settings":
			return m.switchPage(m.settings)
		}
	}

	// forwards the same incoming message to the currently active page model
	updated, cmd := m.activePage.Update(msg)
	m.activePage = asPage(updated)

	// Keep the stored copy of the page in step with the active one so state
	// survives navigating away and back.
	m.retain(m.activePage)

	return m, tea.Batch(helpCmd, cmd)
}

// everyPage hands a message to all five pages and keeps the one on screen as
// the copy that is drawn.
func (m appModel) everyPage(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 5)

	for _, page := range []Page{m.home, m.createServer, m.manageServers, m.migrate, m.settings} {
		updated, cmd := page.Update(msg)
		m.retain(asPage(updated))
		cmds = append(cmds, cmd)
	}

	m.activePage = m.stored(m.activePage)
	return m, tea.Batch(cmds...)
}

// stored is the app's own copy of a page, which is where retain has just put
// the updated one.
func (m appModel) stored(page Page) Page {
	switch page.(type) {
	case pages.HomeModel:
		return m.home
	case pages.CreateServerModel:
		return m.createServer
	case pages.ManageServersModel:
		return m.manageServers
	case pages.MigrateModel:
		return m.migrate
	case pages.SettingsModel:
		return m.settings
	}
	return page
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
	case pages.MigrateModel:
		m.migrate = p
	case pages.SettingsModel:
		m.settings = p
	}
}

// baseFrame is the app as it stands, before the selection highlight and the
// help modal are drawn over it.
//
// A page's View is a function of its model and nothing else, so this can be
// asked for again when a selection needs to know what was on the screen -
// View itself cannot keep a copy, having only a value receiver.
func (m appModel) baseFrame() string {
	// The confirmation is shown where the help indicator is, which every page
	// draws for itself in its own bottom-right corner.
	styles.Notice(m.copied)

	content := m.activePage.View().Content

	// A page without a title bar is drawn on its own. Joining it to an empty
	// title would still spend a row on that title, putting every row of the
	// page one lower than the layout its components hit-test clicks against -
	// which is why the title bar's presence is decided by the same titleHeight
	// the layout is derived from.
	if m.titleHeight(m.activePage) > 0 {
		content = lipgloss.JoinVertical(
			0,
			styles.Title(m.layout.Width, "Osmium", m.activePage.Title()),
			content,
		)
	}

	return content
}

func (m appModel) View() tea.View {
	// Below the minimum there is no layout worth drawing: every pane would be
	// smaller than its own borders.
	if m.layout.Width < styles.MinWidth || m.layout.Height < styles.MinHeight {
		view := tea.NewView(styles.TooSmall(m.layout.Width, m.layout.Height))
		view.AltScreen = true
		return view
	}

	baseContent := m.highlight(m.baseFrame())

	// The page describes the keys that work where the user actually is, so the
	// modal lists those rather than one fixed set for the whole app.
	var sections []core.HelpSection
	if helper, ok := m.activePage.(core.Helper); ok {
		sections = helper.Help()
	}

	viewContent := m.help.Render(baseContent, sections)

	view := tea.NewView(viewContent)
	view.AltScreen = true

	// Cell motion reports clicks and wheel events with coordinates, which is
	// what the panels hit-test against.
	view.MouseMode = tea.MouseModeCellMotion

	return view
}

func (m appModel) Title() string {
	return "App"
}

func (m appModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l
	return m
}
