package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
)

// Setup Model -------------------------------------------------------------------------
type SetupModel struct {
	cursor     int
	step       int
	GoBack     bool
	jarType    string
	jarVersion string
	options    []string
	infoText   string
	textInput  textinput.Model
	err        error
}

// initialized setup model
func NewSetupModel() SetupModel {
	// textInput creating
	ti := textinput.New()
	ti.Placeholder = "Enter server name..."
	ti.Focus() // Start with the cursor blinking inside it
	ti.CharLimit = 20
	ti.Width = 20
	ti.SetValue("false")

	return SetupModel{
		cursor:    0,
		options:   []string{"Vanilla", "Paper", "Purpur"}, //[]string{"Vanilla", "Bukkit", "Spigot", "Paper", "Purpur"},
		infoText:  "Choose the type of server you would like to create:",
		textInput: ti,
	}
}
