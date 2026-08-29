package actions

import (
	"fmt"
	"sort"

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
	stateProject
)

// The root screen's rows: the operations, then a blank and a heading, then one
// row per installed project. The cursor walks the whole way down, which is what
// lets a long list of mods be reached at all.
var (
	installedBlank   = len(projectOperations)
	installedHeading = installedBlank + 1
	installedFirst   = installedHeading + 1
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

	// chosen is the installed project the per-project screen is about.
	chosen string

	// installed is the slug list read from the server's osmium.json, and
	// versions is what is installed for each of them.
	installed []string
	versions  map[string]string
	selected  map[int]bool

	query core.Field
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
	m := ProjectsModel{
		base:     newBase(),
		kind:     kind,
		selected: make(map[int]bool),
		query: core.NewField("Enter " + kind.noun + " id or slug...").
			WithPrompt("» ").WithLimit(60).SetWidth(40),
	}
	m.base = m.setCount(len(projectOperations))
	return m
}

// Editing reports whether the id/slug field is open.
func (m ProjectsModel) Editing() bool { return m.query.IsOpen() }

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
// the user out of the screen they are working in - so removing one of three
// mods leaves the other two on screen, freshly read, rather than bouncing back
// to the menu.
func (m ProjectsModel) refresh() ProjectsModel {
	m.installed, m.versions = nil, map[string]string{}

	if conf, err := config.ReadConfigIn(m.dir()); err == nil {
		tracked := conf.Mods
		if m.kind.folder == "plugins" {
			tracked = conf.Plugins
		}
		for slug, project := range tracked {
			m.installed = append(m.installed, slug)
			m.versions[slug] = project.VersionNumber
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
	m.query = m.query.Close()
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
	case stateProject:
		return len(projectActions)
	default:
		return installedFirst + len(m.installed)
	}
}

// selectableRow reports whether the cursor may rest on a row of the root
// screen: everything but the blank and the heading above the installed list.
func (m ProjectsModel) selectableRow(index int) bool {
	return index != installedBlank && index != installedHeading
}

// chosenProject is the installed project the cursor is on, or an empty string
// when it is on one of the operations.
func (m ProjectsModel) chosenProject() string {
	index := m.list.Cursor - installedFirst
	if m.state != stateOperations || index < 0 || index >= len(m.installed) {
		return ""
	}
	return m.installed[index]
}

// projectActions are what can be done to one installed project.
var projectActions = []string{"Update %s", "Remove %s"}

func (m ProjectsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.common(msg); handled {
		m.base = updated
		return m, cmd
	}

	switch msg := msg.(type) {
	case core.OperationDoneMsg:
		// The operation has finished: say so, and re-read what is tracked.
		updated, cmd := m.finished(msg)
		m.base = updated
		return m.refresh(), cmd

	case core.RefreshMsg:
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
			// Backspace on an empty query leaves the screen; otherwise it is
			// the field's own deletion.
			if msg.String() == "backspace" && m.query.Value() == "" {
				return m.gotoState(stateOperations), nil
			}

			field, result, cmd := m.query.HandleKey(msg)
			m.query = field

			switch result {
			case core.FieldCommitted:
				return m.activate()

			case core.FieldCancelled:
				// The first esc clears what was typed; with nothing left to
				// clear, the second leaves the screen.
				if m.query.Value() == "" {
					return m.gotoState(stateOperations), nil
				}
				return m.reopenQuery()
			}

			return m, cmd
		}

		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list

			if m.state == stateOperations {
				direction := 1
				if key := msg.String(); key == "up" || key == "k" || key == "pgup" {
					direction = -1
				}
				m.list = core.Settle(m.list, direction, m.selectableRow)
			}
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
	case stateProject:
		slug := m.chosen
		if m.list.Cursor == 0 {
			return m.start(0, "Updating "+slug+"…", func(sc scope.Scope) error {
				return shared.UpdateProject(sc, slug, folder)
			})
		}

		return m.start(1, "Removing "+slug+"…", func(sc scope.Scope) error {
			return shared.RemoveProjectByID(sc, slug, folder)
		})

	case stateOperations:
		// A row below the operations is an installed project: open what can be
		// done to that one.
		if slug := m.chosenProject(); slug != "" {
			m.chosen = slug
			return m.gotoState(stateProject), nil
		}

		switch m.list.Cursor {
		case 0:
			m = m.gotoState(stateAdd)
			return m.reopenQuery()
		case 1:
			return m.gotoState(stateRemove), nil
		case 2:
			return m.start(2, "Installing tracked "+m.kind.noun+"s…",
				shared.InstallProjectsFromConfig)
		case 3:
			return m.gotoState(stateUpdate), nil
		case 4:
			return m.start(4, "Identifying jars…", shared.TrackProjects)
		}

	case stateAdd:
		query := m.query.Value()
		if query == "" {
			return m.reopenQuery()
		}
		m.query = m.query.SetValue("")
		m = m.gotoState(stateOperations)

		return m.start(0, "Adding "+query+"…", func(sc scope.Scope) error {
			return shared.AddProjectByID(sc, query, folder)
		})

	case stateRemove:
		slugs := m.markedSlugs()
		if len(slugs) == 0 {
			return m, nil
		}
		return m.start(m.list.Cursor, "Removing "+plural(len(slugs), m.kind.noun)+"…",
			func(sc scope.Scope) error {
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
			return m.start(0, "Updating every "+m.kind.noun+"…", func(sc scope.Scope) error {
				return shared.UpdateAllProjects(sc, folder)
			})
		}

		slug := m.installed[m.list.Cursor-1]
		return m.start(m.list.Cursor, "Updating "+slug+"…", func(sc scope.Scope) error {
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

// start runs an operation from a row, so the row can show that it is working.
func (m ProjectsModel) start(row int, label string, op func(scope.Scope) error) (tea.Model, tea.Cmd) {
	updated, cmd := m.begin(row, label, op)
	m.base = updated
	return m, cmd
}

// plural renders a count with its noun.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

// reopenQuery puts the keyboard back in the id field, which is where it belongs
// for as long as the add screen is showing.
func (m ProjectsModel) reopenQuery() (tea.Model, tea.Cmd) {
	field, cmd := m.query.Open(m.query.Value())
	m.query = field
	return m, cmd
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
			m.query.View(),
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
			return m.row(i, m.rowMarker(i)+label, m.versions[m.installed[i]])
		}, m.Hints()...)

	case stateUpdate:
		return m.listView(m.Title()+" · Update", func(i int) string {
			if i == 0 {
				return m.row(i, m.rowMarker(0)+"All "+m.kind.noun+"s", "")
			}
			slug := m.installed[i-1]
			return m.row(i, m.rowMarker(i)+slug, m.versions[slug])
		}, m.Hints()...)

	case stateProject:
		return m.listView(m.Title()+" · "+m.chosen, func(i int) string {
			return m.row(i, m.rowMarker(i)+fmt.Sprintf(projectActions[i], m.chosen),
				m.versions[m.chosen])
		}, m.Hints()...)

	default:
		return m.listView(m.Title(), m.rootRow, m.Hints()...)
	}
}

// rootRow draws one row of the root screen: the operations, then the installed
// projects under their heading. It is one list so that the cursor can walk from
// the operations into the projects, and so a long list scrolls.
func (m ProjectsModel) rootRow(index int) string {
	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	switch {
	case index < len(projectOperations):
		return m.row(index, m.rowMarker(index)+m.operations()[index], "")

	case index == installedBlank:
		return ""

	case index == installedHeading:
		return muted.Render(fmt.Sprintf("INSTALLED (%d)", len(m.installed)))
	}

	slug := m.installed[index-installedFirst]
	return m.row(index, m.rowMarker(index)+slug, m.versions[slug])
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

	case stateProject:
		return []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "enter", Desc: "run"},
			{Key: "backspace", Desc: "back"},
		}

	default:
		what := "select"
		if m.chosenProject() != "" {
			what = "update or remove this " + m.kind.noun
		}

		return []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "enter", Desc: what},
			{Key: "backspace", Desc: "back"},
		}
	}
}

func (m ProjectsModel) Title() string {
	return m.kind.title
}

func (m ProjectsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.query = m.query.SetWidth(l.ContentWidth() - 4)
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
