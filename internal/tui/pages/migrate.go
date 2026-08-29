package pages

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// MigrateModel moves an existing server to another loader or Minecraft version,
// replacing its jar and every mod and plugin that has a build for the target.
//
// It is a wizard like Create Server, and for the same reason: it asks for an
// engine and a version, then does something long enough to need a live log.
// Both pages are the same skeleton with different steps.
type MigrateModel struct {
	wizard

	store  *storage.ServerStore
	config *steps.Config

	// origin is where esc goes back to. Opened from the Actions pane the server
	// was already chosen, so leaving returns there; opened from the home menu
	// the wizard picked the server itself, so leaving returns home.
	origin string
}

func NewMigrateModel(store *storage.ServerStore) MigrateModel {
	m := MigrateModel{store: store}
	return m.forServer(nil)
}

// forServer rebuilds the wizard for a server, leaving out the picker when the
// caller already knows which server is meant.
func (m MigrateModel) forServer(server *storage.Server) MigrateModel {
	m.config = &steps.Config{}
	m.origin = "Home"

	stages := []string{"Server", "Engine", "Version", "Review", "Migrate"}
	chooser := []steps.Step{steps.NewServerStep(m.config, m.store)}

	if server != nil {
		m.config.Dir = server.Path
		m.config.Name = server.Name

		m.origin = "ManageServers"
		stages, chooser = stages[1:], nil
	}

	m.wizard = wizard{
		layout: m.layout,
		stages: stages,
		steps: append(chooser,
			steps.NewEngineStep(m.config),
			steps.NewVersionStep(m.config),
			steps.NewMigrateReviewStep(m.config),
			steps.NewMigrateStep(m.config),
		),
	}

	return MigrateModel{wizard: m.wizard.setLayout(m.layout), store: m.store, config: m.config, origin: m.origin}
}

// SetServer points the page at a server, which is what the routing request
// carries when Migrate is opened from the Actions pane.
func (m MigrateModel) SetServer(server *storage.Server) MigrateModel {
	return m.forServer(server)
}

func (m MigrateModel) Init() tea.Cmd {
	return m.current().Init()
}

// migrateStep is the page's final step, which runs the migration.
func (m MigrateModel) migrateStep() *steps.MigrateStep {
	return m.steps[len(m.steps)-1].(*steps.MigrateStep)
}

// step moves the wizard and returns the page.
func (m MigrateModel) step(index int) (tea.Model, tea.Cmd) {
	updated, cmd := m.goTo(index)
	m.wizard = updated
	return m, cmd
}

func (m MigrateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case steps.NextMsg:
		return m.step(m.active + 1)

	case steps.PrevMsg:
		return m.step(m.active - 1)

	case tea.KeyPressMsg:
		// A step with a field open keeps the keys that would otherwise
		// navigate, exactly as in the create wizard.
		if m.current().Editing() && (msg.String() == "backspace" || msg.String() == "esc") {
			break
		}

		switch msg.String() {
		// Esc leaves for wherever this page was opened from, as it does in the
		// create wizard.
		case "esc":
			if m.last() && !m.migrateStep().Done() {
				return m, nil
			}
			return m, core.RouteTo(m.origin)

		case "backspace", "shift+tab":
			// A migration that has already started cannot be walked back out
			// of: the files on disk have changed.
			if m.last() && !m.migrateStep().Done() {
				return m, nil
			}
			if m.active > 0 {
				return m.step(m.active - 1)
			}
			return m, core.RouteTo(m.origin)

		case "enter":
			if m.last() && m.migrateStep().Done() {
				return m, tea.Batch(core.ReloadServers(), core.RouteTo("ManageServers"))
			}
		}
	}

	updated, cmd := m.wizard.update(msg)
	m.wizard = updated
	return m, cmd
}

func (m MigrateModel) View() tea.View {
	if m.layout.Width == 0 {
		return tea.NewView("loading...")
	}
	return m.view(m.title(), styles.Hints(m.layout.ContentWidth(), m.hints()...))
}

// title names the panel after the server being migrated, once one is chosen.
func (m MigrateModel) title() string {
	if m.config.Name == "" {
		return "Migrate Server"
	}
	return "Migrate " + m.config.Name
}

// hints describe what the current step responds to.
func (m MigrateModel) hints() []styles.Hint {
	if m.last() {
		if m.migrateStep().Done() {
			return []styles.Hint{
				{Key: "enter", Desc: "manage servers"},
				{Key: "esc", Desc: "home"},
			}
		}
		return []styles.Hint{{Key: "↑/↓", Desc: "scroll log"}}
	}

	return m.navHints()
}

// Help lists the keys of the step in view.
func (m MigrateModel) Help() []core.HelpSection {
	return []core.HelpSection{{
		Title: "Migrate — " + m.current().Title(),
		Hints: m.hints(),
	}}
}

func (m MigrateModel) Title() string {
	return "Migrate Server"
}

func (m MigrateModel) SetLayout(l core.Layout) tea.Model {
	m.wizard = m.wizard.setLayout(l)
	return m
}
