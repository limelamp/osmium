package actions

import (
	"fmt"
	"sort"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// projectKind is what separates the mod panel from the plugin panel: the folder
// Modrinth downloads land in, and the words shown to the user. Everything else
// about managing mods and plugins is identical, so both panels are the same
// model with a different kind.
type projectKind struct {
	title  string
	folder string
	noun   string
}

var (
	modKind    = projectKind{title: "Mod Management", folder: "mods", noun: "mod"}
	pluginKind = projectKind{title: "Plugin Management", folder: "plugins", noun: "plugin"}
)

// projectState is which of the panel's nested screens is showing.
type projectState int

const (
	stateOperations projectState = iota
	stateAdd
	stateRemove
	stateUpdate
)

// projectOperations are the entries of the panel's root list, in order.
var projectOperations = []string{
	"Add a new %s",
	"Remove installed %ss",
	"Install all tracked %ss",
	"Update installed %ss",
	"Track untracked %ss",
}

// ProjectsModel manages the Modrinth mods or plugins of the selected server.
type ProjectsModel struct {
	base

	kind  projectKind
	state projectState

	// installed is the slug list read from the server's osmium.json.
	installed []string
	selected  map[int]bool

	queryInput textinput.Model
}

// NewModManagementModel returns the mods variant of the projects panel.
func NewModManagementModel() ProjectsModel {
	return newProjectsModel(modKind)
}

// NewPluginManagementModel returns the plugins variant of the projects panel.
func NewPluginManagementModel() ProjectsModel {
	return newProjectsModel(pluginKind)
}

func newProjectsModel(kind projectKind) ProjectsModel {
	ti := textinput.New()
	ti.Placeholder = "Enter " + kind.noun + " id or slug..."
	ti.Prompt = "» "
	ti.CharLimit = 60
	ti.SetWidth(40)

	m := ProjectsModel{
		kind:       kind,
		selected:   make(map[int]bool),
		queryInput: ti,
	}
	m.base = m.setCount(len(projectOperations))
	return m
}

// Editing reports whether the id/slug field is open.
func (m ProjectsModel) Editing() bool { return m.state == stateAdd }

func (m ProjectsModel) Init() tea.Cmd {
	return nil
}

// operations renders the root list labels for this panel's kind.
func (m ProjectsModel) operations() []string {
	labels := make([]string, len(projectOperations))
	for i, format := range projectOperations {
		labels[i] = fmt.Sprintf(format, m.kind.noun)
	}
	return labels
}

// reload rereads what is tracked and returns to the panel's root screen. It is
// what a fresh selection gets: a different server is a different panel.
func (m ProjectsModel) reload() ProjectsModel {
	return m.refresh().gotoState(stateOperations)
}

// refresh rereads which projects the selected server has tracked, leaving the
// screen the user is on exactly where it is.
//
// An operation finishing is a reason to re-read the list, not a reason to throw
// the user out of the screen they are working in — so removing one of three
// mods leaves the other two on screen, freshly read, rather than bouncing back
// to the menu.
func (m ProjectsModel) refresh() ProjectsModel {
	m.installed = nil

	if conf, err := config.ReadConfigIn(m.dir()); err == nil {
		tracked := conf.Mods
		if m.kind.folder == "plugins" {
			tracked = conf.Plugins
		}
		for slug := range tracked {
			m.installed = append(m.installed, slug)
		}
		sort.Strings(m.installed)
	}

	m.selected = make(map[int]bool)
	m.base = m.setCount(m.itemCount())
	return m
}

// gotoState switches nested screens and resizes the list to the new contents.
func (m ProjectsModel) gotoState(state projectState) ProjectsModel {
	m.state = state
	m.selected = make(map[int]bool)
	m.list = core.List{}
	m.queryInput.Blur()
	m.base = m.setCount(m.itemCount())
	return m
}

// itemCount is how many rows the current screen lists.
func (m ProjectsModel) itemCount() int {
	switch m.state {
	case stateRemove:
		return len(m.installed)
	case stateUpdate:
		return len(m.installed) + 1 // plus the "update everything" row
	case stateAdd:
		return 0
	default:
		return len(projectOperations)
	}
}

func (m ProjectsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.RefreshMsg:
		// An install or a removal has finished; what is tracked has changed.
		return m.refresh(), nil

	case tea.MouseMsg:
		updated, activated := m.handleMouse(msg)
		m.base = updated
		if activated {
			return m.activate()
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.state == stateAdd {
			switch msg.String() {
			case "enter":
				return m.activate()
			case "esc":
				if m.queryInput.Value() != "" {
					m.queryInput.Reset()
					return m, nil
				}
				return m.gotoState(stateOperations), nil

			case "backspace":
				if m.queryInput.Value() != "" {
					break // let the input handle its own deletion
				}
				return m.gotoState(stateOperations), nil
			}

			var cmd tea.Cmd
			m.queryInput, cmd = m.queryInput.Update(msg)
			return m, cmd
		}

		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

		switch msg.String() {
		case "backspace":
			// Nested navigation: back out of the sub-screen first.
			if m.state != stateOperations {
				return m.gotoState(stateOperations), nil
			}
			return m, core.CloseAction()

		case "space":
			if m.state == stateRemove && len(m.installed) > 0 {
				m.selected[m.list.Cursor] = !m.selected[m.list.Cursor]
			}

		case "ctrl+a":
			if m.state == stateRemove {
				return m.toggleAll(), nil
			}

		case "enter":
			return m.activate()
		}
	}

	return m, nil
}

// activate runs whatever the current screen's enter key means.
func (m ProjectsModel) activate() (tea.Model, tea.Cmd) {
	if m.needsServer() {
		return m, nil
	}

	folder := m.kind.folder

	switch m.state {
	case stateOperations:
		switch m.list.Cursor {
		case 0:
			m = m.gotoState(stateAdd)
			return m, m.queryInput.Focus()
		case 1:
			return m.gotoState(stateRemove), nil
		case 2:
			return m, m.run("Installing tracked "+m.kind.noun+"s...", func(sc scope.Scope) error {
				return shared.InstallProjectsFromConfig(sc)
			})
		case 3:
			return m.gotoState(stateUpdate), nil
		case 4:
			return m, m.run("Tracking untracked "+m.kind.noun+"s...", func(sc scope.Scope) error {
				return shared.TrackProjects(sc)
			})
		}

	case stateAdd:
		query := m.queryInput.Value()
		if query == "" {
			return m, nil
		}
		m.queryInput.Reset()
		return m.gotoState(stateOperations), m.run("Adding "+query+"...", func(sc scope.Scope) error {
			return shared.AddProjectByID(sc, query, folder)
		})

	case stateRemove:
		slugs := m.markedSlugs()
		if len(slugs) == 0 {
			return m, nil
		}
		return m, m.run("Removing "+m.kind.noun+"s...", func(sc scope.Scope) error {
			for _, slug := range slugs {
				if err := shared.RemoveProjectByID(sc, slug, folder); err != nil {
					return err
				}
				sc.Logf("Removed %s", slug)
			}
			return nil
		})

	case stateUpdate:
		if m.list.Cursor == 0 {
			return m, m.run("Updating all "+m.kind.noun+"s...", func(sc scope.Scope) error {
				return shared.UpdateAllProjects(sc, folder)
			})
		}
		slug := m.installed[m.list.Cursor-1]
		return m, m.run("Updating "+slug+"...", func(sc scope.Scope) error {
			return shared.UpdateProject(sc, slug, folder)
		})
	}

	return m, nil
}

// toggleAll marks every installed project, or clears them all when they
// already are.
func (m ProjectsModel) toggleAll() ProjectsModel {
	marked := len(m.markedSlugs())

	for i := range m.installed {
		m.selected[i] = marked < len(m.installed)
	}
	return m
}

// markedSlugs collects the projects the user ticked on the remove screen.
func (m ProjectsModel) markedSlugs() []string {
	var slugs []string
	for i, marked := range m.selected {
		if marked && i >= 0 && i < len(m.installed) {
			slugs = append(slugs, m.installed[i])
		}
	}
	return slugs
}

func (m ProjectsModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	switch m.state {
	case stateAdd:
		body := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Foreground(theme.Inactive).
				Render("Modrinth id or slug of the "+m.kind.noun+" to install:"),
			"",
			m.queryInput.View(),
		)
		return m.panelView(m.Title()+" · Add", body, m.Hints()...)

	case stateRemove:
		if len(m.installed) == 0 {
			return m.emptyView(m.Title()+" · Remove", "No tracked "+m.kind.noun+"s to remove.",
				styles.Hint{Key: "backspace", Desc: "back"})
		}
		return m.listView(m.Title()+" · Remove", func(i int) string {
			label := "  " + m.installed[i]
			if m.selected[i] {
				label = lipgloss.NewStyle().Foreground(theme.Accent).Render("✓ " + m.installed[i])
			}
			return m.row(i, label, "")
		}, m.Hints()...)

	case stateUpdate:
		return m.listView(m.Title()+" · Update", func(i int) string {
			if i == 0 {
				return m.row(i, "All "+m.kind.noun+"s", "")
			}
			return m.row(i, m.installed[i-1], "")
		}, m.Hints()...)

	default:
		labels := m.operations()
		return m.listView(m.Title(), func(i int) string {
			return m.row(i, labels[i], "")
		}, m.Hints()...)
	}
}

// Hints are the keys this panel responds to, which depend on which of its
// nested screens is showing.
func (m ProjectsModel) Hints() []styles.Hint {
	switch m.state {
	case stateAdd:
		return []styles.Hint{
			{Key: "enter", Desc: "install"},
			{Key: "esc", Desc: "clear / back"},
		}

	case stateRemove:
		return []styles.Hint{
			{Key: "space", Desc: "mark"},
			{Key: "ctrl+a", Desc: "mark all"},
			{Key: "enter", Desc: "remove marked"},
			{Key: "backspace", Desc: "back"},
		}

	case stateUpdate:
		return []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "enter", Desc: "update"},
			{Key: "backspace", Desc: "back"},
		}

	default:
		return []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "enter", Desc: "select"},
			{Key: "backspace", Desc: "back"},
		}
	}
}

// additional methods
func (m ProjectsModel) Title() string {
	return m.kind.title
}

func (m ProjectsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.queryInput.SetWidth(max(l.ContentWidth()-4, 10))
	m.base = m.setCount(m.itemCount())
	return m
}

func (m ProjectsModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m ProjectsModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	return m.reload()
}
