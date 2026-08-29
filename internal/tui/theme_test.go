package tui

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// A theme chosen while the app is running has to reach everything drawn, not
// only what happens to be re-rendered afterwards. Anything that keeps its
// colours — a component handed its styles when it was built, a panel holding
// text it has already rendered — shows up here as a colour from the old theme
// still on screen.
func TestThemeChangeReachesEveryPage(t *testing.T) {
	t.Cleanup(func() { theme.SetTheme("Teal") })

	theme.SetTheme("Teal")
	before := theme.Themes["Teal"]

	for _, page := range []string{"Home", "ManageServers", "Settings", "CreateServer", "Migrate"} {
		theme.SetTheme("Teal")

		var app tea.Model = NewAppModel(storage.NewServerStore())
		app, _ = app.Update(tea.WindowSizeMsg{Width: 110, Height: 30})
		app, _ = app.Update(core.ChangePageMsg{Target: page})

		// Something in every console, so the log is rendered and cached.
		app, _ = app.Update(core.LogMsg{Lines: []supervisor.Line{
			{Tag: supervisor.TagServer, Text: "a line of log"},
		}})

		// Under the old theme the page is drawn in the old colours.
		if !strings.Contains(app.View().Content, rgb(before.Primary)) {
			t.Fatalf("%s: the page was not drawn in the theme it started in", page)
		}

		theme.SetTheme("Dracula")
		app, _ = app.Update(core.ThemeChangedMsg{})

		frame := app.View().Content
		for name, stale := range map[string]color.Color{
			"primary":  before.Primary,
			"inactive": before.Inactive,
			"accent":   before.Accent,
		} {
			if strings.Contains(frame, rgb(stale)) {
				t.Errorf("%s: something is still drawn in the old theme's %s colour\n%s",
					page, name, colouredRows(frame, rgb(stale)))
			}
		}
	}
}

// rgb is how a colour appears in a rendered frame.
func rgb(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf(";2;%d;%d;%d", r>>8, g>>8, b>>8)
}

// colouredRows are the rows of a frame that carry a colour, for a failure to
// point at.
func colouredRows(frame, colour string) string {
	var found []string
	for i, row := range strings.Split(frame, "\n") {
		if strings.Contains(row, colour) {
			found = append(found, fmt.Sprintf("%2d|%s", i, regexp.MustCompile("\x1b\\[[0-9;]*m").ReplaceAllString(row, "")))
		}
	}
	return strings.Join(found, "\n")
}
