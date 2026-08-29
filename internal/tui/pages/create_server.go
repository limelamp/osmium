package pages

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// CreateServerModel walks the user through creating a server. It owns the
// shared configuration; the chrome and the movement between steps come from the
// wizard it embeds, and the screens themselves live in the steps package.
type CreateServerModel struct {
	wizard

	store  *storage.ServerStore
	config *steps.Config
}

func NewCreateServerModel(store *storage.ServerStore) CreateServerModel {
	cfg, _ := config.LoadConfig()

	wizardConfig := &steps.Config{
		Category: "Vanilla",
		Software: "Vanilla",
		RAM:      cfg.DefaultRAM,
	}

	return CreateServerModel{
		store:  store,
		config: wizardConfig,
		wizard: wizard{
			stages: []string{
				"Location", "Name", "Engine", "Version", "Memory", "Agreements", "Review", "Build",
			},
			steps: []steps.Step{
				steps.NewLocationStep(wizardConfig, cfg.ServersDir()),
				steps.NewNameStep(wizardConfig),
				steps.NewEngineStep(wizardConfig),
				steps.NewVersionStep(wizardConfig),
				steps.NewMemoryStep(wizardConfig),
				steps.NewAgreementsStep(wizardConfig),
				steps.NewReviewStep(wizardConfig),
				steps.NewBuildStep(wizardConfig, store),
			},
		},
	}
}

func (m CreateServerModel) Init() tea.Cmd {
	return m.current().Init()
}

// buildStep is the wizard's final step, which owns the created server record.
func (m CreateServerModel) buildStep() *steps.BuildStep {
	return m.steps[len(m.steps)-1].(*steps.BuildStep)
}

// step moves the wizard and returns the page, keeping the embedded wizard's
// result in the page it belongs to.
func (m CreateServerModel) step(index int) (tea.Model, tea.Cmd) {
	updated, cmd := m.goTo(index)
	m.wizard = updated
	return m, cmd
}

func (m CreateServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case steps.NextMsg:
		return m.step(m.active + 1)

	case steps.PrevMsg:
		return m.step(m.active - 1)

	case tea.KeyPressMsg:
		// A step with an open text field keeps backspace and esc, so typing
		// into it can never walk the wizard backwards.
		if m.current().Editing() && (msg.String() == "backspace" || msg.String() == "esc") {
			break
		}

		switch msg.String() {
		// Esc leaves. It is the way out of the wizard from wherever you are —
		// including out of the folder browser — and the guard above means it
		// only ever gets here when nothing is being typed.
		case "esc":
			return m, core.RouteTo("Home")

		// Backspace and shift+tab step back one screen. A step with somewhere
		// of its own to go — the folder browser — takes the key first.
		case "backspace", "shift+tab":
			if handled, updated, cmd := m.stepHandlesBack(msg); handled {
				m.wizard = updated
				return m, cmd
			}
			if m.active > 0 {
				return m.step(m.active - 1)
			}
			return m, core.RouteTo("Home")

		case "enter":
			// Finishing the wizard opens Manage Servers, booting the new server
			// first when that was asked for.
			if m.last() && m.buildStep().Done() && !m.buildStep().Failed() {
				// The wizard is left behind as a fresh one: coming back to
				// Create Server should start a new server, not sit on the
				// finished build of the last one.
				return m.reset(), m.finish()
			}
		}
	}

	updated, cmd := m.wizard.update(msg)
	m.wizard = updated
	return m, cmd
}

// reset returns the wizard to its first step with empty choices, reloading the
// defaults from settings in case they changed while it was open.
func (m CreateServerModel) reset() tea.Model {
	return NewCreateServerModel(m.store).SetLayout(m.layout)
}

// finish leaves the wizard for Manage Servers, starting the server on the way
// when the user asked for it. The commands run in order so the page is already
// showing before the start request reaches it.
func (m CreateServerModel) finish() tea.Cmd {
	if m.config.AutoRun {
		return tea.Sequence(
			core.RouteTo("ManageServers"),
			core.ControlServer(core.ServerStart, m.buildStep().Server()),
		)
	}
	return core.RouteTo("ManageServers")
}

func (m CreateServerModel) View() tea.View {
	if m.layout.Width == 0 {
		return tea.NewView("loading...")
	}
	return m.view("Setup New Server", styles.Hints(m.layout.ContentWidth(), m.hints()...))
}

// hints describe what the current step responds to.
func (m CreateServerModel) hints() []styles.Hint {
	if m.last() {
		if m.buildStep().Done() {
			return []styles.Hint{
				{Key: "enter", Desc: "manage servers"},
				{Key: "backspace", Desc: "back"},
			}
		}
		return []styles.Hint{{Key: "↑/↓", Desc: "scroll log"}}
	}

	return m.navHints()
}

// Help lists the keys of the step in view.
func (m CreateServerModel) Help() []core.HelpSection {
	return []core.HelpSection{{
		Title: "Create Server — " + m.current().Title(),
		Hints: m.hints(),
	}}
}

func (m CreateServerModel) Title() string {
	return "Create Server"
}

func (m CreateServerModel) SetLayout(l core.Layout) tea.Model {
	m.wizard = m.wizard.setLayout(l)
	return m
}
