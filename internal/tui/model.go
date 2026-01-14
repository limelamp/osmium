// Models declared here

package tui

import (
	"os"

	"github.com/charmbracelet/bubbles/textinput"
)

// Setup Model
type SetupModel struct {
	cursor     int
	step       int
	GoBack     bool
	category   string // Vanilla/Simple, Plugin-Based, Mod Loaders, Hybrid
	jarType    string
	jarVersion string
	options    []string
	infoText   string
	textInput  textinput.Model
	err        error
}

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
		options:   []string{"Vanilla/Simple", "Plugin-Based", "Mod Loaders", "Hybrid"},
		infoText:  "Choose the type of server you would like to create:",
		textInput: ti,
	}
}

// Dashboard Model
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

// RunScript Model
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

// RunServer Model
type RunServerModel struct {
	cursor    int
	options   []string
	textInput textinput.Model
	firstRun  bool
	GoBack    bool
	err       error
}

func NewRunServerModel() RunServerModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter server name..."
	ti.Focus() // Start with the cursor blinking inside it
	ti.CharLimit = 500
	ti.Width = 20

	return RunServerModel{
		cursor:    0,
		options:   []string{"Recommended settings", "Detailed"},
		textInput: ti,
		firstRun:  true,
		GoBack:    false,
	}
}

// RemoveFiles Model
type RemoveFilesModel struct {
	cursor   int
	options  map[int]os.DirEntry
	selected map[int]bool
	GoBack   bool
	err      error
}

func NewRemoveFilesModel() RemoveFilesModel {
	entries, _ := os.ReadDir(".")
	options := make(map[int]os.DirEntry)
	for index, value := range entries {
		options[index] = value
	}

	return RemoveFilesModel{
		cursor:   0,
		options:  options,
		selected: make(map[int]bool),
		GoBack:   false,
	}
}

// PluginManagement Model
type PluginManagementModel struct {
	cursor     int
	options    []string
	GoBack     bool
	queryInput textinput.Model
	err        error
}

func NewPluginManagementModel() PluginManagementModel {
	ti := textinput.New()
	ti.Placeholder = "Enter plugin id..."
	ti.Focus() // Start with the cursor blinking inside it
	ti.CharLimit = 20
	ti.Width = 20

	return PluginManagementModel{
		cursor:     0,
		options:    []string{"Recommended settings", "Detailed"},
		GoBack:     false,
		queryInput: ti,
	}
}
