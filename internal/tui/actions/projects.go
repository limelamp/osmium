package actions

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/cache"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/modrinth"
	"github.com/limelamp/osmium/internal/pack"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// projectKind is what separates the mod panel from the plugin panel: the words
// shown and the folder installed files land in. Everything else about managing
// mods and plugins is identical, so both panels are the same model.
type projectKind struct {
	title string
	kind  config.Kind
	noun  string
}

var (
	modKind    = projectKind{title: "Mod Management", kind: config.Mods, noun: "mod"}
	pluginKind = projectKind{title: "Plugin Management", kind: config.Plugins, noun: "plugin"}
)

// projectState is which of the panel's nested screens is showing.
type projectState int

const (
	stateList projectState = iota
	stateAdd
	stateRemove
	stateProject
	stateVersions
	statePlan
)

// The root screen's rows: the operations, then a blank and a heading, then one
// row per installed project.
var (
	installedBlank   = len(projectOperations)
	installedHeading = installedBlank + 1
	installedFirst   = installedHeading + 1
)

// projectOperations are the entries of the panel's root list, in order.
var projectOperations = []string{
	"Add a new %s",
	"Remove installed %ss",
	"Install what the manifest describes",
	"Update installed %ss",
	"Adopt untracked %ss",
}

// projectActions are what can be done to one installed project.
var projectActions = []string{"Update %s", "Choose a build of %s", "Pin %s", "Remove %s"}

// Operation is a piece of work the panel can be told to start as it opens,
// which is how the Actions menu's whole-server entries end up on the same plan
// screen as the ones asked for inside the panel.
type Operation int

// The operations the panel can be started on.
const (
	None Operation = iota
	Sync
	UpdateAll
	Adopt
)

// startMsg is the panel being told to get on with what it was opened for.
type startMsg struct{ op Operation }

// plannedMsg carries a resolution back to the panel that asked for one.
type plannedMsg struct {
	plan pack.Plan
	err  error
}

// versionsMsg carries the builds a project has for this server.
type versionsMsg struct {
	choices []pack.VersionChoice
	err     error
}

// ProjectsModel manages the mods or plugins of the selected server.
//
// Nothing it offers happens straight away: every operation resolves first and
// shows what it would do, and only what is agreed to is carried out. That is
// the whole difference between this and a list of buttons that download files.
type ProjectsModel struct {
	base

	kind  projectKind
	state projectState

	// installed is what the lock says the server has.
	installed []pack.Installed
	selected  map[int]bool

	// chosen is the project the per-project screens are about.
	chosen  string
	choices []pack.VersionChoice

	// plan is what is waiting to be agreed to, and resolver is the server it
	// was worked out against — the two travel together so that applying a plan
	// writes exactly what was shown.
	plan     pack.Plan
	resolver *pack.Resolver

	query  core.Field
	filter core.Field

	// pending is what the panel was opened to do, if anything.
	pending Operation
}

// Start opens the panel on a piece of work rather than on its menu.
func (m ProjectsModel) Start(op Operation) ProjectsModel {
	m.pending = op
	return m
}

// NewModManagementModel returns the mods variant of the projects panel.
func NewModManagementModel() ProjectsModel { return newProjectsModel(modKind) }

// NewPluginManagementModel returns the plugins variant of the projects panel.
func NewPluginManagementModel() ProjectsModel { return newProjectsModel(pluginKind) }

func newProjectsModel(kind projectKind) ProjectsModel {
	m := ProjectsModel{
		base:     newBase(),
		kind:     kind,
		selected: make(map[int]bool),
		query: core.NewField("name, or name@build...").
			WithPrompt("» ").WithLimit(60).SetWidth(40),
		filter: core.NewField("filter...").WithPrompt("/ ").WithLimit(40).SetWidth(30),
	}

	m.base = m.setCount(len(projectOperations))
	return m
}

// Editing reports whether a text field has the keyboard.
func (m ProjectsModel) Editing() bool { return m.query.IsOpen() || m.filter.IsOpen() }

