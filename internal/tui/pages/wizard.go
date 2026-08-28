package pages

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// headingRows is how many rows the step heading and its trailing blank take.
const headingRows = 3

// wizard is the shared skeleton of the app's step-by-step pages: the stage
// strip, the numbered heading, the body rectangle a step draws in, and the
// keys that move between steps.
//
// Creating a server and migrating one are the same kind of navigation, so they
// are the same code: a page embeds a wizard, fills in its own steps and hints,
// and inherits the chrome and the movement rules rather than restating them.
type wizard struct {
	layout core.Layout

	// stages names the steps in the progress bar and is the single source of
	// the numbering, so a step's heading can never drift from the position the
	// bar puts it in.
	stages []string
	steps  []steps.Step
	active int
}

// current is the step being shown.
func (w wizard) current() steps.Step {
	return w.steps[w.active]
}

// last reports whether the wizard is on its final step.
func (w wizard) last() bool {
	return w.active == len(w.steps)-1
}

// goTo moves to a step and lays it out, returning the command that initialises
// it — which is what starts the version fetch, the build or the migration.
func (w wizard) goTo(index int) (wizard, tea.Cmd) {
	w.active = min(max(index, 0), len(w.steps)-1)
	w.steps[w.active] = w.steps[w.active].SetLayout(w.bodyLayout())
	return w, w.steps[w.active].Init()
}

// update hands a message to the current step.
func (w wizard) update(msg tea.Msg) (wizard, tea.Cmd) {
	updated, cmd := w.current().Update(msg)
	w.steps[w.active] = updated.(steps.Step)
	return w, cmd
}

// stepHandlesBack lets a step with somewhere of its own to go back to — the
// location browser — take the key first, mirroring how nested navigation works
// everywhere else in the app.
func (w wizard) stepHandlesBack(msg tea.Msg) (bool, wizard, tea.Cmd) {
	location, ok := w.current().(*steps.LocationStep)
	if !ok || !location.Browsing() {
		return false, w, nil
	}

	updated, cmd := w.update(msg)
	return true, updated, cmd
}

// setLayout re-flows every step for a new page rectangle.
func (w wizard) setLayout(l core.Layout) wizard {
	w.layout = l

	body := w.bodyLayout()
	for i, step := range w.steps {
		w.steps[i] = step.SetLayout(body)
	}
	return w
}

// bodyLayout is the rectangle a step may draw its body in, below the progress
// bar and the heading.
func (w wizard) bodyLayout() core.Layout {
	// One row for the progress bar plus its blank, then the heading block.
	top := 2 + headingRows

	return core.Layout{
		X:      w.layout.X + 2,
		Y:      w.layout.Y + 1 + top,
		Width:  w.layout.ContentWidth(),
		Height: max(w.layout.ContentHeight()-top-2, 1),
	}
}

// view draws the page: the stage strip, the step's heading and body, and the
// hint bar pinned to the bottom.
func (w wizard) view(title string, hints string) tea.View {
	step := w.current()

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		w.renderProgressBar(),
		"",
		styles.StepHeading(w.active+1, step.Title(), step.Subtitle()),
		"",
		step.View().Content,
	)

	// Pad the body so the hint bar sits on the panel's last row rather than
	// floating directly under the step.
	body := styles.Body(content, w.layout.ContentWidth(), w.layout.ContentHeight()-1)
	scroll := step.Scroll()

	return tea.NewView(styles.Container(styles.ContainerOpts{
		Width:         w.layout.Width,
		Height:        w.layout.Height,
		Focused:       true,
		Title:         title,
		Content:       body + "\n" + hints,
		ShowHelp:      true,
		ScrollOffset:  scroll.Offset,
		ScrollTotal:   scroll.Total,
		ScrollVisible: scroll.Visible,
	}))
}

// renderProgressBar draws the stage strip across the top, using the active
// theme's colours for the completed, current and pending stages.
func (w wizard) renderProgressBar() string {
	current := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	completed := lipgloss.NewStyle().Foreground(theme.Accent)
	pending := lipgloss.NewStyle().Foreground(theme.Inactive)

	rendered := make([]string, 0, len(w.stages))
	for i, name := range w.stages {
		switch {
		case i == w.active:
			rendered = append(rendered, current.Render(fmt.Sprintf("%d %s", i+1, name)))
		case i < w.active:
			rendered = append(rendered, completed.Render("✓ "+name))
		default:
			rendered = append(rendered, pending.Render(fmt.Sprintf("%d %s", i+1, name)))
		}
	}

	bar := strings.Join(rendered, pending.Render(" › "))
	if lipgloss.Width(bar) <= w.layout.ContentWidth() {
		return bar
	}

	// Narrow terminals get a compact form; wrapping the strip onto a second
	// line would push the step's own content off the bottom of the panel.
	return current.Render(fmt.Sprintf("%d/%d %s", w.active+1, len(w.stages), w.stages[w.active])) +
		pending.Render(w.progressTrack())
}

// progressTrack is the compact bar drawn beside the stage name when the full
// strip does not fit.
func (w wizard) progressTrack() string {
	filled := strings.Repeat("━", w.active+1)
	return "  " + filled + strings.Repeat("─", len(w.stages)-w.active-1)
}

// navHints are the movement hints every wizard shows, with the back key named
// after whichever one works on the step in view.
func (w wizard) navHints() []styles.Hint {
	back := styles.Hint{Key: "backspace", Desc: "back"}
	if w.current().Editing() {
		back = styles.Hint{Key: "shift+tab", Desc: "back"}
	}

	return []styles.Hint{
		{Key: "↑/↓/←/→", Desc: "navigate"},
		{Key: "enter", Desc: "select"},
		back,
	}
}
