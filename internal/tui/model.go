// Models declared here

package tui

import (
	"bytes"
	"io"
	"os"
	"os/exec"

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
	ti.Placeholder = "Type \"true\" to accept..."
	ti.Focus()     // Start with the cursor blinking inside it
	ti.Prompt = "" // Remove the ">" out of the way
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
		options:       []string{"Create a run script", "Run the server", "Manage server configurations", "Remove selected server files", "Plugin Management", "Mod Managament"},
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
	javaCMD   *exec.Cmd
	output    *bytes.Buffer // The "bucket" for logs
	inputPipe io.WriteCloser
	GoBack    bool
	err       error
}

func NewRunServerModel() RunServerModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter a command..."
	ti.Focus()     // Start with the cursor blinking inside it
	ti.Prompt = "" // Remove the ">" out of the way
	ti.CharLimit = 500
	ti.Width = 20

	return RunServerModel{
		cursor:    0,
		options:   []string{"Recommended settings", "Detailed"},
		textInput: ti,
		firstRun:  true,
		output:    &bytes.Buffer{},
		GoBack:    false,
	}
}

// ManageConfigsModel
type ManageConfigsModel struct {
	cursor             int
	fileType           string
	fileName           string
	step               int
	selected           int
	options            []string
	configOptionKeys   []string
	configOptionValues []string
	textInput          textinput.Model
	GoBack             bool
	topItem            int // The index of the first item currently visible
	viewHeight         int // How many items to show at once
	err                error
}

func NewManageConfigsModel() ManageConfigsModel {
	// textInput init
	ti := textinput.New()
	ti.Placeholder = "Enter a value..."
	ti.Focus()     // Start with the cursor blinking inside it
	ti.Prompt = "" // Remove the ">" out of the way
	ti.CharLimit = 500
	ti.Width = 20

	return ManageConfigsModel{
		cursor:     0,
		step:       0,
		selected:   -1,
		options:    []string{"server.properties", "bukkit.yml", "spigot.yml", "config/paper-global.yml", "config/paper-world-defaults.yml", "purpur.yml"},
		textInput:  ti,
		GoBack:     false,
		viewHeight: 40,
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