func (m ProjectsModel) Init() tea.Cmd {
	if m.pending == None {
		return nil
	}

	op := m.pending
	return func() tea.Msg { return startMsg{op: op} }
}

// packFor opens the selected server through the package manager.
func packFor(dir string) (*pack.Resolver, error) {
	server, err := pack.Open(dir)
	if err != nil {
		return nil, err
	}

	store, err := cache.Open()
	if err != nil {
		store = cache.Off()
	}

	return &pack.Resolver{
		Client: modrinth.New(store),
		Server: server,
		Cache:  store,
	}, nil
}

// resolve runs a resolution in the background and brings the plan back.
func (m ProjectsModel) resolve(label string, plan func(*pack.Resolver) (pack.Plan, error)) (tea.Model, tea.Cmd) {
	resolver, err := packFor(m.dir())
	if err != nil {
		m.err = err
		return m, nil
	}

	m.resolver = resolver

	updated, tick := m.working(m.list.Cursor, label)
	m.base = updated

	return m, tea.Batch(tick, func() tea.Msg {
		result, err := plan(resolver)
		return plannedMsg{plan: result, err: err}
	})
}

// apply carries out the plan the user has agreed to.
func (m ProjectsModel) apply() (tea.Model, tea.Cmd) {
	resolver, plan := m.resolver, m.plan

	updated, cmd := m.begin(m.list.Cursor, plan.Summary(), func(sc scope.Scope) error {
		return resolver.Server.Apply(context.Background(), plan, pack.Options{
			Cache: resolver.Cache, Scope: sc,
		})
	})

	m.base = updated
	return m.gotoState(stateList), cmd
}

// refresh rereads what the server has, leaving the screen where it is.
func (m ProjectsModel) refresh() ProjectsModel {
	m.installed = nil

	if server, err := pack.Open(m.dir()); err == nil {
		m.installed = server.List(m.kind.kind)
	}

	m.base = m.setCount(m.itemCount())
	return m
}

// reload is what a freshly selected server gets: a different server is a
// different panel.
func (m ProjectsModel) reload() ProjectsModel {
	return m.refresh().gotoState(stateList)
}

// gotoState switches nested screens and resizes the list to the new contents.
func (m ProjectsModel) gotoState(state projectState) ProjectsModel {
	m.state = state
	m.selected = make(map[int]bool)
	m.list = core.List{}
	m.query = m.query.Close()

	if state != stateList {
		m.filter = m.filter.Close()
	}

	m.base = m.setCount(m.itemCount())
	return m
}

// visible is the installed list as the filter leaves it.
func (m ProjectsModel) visible() []pack.Installed {
	needle := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if needle == "" {
		return m.installed
	}

	kept := make([]pack.Installed, 0, len(m.installed))
	for _, entry := range m.installed {
		if strings.Contains(strings.ToLower(entry.Slug), needle) {
			kept = append(kept, entry)
		}
	}
	return kept
}

// itemCount is how many rows the current screen lists.
func (m ProjectsModel) itemCount() int {
	switch m.state {
	case stateRemove:
		return len(m.visible())
	case stateProject:
		return len(projectActions)
	case stateVersions:
		return len(m.choices)
	case stateAdd, statePlan:
		return 0
	default:
		return installedFirst + len(m.visible())
	}
}

// selectableRow reports whether the cursor may rest on a row of the root
// screen: everything but the blank and the heading.
func (m ProjectsModel) selectableRow(index int) bool {
	return index != installedBlank && index != installedHeading
}

// chosenProject is the installed project the cursor is on, if any.
func (m ProjectsModel) chosenProject() string {
	index := m.list.Cursor - installedFirst
	visible := m.visible()

	if m.state != stateList || index < 0 || index >= len(visible) {
		return ""
	}
	return visible[index].Slug
}

