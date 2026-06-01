package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme holds the color definitions for a specific UI look.
type Theme struct {
	Name     string
	Primary  color.Color
	Inactive color.Color
	Accent   color.Color
}

// Themes holds the list of pre-configured schemes.
var Themes = map[string]Theme{
	"Teal": {
		Name:     "Teal",
		Primary:  lipgloss.Color("#86C7C1"),
		Inactive: lipgloss.Color("#2F6D66"),
		Accent:   lipgloss.Color("#F2C94C"),
	},
	"Dracula": {
		Name:     "Dracula",
		Primary:  lipgloss.Color("#BD93F9"), // Purple
		Inactive: lipgloss.Color("#44475A"), // Dark Comment Grey
		Accent:   lipgloss.Color("#FF79C6"), // Pink
	},
	"Nord": {
		Name:     "Nord",
		Primary:  lipgloss.Color("#88C0D0"), // Frost Blue
		Inactive: lipgloss.Color("#4C566A"), // Polar Night
		Accent:   lipgloss.Color("#EBCB8B"), // Nord Yellow
	},
}

// ThemeNames provides an ordered list for UI navigation.
var ThemeNames = []string{"Teal", "Dracula", "Nord"}

// Mutable package-level styling hooks.
// Because your rendering helpers retrieve these at runtime, modifying them changes the UI.
var (
	Primary      = Themes["Teal"].Primary
	Inactive     = Themes["Teal"].Inactive
	Accent       = Themes["Teal"].Accent
	CurrentTheme = "Teal"
)

// SetTheme updates the global colors dynamically.
func SetTheme(themeName string) {
	theme, exists := Themes[themeName]
	if !exists {
		theme = Themes["Teal"]
	}
	Primary = theme.Primary
	Inactive = theme.Inactive
	Accent = theme.Accent
	CurrentTheme = theme.Name
}
