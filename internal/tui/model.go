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

// Dashboard Model --------------------------------------------------------------------
type DashboardModel struct {
	cursor        int
	options       []string
	CurrentAction int
}

func NewDashboardModel() DashboardModel {
	return DashboardModel{
		cursor:        0,
		options:       []string{"Create a run script", "Run the server", "Manage server properties", "Remove all server files", "Plugin Management", "Hi", "My", "Name", "Is", "Edwin", "And", "I", "Made", "The", "Mimic"},
		CurrentAction: 0,
	}
}

// RunScript Model --------------------------------------------------------------------
type RunScriptModel struct {
	cursor  int
	options []string
	GoBack  bool
	err     error
}

func NewRunScriptModel() RunScriptModel {
	return RunScriptModel{
		cursor:  0,
		options: []string{"Recommended settings", "Detailed"},
		GoBack:  false,
	}
}