func (m ProjectsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.common(msg); handled {
		m.base = updated
		return m, cmd
	}

	switch msg := msg.(type) {
	case startMsg:
		m.pending = None
		return m.runOperation(msg.op)

	case plannedMsg:
		m.busyRow, m.busyLabel = -1, ""

		if msg.err != nil {
			updated, cmd := m.report(msg.err.Error(), true)
			m.base = updated
			return m, cmd
		}

		m.plan = msg.plan
		if msg.plan.Empty() && !msg.plan.Blocked() {
			updated, cmd := m.report("nothing to do", false)
			m.base = updated
			return m.refresh(), cmd
		}

		return m.gotoState(statePlan), nil

	case versionsMsg:
		m.busyRow, m.busyLabel = -1, ""

		if msg.err != nil {
			updated, cmd := m.report(msg.err.Error(), true)
			m.base = updated
			return m, cmd
		}

		m.choices = msg.choices
		return m.gotoState(stateVersions), nil

	case core.OperationDoneMsg:
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
		return m.key(msg)
	}

	return m, nil
}

// key handles a keypress for whichever screen is showing.
func (m ProjectsModel) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.query.IsOpen() {
		return m.typing(msg)
	}

	if m.filter.IsOpen() {
		field, result, cmd := m.filter.HandleKey(msg)
		m.filter = field

		switch result {
		case core.FieldCancelled:
			m.filter = m.filter.Reset()
		}

		m.base = m.setCount(m.itemCount())
		return m, cmd
	}

	if m.state == statePlan {
		switch msg.String() {
		case "enter":
			if m.plan.Blocked() {
				return m.gotoState(stateList), nil
			}
			return m.apply()
		case "esc", "backspace":
			return m.gotoState(stateList), nil
		}
		return m, nil
	}

	if list, handled := m.list.HandleKey(msg.String()); handled {
		m.list = list

		if m.state == stateList {
			direction := 1
			if key := msg.String(); key == "up" || key == "k" || key == "pgup" {
				direction = -1
			}
			m.list = core.Settle(m.list, direction, m.selectableRow)
		}
		return m, nil
	}

	switch msg.String() {
	case "/":
		if m.state == stateList || m.state == stateRemove {
			field, cmd := m.filter.Open(m.filter.Value())
			m.filter = field
			return m, cmd
		}

	case "backspace":
		if m.state != stateList {
			return m.gotoState(stateList), nil
		}
		if m.filter.Value() != "" {
			m.filter = m.filter.Reset()
			m.base = m.setCount(m.itemCount())
			return m, nil
		}
		return m, core.CloseAction()

	case "space":
		if m.state == stateRemove && len(m.visible()) > 0 {
			m.selected[m.list.Cursor] = !m.selected[m.list.Cursor]
		}

	case "ctrl+a":
		if m.state == stateRemove {
			return m.toggleAll(), nil
		}

	case "enter":
		return m.activate()
	}

	return m, nil
}

// typing handles the add screen's field.
func (m ProjectsModel) typing(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "backspace" && m.query.Value() == "" {
		return m.gotoState(stateList), nil
	}

	field, result, cmd := m.query.HandleKey(msg)
	m.query = field

	switch result {
	case core.FieldCommitted:
		return m.activate()

	case core.FieldCancelled:
		// The first esc clears what was typed; with nothing left to clear, the
		// second leaves the screen.
		if m.query.Value() == "" {
			return m.gotoState(stateList), nil
		}
		return m.reopenQuery()
	}

	return m, cmd
}

// reopenQuery puts the keyboard back in the field after a cancel that only
// cleared it.
func (m ProjectsModel) reopenQuery() (tea.Model, tea.Cmd) {
	field, cmd := m.query.Open("")
	m.query = field
	return m, cmd
}

// toggleAll marks or clears every row of the remove screen.
func (m ProjectsModel) toggleAll() ProjectsModel {
	all := true
	for i := range m.visible() {
		if !m.selected[i] {
			all = false
			break
		}
	}

	for i := range m.visible() {
		m.selected[i] = !all
	}
	return m
}

