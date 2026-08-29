package core

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/limelamp/osmium/internal/tui/theme"
)

// A field is a component from the library, and the library keeps whatever
// styles it was handed when the field was built. The theme can change while
// the app is running, so a field has to be dressed as it is drawn — otherwise
// every text input in the app stays in the theme Osmium was started in.
func TestFieldFollowsTheTheme(t *testing.T) {
	t.Cleanup(func() { theme.SetTheme("Teal") })

	theme.SetTheme("Teal")
	teal := theme.Themes["Teal"]

	for _, state := range []struct {
		name  string
		field Field
	}{
		{name: "being typed into", field: opened()},
		{name: "waiting", field: NewField("type here")},
	} {
		if !strings.Contains(state.field.View(), rgb(teal.Accent)+";") &&
			!strings.Contains(state.field.View(), rgb(teal.Inactive)+"m") &&
			!strings.Contains(state.field.View(), rgb(teal.Primary)+"m") {
			t.Fatalf("a field %s is drawn in no colour of the theme at all:\n%q",
				state.name, state.field.View())
		}

		theme.SetTheme("Dracula")

		after := state.field.View()
		for name, stale := range map[string]color.Color{
			"primary":  teal.Primary,
			"inactive": teal.Inactive,
			"accent":   teal.Accent,
		} {
			if strings.Contains(after, rgb(stale)) {
				t.Errorf("a field %s keeps the old theme's %s colour:\n%q", state.name, name, after)
			}
		}

		theme.SetTheme("Teal")
	}
}

// opened is a field with the keyboard in it, which is what draws the prompt
// and the cursor.
func opened() Field {
	field, _ := NewField("type here").Open("a value")
	return field
}

// rgb is how a colour appears in rendered output.
func rgb(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf(";2;%d;%d;%d", r>>8, g>>8, b>>8)
}
