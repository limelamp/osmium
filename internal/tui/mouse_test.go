package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/limelamp/osmium/internal/tui/components"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/pages"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// A click has to land on the row the user actually sees, which means every
// component's hit-testing has to agree with its own rendering to the row.
//
// These tests click on the row a label was drawn on — found in the rendered
// output rather than assumed — and check that the component selected that
// label. That is the one thing a coordinate mistake cannot survive, whether it
// comes from a container border, a spacer or a heading.

// screen renders a view at a layout and returns its rows with the styling
// stripped, so a row can be found by the text on it.
func screen(content string) []string {
	rows := strings.Split(content, "\n")
	for i, row := range rows {
		rows[i] = ansi.Strip(row)
	}
	return rows
}

// rowOf finds the screen row a label was drawn on, as an absolute Y for the
// layout the view was rendered at.
func rowOf(t *testing.T, rows []string, layout core.Layout, label string) int {
	t.Helper()

	for i, row := range rows {
		if strings.Contains(row, label) {
			return layout.Y + i
		}
	}

	t.Fatalf("%q was not drawn:\n%s", label, strings.Join(rows, "\n"))
	return -1
}

// click sends a left click at an absolute position.
func click(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func TestHomeMenuClickSelectsTheRowUnderThePointer(t *testing.T) {
	layout := core.Layout{Width: 120, Height: 30}

	home := pages.NewHomeModel().SetLayout(layout).(pages.HomeModel)
	rows := screen(home.View().Content)

	for _, entry := range []string{"Manage servers", "Migrate server", "Settings", "Quit"} {
		y := rowOf(t, rows, layout, entry)

		updated, _ := home.Update(click(40, y))
		selected := screen(updated.(pages.HomeModel).View().Content)

		if want := "> " + entry; !containsRow(selected, want) {
			t.Errorf("clicking row %d (%s) did not select it:\n%s", y, entry, strings.Join(selected, "\n"))
		}
	}
}

func TestServersListClickSelectsTheRowUnderThePointer(t *testing.T) {
	layout := core.Layout{X: 0, Y: 1, Width: 40, Height: 12}

	servers := components.NewServersModel(storage.NewServerStore()).SetLayout(layout).SetFocus(true)
	loaded, _ := servers.Update(core.LoadedServersMsg{Servers: []storage.Server{
		{Name: "Alpha", Path: "/tmp/alpha"},
		{Name: "Beta", Path: "/tmp/beta"},
		{Name: "Gamma", Path: "/tmp/gamma"},
	}})
	servers = loaded.(components.ServersModel)

	rows := screen(servers.View().Content)
	y := rowOf(t, rows, layout, "Gamma")

	updated, _ := servers.Update(click(layout.X+3, y))
	if selected := updated.(components.ServersModel).Selected(); selected == nil || selected.Name != "Gamma" {
		t.Fatalf("clicking Gamma's row (y=%d) selected %v", y, selected)
	}
}

func TestActionsMenuClickSkipsHeadingsAndSelectsEntries(t *testing.T) {
	layout := core.Layout{X: 40, Y: 1, Width: 60, Height: 20}

	actions := components.NewActionsModel(storage.NewServerStore()).
		SetLayout(layout).SetFocus(true).(components.ActionsModel)

	rows := screen(actions.View().Content)
	y := rowOf(t, rows, layout, "Restart server")

	updated, _ := actions.Update(click(layout.X+4, y))
	after := screen(updated.(components.ActionsModel).View().Content)

	if !containsRow(after, "➔ Restart server") {
		t.Errorf("clicking Restart server (y=%d) did not select it:\n%s", y, strings.Join(after, "\n"))
	}

	// A heading is not selectable, so clicking one must leave the cursor alone.
	heading := rowOf(t, rows, layout, "CONTENT")
	unchanged, _ := updated.(components.ActionsModel).Update(click(layout.X+4, heading))

	if !containsRow(screen(unchanged.(components.ActionsModel).View().Content), "➔ Restart server") {
		t.Error("clicking a group heading moved the cursor")
	}
}

func TestWizardStepClickSelectsTheRowUnderThePointer(t *testing.T) {
	config := &steps.Config{}

	// The wizard hands its steps a body rectangle rather than a container, so
	// these are the components whose hit-testing must *not* allow for a border.
	body := core.Layout{X: 2, Y: 7, Width: 60, Height: 12}

	memory := steps.NewMemoryStep(config).SetLayout(body)
	rows := screen(memory.View().Content)
	y := rowOf(t, rows, body, "8G")

	updated, _ := memory.Update(click(body.X+2, y))
	if !containsRow(screen(updated.(steps.Step).View().Content), "➔ 8G") {
		t.Errorf("clicking 8G (y=%d) did not select it:\n%s", y, strings.Join(rows, "\n"))
	}

	location := steps.NewLocationStep(config, "/tmp").SetLayout(body)
	rows = screen(location.View().Content)
	y = rowOf(t, rows, body, "Default folder")

	updated, _ = location.Update(click(body.X+2, y))
	if !containsRow(screen(updated.(steps.Step).View().Content), "➔ Default folder") {
		t.Errorf("clicking Default folder (y=%d) did not select it:\n%s", y, strings.Join(rows, "\n"))
	}
}

func TestSettingsClickSelectsTheRowUnderThePointer(t *testing.T) {
	layout := core.Layout{X: 0, Y: 1, Width: 100, Height: 20}

	settings := pages.NewSettingsModel().SetLayout(layout).(pages.SettingsModel)
	rows := screen(settings.View().Content)

	y := rowOf(t, rows, layout, "Defaults")
	updated, _ := settings.Update(click(4, y))
	after := screen(updated.(pages.SettingsModel).View().Content)

	if !containsRow(after, "➔ Defaults") {
		t.Errorf("clicking the Defaults section (y=%d) did not select it:\n%s", y, strings.Join(after, "\n"))
	}
}

// containsRow reports whether any row holds the text.
func containsRow(rows []string, text string) bool {
	for _, row := range rows {
		if strings.Contains(row, text) {
			return true
		}
	}
	return false
}

// The page is drawn inside the app, under a title bar that only some pages
// have. A click is answered against the layout the app handed the page, so the
// two have to agree about where the page starts — this drives the whole app so
// that a row spent on the chrome cannot go unnoticed.
func TestAppClickReachesTheRowUnderThePointer(t *testing.T) {
	app := NewAppModel(storage.NewServerStore())

	var model tea.Model = app
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	rows := screen(model.View().Content)
	y := rowOf(t, rows, core.Layout{}, "Migrate server")

	model, _ = model.Update(click(40, y))
	if after := screen(model.View().Content); !containsRow(after, "> Migrate server") {
		t.Errorf("clicking Migrate server on screen row %d did not select it:\n%s",
			y, strings.Join(after, "\n"))
	}
}