// activate runs whatever the current screen's enter key means.
func (m ProjectsModel) activate() (tea.Model, tea.Cmd) {
	if m.needsServer() {
		return m, nil
	}

	kind := m.kind.kind
	ctx := context.Background()

	switch m.state {
	case statePlan:
		if m.plan.Blocked() {
			return m.gotoState(stateList), nil
		}
		return m.apply()

	case stateProject:
		slug := m.chosen

		switch m.list.Cursor {
		case 0:
			return m.resolve("resolving "+slug, func(r *pack.Resolver) (pack.Plan, error) {
				return r.Update(ctx, kind, []string{slug})
			})

		case 1:
			return m.listVersions(slug)

		case 2:
			entry, _ := m.find(slug)
			if entry.Pinned {
				return m.resolve("unpinning "+slug, func(r *pack.Resolver) (pack.Plan, error) {
					return r.Unpin(ctx, kind, slug)
				})
			}
			return m.resolve("pinning "+slug, func(r *pack.Resolver) (pack.Plan, error) {
				return r.Pin(ctx, kind, slug, "")
			})

		default:
			return m.resolve("resolving "+slug, func(r *pack.Resolver) (pack.Plan, error) {
				return r.Remove(ctx, kind, []string{slug})
			})
		}

	case stateVersions:
		if m.list.Cursor >= len(m.choices) {
			return m, nil
		}

		build := m.choices[m.list.Cursor].VersionNumber
		slug := m.chosen

		return m.resolve("resolving "+slug+"@"+build, func(r *pack.Resolver) (pack.Plan, error) {
			return r.Add(ctx, kind, []string{slug + "@" + build})
		})

	case stateRemove:
		var slugs []string
		for index, marked := range m.selected {
			if marked && index < len(m.visible()) {
				slugs = append(slugs, m.visible()[index].Slug)
			}
		}

		if len(slugs) == 0 {
			return m, nil
		}

		return m.resolve("resolving", func(r *pack.Resolver) (pack.Plan, error) {
			return r.Remove(ctx, kind, slugs)
		})

	case stateAdd:
		query := strings.TrimSpace(m.query.Value())
		if query == "" {
			field, cmd := m.query.Open("")
			m.query = field
			return m, cmd
		}

		return m.resolve("resolving "+query, func(r *pack.Resolver) (pack.Plan, error) {
			return r.Add(ctx, kind, []string{query})
		})
	}

	// The root screen: an operation, or a project.
	if slug := m.chosenProject(); slug != "" {
		m.chosen = slug
		return m.gotoState(stateProject), nil
	}

	switch m.list.Cursor {
	case 0:
		next := m.gotoState(stateAdd)

		field, cmd := next.query.Open("")
		next.query = field
		return next, cmd

	case 1:
		if len(m.installed) == 0 {
			return m, nil
		}
		return m.gotoState(stateRemove), nil

	case 2:
		return m.resolve("resolving", func(r *pack.Resolver) (pack.Plan, error) {
			return r.Sync(ctx, kind)
		})

	case 3:
		return m.resolve("looking for newer builds", func(r *pack.Resolver) (pack.Plan, error) {
			return r.Update(ctx, kind, nil)
		})

	case 4:
		return m.adopt()
	}

	return m, nil
}

// runOperation runs the work the panel was opened for.
func (m ProjectsModel) runOperation(op Operation) (tea.Model, tea.Cmd) {
	kind := m.kind.kind
	ctx := context.Background()

	switch op {
	case Sync:
		return m.resolve("resolving", func(r *pack.Resolver) (pack.Plan, error) {
			return r.Sync(ctx, kind)
		})

	case UpdateAll:
		return m.resolve("looking for newer builds", func(r *pack.Resolver) (pack.Plan, error) {
			return r.Update(ctx, kind, nil)
		})

	case Adopt:
		return m.adopt()
	}

	return m, nil
}

