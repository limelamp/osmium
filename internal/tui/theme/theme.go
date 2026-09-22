// Package theme is the palette the interface draws with, and the set of themes
// a user can choose between.
//
// Teal is Osmium's own. Every other theme is named after a community colour
// scheme and takes three colours from that scheme's published palette, so that
// someone who already uses it recognises it here. Osmium is not affiliated
// with, endorsed by or connected to any of those projects, and each name
// belongs to the project that made it; NOTICE credits them one by one.
package theme

import (
	"fmt"
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
	"Catppuccin": {
		Name:     "Catppuccin",
		Primary:  lipgloss.Color("#89B4FA"), // Mocha Blue
		Inactive: lipgloss.Color("#585B70"), // Surface 2
		Accent:   lipgloss.Color("#F5C2E7"), // Pink
	},
	"Tokyo Night": {
		Name:     "Tokyo Night",
		Primary:  lipgloss.Color("#7AA2F7"), // Blue
		Inactive: lipgloss.Color("#414868"), // Storm Grey
		Accent:   lipgloss.Color("#BB9AF7"), // Purple
	},
	"Gruvbox": {
		Name:     "Gruvbox",
		Primary:  lipgloss.Color("#83A598"), // Faded Blue
		Inactive: lipgloss.Color("#504945"), // Dark Grey
		Accent:   lipgloss.Color("#FABD2F"), // Bright Yellow
	},
	"Rosé Pine": {
		Name:     "Rosé Pine",
		Primary:  lipgloss.Color("#9CCFD8"), // Foam
		Inactive: lipgloss.Color("#403D52"), // Highlight Med
		Accent:   lipgloss.Color("#EBBCBA"), // Rose
	},
	"Everforest": {
		Name:     "Everforest",
		Primary:  lipgloss.Color("#A7C080"), // Green
		Inactive: lipgloss.Color("#4F5B58"), // Grey 1
		Accent:   lipgloss.Color("#DBBC7F"), // Yellow
	},
	"Kanagawa": {
		Name:     "Kanagawa",
		Primary:  lipgloss.Color("#7E9CD8"), // Crystal Blue
		Inactive: lipgloss.Color("#54546D"), // Sumi Ink 4
		Accent:   lipgloss.Color("#FFA066"), // Surimi Orange
	},
	"Solarized": {
		Name:     "Solarized",
		Primary:  lipgloss.Color("#268BD2"), // Blue
		Inactive: lipgloss.Color("#586E75"), // Base 01
		Accent:   lipgloss.Color("#B58900"), // Yellow
	},
}

// ThemeNames provides an ordered list for UI navigation.
var ThemeNames = []string{
	"Teal", "Dracula", "Nord", "Catppuccin", "Tokyo Night",
	"Gruvbox", "Rosé Pine", "Everforest", "Kanagawa", "Solarized",
}

// Mutable package-level styling hooks.
// Because your rendering helpers retrieve these at runtime, modifying them changes the UI.
var (
	Primary  = Themes["Teal"].Primary
	Inactive = Themes["Teal"].Inactive
	Accent   = Themes["Teal"].Accent

	// Selection is the wash a mouse selection is drawn on. It is the theme's
	// own inactive tone taken down towards black: dark enough that the text it
	// covers keeps its colours and stays readable, distinct enough to show
	// what is selected without a block of accent colour shouting over it.
	Selection = dim(Themes["Teal"].Inactive)

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
	Selection = dim(theme.Inactive)
	CurrentTheme = theme.Name
}

// dim takes a colour halfway to black, keeping its hue.
func dim(c color.Color) color.Color {
	const keep = 0.5

	r, g, b, _ := c.RGBA()
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
		uint8(float64(r>>8)*keep), uint8(float64(g>>8)*keep), uint8(float64(b>>8)*keep)))
}
