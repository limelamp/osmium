package pages

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Json structs ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// The "Master Manifest" lists all versions
type VanillaVersionManifest struct {
	Latest struct {
		Release string `json:"release"`
	} `json:"latest"`
	Versions []struct {
		ID  string `json:"id"`  // e.g., "1.21.1"
		URL string `json:"url"` // The link to this version's specific JSON
	} `json:"versions"`
}

// The "Version Specific JSON" contains the actual jar link
type VanillaVersionPackage struct {
	Downloads struct {
		Server struct {
			URL string `json:"url"`
		} `json:"server"`
	} `json:"downloads"`
}

// https://api.papermc.io/v2/projects/paper/versions/{version}/builds/{build}/downloads/{filename}
type PaperBuilds struct {
	Builds []int `json:"builds"` // Build numbers
}

type PaperDownloads struct {
	Downloads struct {
		Application struct {
			Name string `json:"name"`
		} `json:"application"`
	} `json:"downloads"`
}

// General functions ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
func getVanillaServerURL(targetVersion string) (string, error) {
	// Step 1: Get the Master Manifest
	resp, err := http.Get("https://launchermeta.mojang.com/mc/game/version_manifest.json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var manifest VanillaVersionManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return "", err
	}

	// Step 2: Find the specific version in the list
	var versionDetailURL string
	for _, v := range manifest.Versions {
		if v.ID == targetVersion {
			versionDetailURL = v.URL
			break
		}
	}

	if versionDetailURL == "" {
		return "", fmt.Errorf("version %s not found", targetVersion)
	}

	// Step 3: Get the specific version's JSON package
	resp, err = http.Get(versionDetailURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var pkg VanillaVersionPackage
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return "", err
	}

	return pkg.Downloads.Server.URL, nil
}

func getPaperServerURL(version string) (string, error) {
	// Step 1: Get the builds list to find the LATEST build number
	buildsURL := fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s", version)
	resp, err := http.Get(buildsURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var buildsData PaperBuilds
	if err := json.NewDecoder(resp.Body).Decode(&buildsData); err != nil {
		return "", err
	}

	if len(buildsData.Builds) == 0 {
		return "", fmt.Errorf("no builds found for version %s", version)
	}
	latestBuild := buildsData.Builds[len(buildsData.Builds)-1]

	// Step 2: Get the filename for that specific build
	infoURL := fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s/builds/%d", version, latestBuild)
	resp, err = http.Get(infoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var info PaperDownloads
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	filename := info.Downloads.Application.Name

	// Step 3: Construct the final direct download link
	finalURL := fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s/builds/%d/downloads/%s",
		version, latestBuild, filename)

	// Potential shortcut method
	// finalURL := "paper-" + targetVersion + "-" + strconv.Itoa(latestBuild) + ".jar"
	// fmt.Println(finalURL)
	// return finalURL, nil

	return finalURL, nil
}

func downloadFile(url string, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func downloadJar(jarType string, jarVersion string) error {
	// Deciding which url
	url := ""
	var err error
	switch jarType {
	case "Vanilla":
		url, err = getVanillaServerURL(jarVersion)
		if err != nil {
			return err
		}
	// case "Bukkit":
	// 	url = ""
	// case "Spigot":
	// 	url = ""
	case "Paper":
		url, _ = getPaperServerURL(jarVersion)
	case "Purpur":
		url = "https://api.purpurmc.org/v2/purpur/" + jarVersion + "/latest/download"
	}

	output := "server.jar"

	fmt.Println("Downloading the required files....")

	if err := downloadFile(url, "server.jar"); err != nil {
		return err
	}

	fmt.Println("Download finished: ", output)
	return nil
}

// Setup data -------------------------------------------------------------------------
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

// Setup State ----------------------------------------------------------------------------------
// Handles the setup model's data and all actions
func (m SetupModel) Init() tea.Cmd {
	return nil
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.options) - 1
			}
		case "down":
			// assuming 4 options
			if m.cursor < len(m.options)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case "enter":
			switch m.step {
			case 0:
				m.jarType = m.options[m.cursor] // Save the type

				// Now change the options to Versions
				m.step = 1
				m.cursor = 0
				m.options = []string{"1.21.11", "1.21", "1.20", "1.19"}
				m.infoText = "Choose the Minecraft version:"

			case 1: // choose version and begin download
				m.jarVersion = m.options[m.cursor] // Save the version

				// return m, downloadServerJar(m.serverType, m.version)
				if err := downloadJar(m.jarType, m.jarVersion); err != nil {
					m.err = err
					return m, nil
				}

				// Move to download state
				m.step = 2
				m.cursor = 0
				m.options = []string{"Yes", "No, skip to the dashboard"}
				m.infoText = "Would you like to initiliaze the files by running the server once?"
			case 2: // init files
				switch m.cursor {
				case 0:
					m.step = 3
					m.cursor = 0
					m.options = []string{}
					m.infoText = "Do you agree to Mojang's EULA? More info at: https://aka.ms/MinecraftEULA\nPlease type \"true\" in order to agree."
				case 1:
					m.GoBack = true
				}

			case 3:
				switch m.textInput.Value() {
				case "true":
					content := "#By changing the setting below to TRUE you are indicating your agreement to our EULA (https://aka.ms/MinecraftEULA).\n"
					content += "eula=true\n"

					os.WriteFile("eula.txt", []byte(content), 0644)

					// Run the server
					javaCMD := exec.Command(
						"java",
						"-jar",
						"-Xms4G",
						"server.jar",
						"nogui",
					)

					// Run in the same directory
					javaCMD.Dir, _ = os.Getwd()

					// Output stuff
					javaCMD.Stdout = os.Stdout
					javaCMD.Stderr = os.Stderr
					javaCMD.Stdin = os.Stdin

					if err := javaCMD.Run(); err != nil {
						m.err = err
					}
				}
			}
		}

	}

	// IMPORTANT: Update the internal textinput model
	// var cmd tea.Cmd
	m.textInput, _ = m.textInput.Update(msg)
	return m, nil
}

// Basically a big print function huh
func (m SetupModel) View() string {
	// Style your header with Lip Gloss
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#63f456ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM - SERVER INITIALIZATION ") + "\n\n"

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		s += errorStyle.Render("Error: "+m.err.Error()) + "\n\n"
	}

	s += "There appears to be no server initialized in the current folder!" + "\n"
	s += "This setup wizard will be guiding you through the creation of the server." + "\n\n"

	s += "\n\n" + m.infoText + "\n\n"

	switch m.step {
	case 0:

	}
	// Create a simple list
	// serverTypes := [length]string{"Vanilla", "Bukkit", "Spigot", "Paper", "Purpur"}
	if m.step != 3 {
		for i := 0; i < len(m.options); i++ {
			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}
			s += fmt.Sprintf("%s %s\n", cursor, m.options[i])
		}
	} else {
		s += "> eula=" + m.textInput.Value()
	}

	s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit.\n\n"
	return s
}