// adopt identifies the jars nobody has recorded and takes them into the lock.
func (m ProjectsModel) adopt() (tea.Model, tea.Cmd) {
	kind := m.kind.kind

	updated, cmd := m.begin(m.list.Cursor, "identifying jars", func(sc scope.Scope) error {
		resolver, err := packFor(sc.Dir)
		if err != nil {
			return err
		}

		adopted, err := resolver.Track(context.Background(), kind)
		if err != nil {
			return err
		}

		if len(adopted) == 0 {
			sc.Logf("every jar is already accounted for")
			return nil
		}
		for _, file := range adopted {
			sc.Logf("%s", file.Line())
		}
		return nil
	})

	m.base = updated
	return m, cmd
}

// listVersions asks what builds a project has for this server.
func (m ProjectsModel) listVersions(slug string) (tea.Model, tea.Cmd) {
	resolver, err := packFor(m.dir())
	if err != nil {
		m.err = err
		return m, nil
	}

	m.resolver, m.chosen = resolver, slug

	updated, tick := m.working(m.list.Cursor, "asking Modrinth")
	m.base = updated

	return m, tea.Batch(tick, func() tea.Msg {
		choices, err := resolver.Versions(context.Background(), m.kind.kind, slug)
		return versionsMsg{choices: choices, err: err}
	})
}

// find returns what the panel knows about one installed project.
func (m ProjectsModel) find(slug string) (pack.Installed, bool) {
	for _, entry := range m.installed {
		if entry.Slug == slug {
			return entry, true
		}
	}
	return pack.Installed{}, false
}

func (m ProjectsModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	switch m.state {
	case statePlan:
		return m.planView()

	case stateAdd:
		body := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Foreground(theme.Inactive).
				Render("Which "+m.kind.noun+"? A Modrinth name, optionally @ a build:"),
			"",
			m.query.View(),
		)
		return m.panelView(m.Title()+" · Add", body, m.Hints()...)

	case stateRemove:
		if len(m.visible()) == 0 {
			return m.emptyView(m.Title()+" · Remove", "Nothing to remove.",
				styles.Hint{Key: "backspace", Desc: "back"})
		}

		return m.panelView(m.Title()+" · Remove", m.filterLine()+m.list.Render(func(i int) string {
			entry := m.visible()[i]

			label := "  " + entry.Slug
			if m.selected[i] {
				label = lipgloss.NewStyle().Foreground(theme.Accent).Render("✓ " + entry.Slug)
			}
			return m.row(i, m.rowMarker(i)+label, entry.VersionNumber)
		}), m.Hints()...)

	case stateProject:
		return m.listView(m.Title()+" · "+m.chosen, func(i int) string {
			label := fmt.Sprintf(projectActions[i], m.chosen)

			if i == 2 {
				if entry, ok := m.find(m.chosen); ok && entry.Pinned {
					label = "Unpin " + m.chosen
				}
			}
			return m.row(i, m.rowMarker(i)+label, "")
		}, m.Hints()...)

	case stateVersions:
		return m.listView(m.Title()+" · "+m.chosen+" builds", func(i int) string {
			choice := m.choices[i]

			mark := choice.Channel
			switch {
			case choice.Installed:
				mark = "installed"
			case i == 0:
				mark = "newest · " + choice.Channel
			}

			return m.row(i, m.rowMarker(i)+choice.VersionNumber, mark)
		}, m.Hints()...)
	}

	return m.rootView()
}

