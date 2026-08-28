package pages

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// wizardStages names the steps in the progress bar. It is the single source of
// the step numbering, so the heading a step shows can never drift from the
// position the progress bar puts it in.
var wizardStages = []string{
	"Location", "Name", "Engine", "Version", "Memory", "Agreements", "Review", "Build",
}

// headingRows is how many rows the step heading and its trailing blank take.
const headingRows = 3

// CreateServerModel walks the user through creating a server. It owns the step
// order, the progress bar and the shared configuration; the screens themselves
// live in the steps package.
type CreateServerModel struct {
	layout core.Layout

	config *steps.Config
	steps  []steps.Step
	active int
}

func NewCreateServerModel(store *storage.ServerStore) CreateServerModel {
	cfg, _ := config.LoadConfig()

	wizardConfig := &steps.Config{
		Category: "Vanilla",
		Software: "Vanilla",
		RAM:      cfg.DefaultRAM,
	}

	return CreateServerModel{
		config: wizardConfig,
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
	}
}

func (m CreateServerModel) Init() tea.Cmd {
	return m.steps[m.active].Init()
}

// current is the step being shown.
func (m CreateServerModel) current() steps.Step {
	return m.steps[m.active]
}

// buildStep is the wizard's final step, which owns the created server record.
func (m CreateServerModel) buildStep() *steps.BuildStep {
	return m.steps[len(m.steps)-1].(*steps.BuildStep)
}

// isBuilding reports whether the wizard is on the build step.
func (m CreateServerModel) isBuilding() bool {
	return m.active == len(m.steps)-1
}

// goTo moves to a step and initialises it, which is what starts the version
// fetch and the build.
func (m CreateServerModel) goTo(index int) (tea.Model, tea.Cmd) {
	m.active = min(max(index, 0), len(m.steps)-1)
	m.steps[m.active] = m.steps[m.active].SetLayout(m.bodyLayout())
	return m, m.steps[m.active].Init()
}

func (m CreateServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case steps.NextMsg:
		return m.goTo(m.active + 1)

	case steps.PrevMsg:
		return m.goTo(m.active - 1)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, core.RouteTo("Home")

		case "backspace":
			// A step handles backspace itself while it has somewhere of its own
			// to go back to; only then does it fall through to the wizard.
			if handled, model, cmd := m.stepHandlesBack(msg); handled {
				return model, cmd
			}
			if m.active > 0 {
				return m.goTo(m.active - 1)
			}
			return m, core.RouteTo("Home")

		case "enter":
			// Finishing the wizard opens Manage Servers, booting the new server
			// first when that was asked for.
			if m.isBuilding() && m.buildStep().Done() && !m.buildStep().Failed() {
				return m, m.finish()
			}
		}
	}

	var cmd tea.Cmd
	updated, cmd := m.current().Update(msg)
	m.steps[m.active] = updated.(steps.Step)
	return m, cmd
}

// stepHandlesBack lets the location browser collapse before the wizard steps
// backwards, mirroring how nested navigation works elsewhere in the app.
func (m CreateServerModel) stepHandlesBack(msg tea.Msg) (bool, tea.Model, tea.Cmd) {
	location, ok := m.current().(*steps.LocationStep)
	if !ok || !location.Browsing() {
		return false, m, nil
	}

	updated, cmd := location.Update(msg)
	m.steps[m.active] = updated.(steps.Step)
	return true, m, cmd
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

	step := m.current()

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderProgressBar(),
		"",
		styles.StepHeading(m.active+1, step.Title(), step.Subtitle()),
		"",
		step.View().Content,
	)

	// Pad the body so the hint bar sits on the panel's last row rather than
	// floating directly under the step.
	body := styles.Body(content, m.layout.ContentHeight()-1)
	scroll := step.Scroll()

	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:         m.layout.Width,
		Height:        m.layout.Height,
		Focused:       true,
		Title:         "Setup New Server",
		Content:       body + "\n" + m.hints(),
		ShowHelp:      true,
		ScrollOffset:  scroll.Offset,
		ScrollTotal:   scroll.Total,
		ScrollVisible: scroll.Visible,
	}))
}

// hints describe what the current step responds to.
func (m CreateServerModel) hints() string {
	width := m.layout.ContentWidth()

	if m.isBuilding() {
		if m.buildStep().Done() {
			return styles.Hints(width,
				styles.Hint{Key: "enter", Desc: "manage servers"},
				styles.Hint{Key: "backspace", Desc: "back"},
			)
		}
		return styles.Hints(width, styles.Hint{Key: "↑/↓", Desc: "scroll log"})
	}

	return styles.Hints(width,
		styles.Hint{Key: "↑/↓/←/→", Desc: "navigate"},
		styles.Hint{Key: "enter", Desc: "select"},
		styles.Hint{Key: "backspace", Desc: "back"},
	)
}

// renderProgressBar draws the stage strip across the top of the wizard, using
// the active theme's colours for the completed, current and pending stages.
func (m CreateServerModel) renderProgressBar() string {
	current := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	completed := lipgloss.NewStyle().Foreground(theme.Accent)
	pending := lipgloss.NewStyle().Foreground(theme.Inactive)

	rendered := make([]string, 0, len(wizardStages))
	for i, name := range wizardStages {
		switch {
		case i == m.active:
			rendered = append(rendered, current.Render(fmt.Sprintf("%d %s", i+1, name)))
		case i < m.active:
			rendered = append(rendered, completed.Render("✓ "+name))
		default:
			rendered = append(rendered, pending.Render(fmt.Sprintf("%d %s", i+1, name)))
		}
	}

	bar := strings.Join(rendered, pending.Render(" › "))
	if lipgloss.Width(bar) <= m.layout.ContentWidth() {
		return bar
	}

	// Narrow terminals get a compact form; wrapping the strip onto a second
	// line would push the step's own content off the bottom of the panel.
	return current.Render(fmt.Sprintf("%d/%d %s", m.active+1, len(wizardStages), wizardStages[m.active])) +
		pending.Render(m.progressTrack())
}

// progressTrack is the compact bar drawn beside the stage name when the full
// strip does not fit.
func (m CreateServerModel) progressTrack() string {
	filled := strings.Repeat("━", m.active+1)
	return "  " + filled + strings.Repeat("─", len(wizardStages)-m.active-1)
}

// bodyLayout is the rectangle a step may draw its body in, below the progress
// bar and the heading.
func (m CreateServerModel) bodyLayout() core.Layout {
	// One row for the progress bar plus its blank, then the heading block.
	top := 2 + headingRows

	return core.Layout{
		X:      m.layout.X + 2,
		Y:      m.layout.Y + 1 + top,
		Width:  m.layout.ContentWidth(),
		Height: max(m.layout.ContentHeight()-top-2, 1),
	}
}

func (m CreateServerModel) Title() string {
	return "Create Server"
}

func (m CreateServerModel) SetLayout(l core.Layout) tea.Model {
	m.layout = l

	body := m.bodyLayout()
	for i, step := range m.steps {
		m.steps[i] = step.SetLayout(body)
	}
	return m
}
