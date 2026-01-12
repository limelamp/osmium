package pages

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The structure of the Modrinth Version API response
type ModrinthVersion struct {
	Files []struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	} `json:"files"`
}

// Black magic
func DownloadPluginByID(projectID string, mcVersion string) error {
	client := &http.Client{}

	// 1. Get the list of versions for this specific project
	// Filter by game_versions to make sure we get the right one
	url := fmt.Sprintf("https://api.modrinth.com/v2/project/%s/version?game_versions=[\"%s\"]", projectID, mcVersion)
	//https://api.modrinth.com/v2/project/skinrestorer/version?game_versions="1.21.11"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Osmium-Manager/1.0") // Modrinth REQUIRES this

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var versions []ModrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return err
	}

	if len(versions) == 0 {
		return fmt.Errorf("no compatible versions found for %s", mcVersion)
	}

	// 2. Pick the first file from the newest version
	fileInfo := versions[0].Files[0]
	fmt.Printf("Downloading %s...\n", fileInfo.Filename)

	// 3. Download the actual .jar
	fileResp, err := http.Get(fileInfo.URL)
	if err != nil {
		return err
	}
	defer fileResp.Body.Close()

	// Ensure the plugins folder exists
	os.MkdirAll("plugins", 0755)

	out, err := os.Create("plugins/" + fileInfo.Filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, fileResp.Body)
	return err
}

func GetInstalledVersion() string {
	// 1. Point to the "versions" folder relative to where the app is running
	path := "versions"

	// 2. Read all files/folders inside
	entries, err := os.ReadDir(path)
	if err != nil {
		return "" //, err
	}

	// 3. Loop through to find the first directory
	for _, entry := range entries {
		if entry.IsDir() {
			// Return the name of the folder (e.g., "1.21.11")
			return entry.Name() //, nil
		}
	}

	return ""
}

// Data --------------------------------------------------------------------
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

// State --------------------------------------------------------------------------------------------------------
// Handles the dashboard model's data and all actions
func (m PluginManagementModel) Init() tea.Cmd {
	return nil
}

func (m PluginManagementModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "shift+backspace":
			m.GoBack = true
			return m, nil
		case "enter":
			DownloadPluginByID(m.queryInput.Value(), GetInstalledVersion())
			fmt.Println("Downloaded! Good Luck lol")
		}
	}
	var cmd tea.Cmd
	m.queryInput, cmd = m.queryInput.Update(msg)
	return m, cmd
}

func (m PluginManagementModel) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#05fae6ff")).
		Padding(0, 1)

	// Header
	s := headerStyle.Render(" OSMIUM - PLUGIN MANAGEMENT") + "\n\n"

	// Error display
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		s += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	// Create a simple list
	for i := 0; i < len(m.options); i++ {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		s += fmt.Sprintf("%s %s\n", cursor, m.options[i])
	}

	s += m.queryInput.View() + "\n\n"

	s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit, 'backspace' to go back.\n\n"
	return s
}