// rootView is the panel's own screen: what can be done, and what is installed.
func (m ProjectsModel) rootView() tea.View {
	visible := m.visible()

	return m.panelView(m.Title(), m.list.Render(func(i int) string {
		switch {
		case i < len(projectOperations):
			return m.row(i, m.rowMarker(i)+strings.ReplaceAll(projectOperations[i], "%s", m.kind.noun), "")

		case i == installedBlank:
			return ""

		case i == installedHeading:
			heading := fmt.Sprintf("INSTALLED (%d)", len(m.installed))
			if m.filter.Value() != "" || m.filter.IsOpen() {
				heading = fmt.Sprintf("INSTALLED (%d of %d)", len(visible), len(m.installed))
			}

			line := lipgloss.NewStyle().Foreground(theme.Inactive).Render(heading)
			if m.filter.IsOpen() {
				return line + "  " + m.filter.View()
			}
			if m.filter.Value() != "" {
				return line + "  " + lipgloss.NewStyle().Foreground(theme.Accent).Render("/"+m.filter.Value())
			}
			return line

		default:
			entry := visible[i-installedFirst]

			name := entry.Slug
			switch {
			case entry.Pinned:
				name += " ·pinned"
			case !entry.Explicit:
				name += " ·needed"
			}
			if !entry.Present {
				name += " ·missing"
			}

			return m.row(i, m.rowMarker(i)+name, entry.VersionNumber)
		}
	}), m.Hints()...)
}

// planView is what a plan looks like before it is agreed to.
func (m ProjectsModel) planView() tea.View {
	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	rows := []string{lipgloss.NewStyle().Foreground(theme.Accent).Render(m.plan.Summary()), ""}

	if m.plan.Blocked() {
		rows = []string{styles.Error(fmt.Errorf("this cannot be applied")), ""}
		for _, conflict := range m.plan.Conflicts {
			rows = append(rows, "  "+conflict)
		}
	}

	for _, change := range m.plan.Changes {
		rows = append(rows, "  "+change.Line())
	}

	if len(m.plan.Optional) > 0 {
		rows = append(rows, "", muted.Render("optional, not installed:"))
		for _, suggestion := range m.plan.Optional {
			rows = append(rows, muted.Render("  "+suggestion.Name()+"  (suggested by "+suggestion.Wanted+")"))
		}
	}

	for _, note := range m.plan.Notes {
		rows = append(rows, muted.Render("  "+note))
	}

	if len(m.plan.Untracked) > 0 {
		rows = append(rows, "", muted.Render("not managed by Osmium: "+strings.Join(m.plan.Untracked, ", ")))
	}

	return m.panelView(m.Title()+" · Plan",
		lipgloss.JoinVertical(lipgloss.Left, rows...), m.Hints()...)
}

// filterLine draws the filter above a list that has one.
func (m ProjectsModel) filterLine() string {
	switch {
	case m.filter.IsOpen():
		return m.filter.View() + "\n"
	case m.filter.Value() != "":
		return lipgloss.NewStyle().Foreground(theme.Accent).Render("/"+m.filter.Value()) + "\n"
	default:
		return ""
	}
}

// Hints are the keys this panel responds to in the state it is in.
func (m ProjectsModel) Hints() []styles.Hint {
	switch m.state {
	case statePlan:
		return []styles.Hint{
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "cancel"},
		}

	case stateAdd:
		return []styles.Hint{
			{Key: "enter", Desc: "resolve"},
			{Key: "esc", Desc: "clear"},
			{Key: "backspace", Desc: "back"},
		}

	case stateRemove:
		return []styles.Hint{
			{Key: "space", Desc: "mark"},
			{Key: "ctrl+a", Desc: "all"},
			{Key: "/", Desc: "filter"},
			{Key: "enter", Desc: "plan"},
			{Key: "backspace", Desc: "back"},
		}

	case stateProject, stateVersions:
		return []styles.Hint{
			{Key: "↑/↓", Desc: "move"},
			{Key: "enter", Desc: "plan"},
			{Key: "backspace", Desc: "back"},
		}
	}

	return []styles.Hint{
		{Key: "↑/↓", Desc: "move"},
		{Key: "enter", Desc: "open"},
		{Key: "/", Desc: "filter"},
		{Key: "backspace", Desc: "back"},
	}
}

func (m ProjectsModel) Title() string { return m.kind.title }

func (m ProjectsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
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
