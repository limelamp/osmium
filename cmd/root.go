package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// General functions ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
func downloadFile(url string, filename string) {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("bad status: %s", resp.Status))
	}

	out, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		panic(err)
	}
}

func downloadJar(jarType string, jarVersion string) {
	// Deciding which url
	url := ""
	switch jarType {
	case "Vanilla":
		url = "https://piston-data.mojang.com/v1/objects/64bb6d763bed0a9f1d632ec347938594144943ed/server.jar"
	case "Bukkit":
		url = ""
	case "Spigot":
		url = ""
	case "Paper":
		url = "https://api.papermc.io/v2/projects/paper/versions/" + jarVersion
	case "Purpur":
		url = "https://api.purpurmc.org/v2/purpur/" + jarVersion + "/latest/download"
	}

	output := "server.jar"

	fmt.Println("Downloading the required files....")

	downloadFile(url, "server.jar")

	fmt.Println("Download finished: ", output)
}

// Bubble tea state data ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// Root data --------------------------------------------------------
type sessionState int

const (
	stateSetup sessionState = iota
	stateDashboard
)

type rootModel struct {
	state     sessionState
	setup     tea.Model // The setup "scene"
	dashboard tea.Model // The main dashboard "scene"
}

// Dashboard data --------------------------------------------------------------------
// Dashboard dashboardModel datatype to store all the dashboard state/data.
type dashboardModel struct {
	cursor int
}

// initialized dashboard model
func initializedDashboardModel() dashboardModel {
	return dashboardModel{cursor: 0}
}

// Setup data -------------------------------------------------------------------------
type setupModel struct {
	cursor     int
	step       int
	jarType    string
	jarVersion string
	options    []string
}

// initialized dashboard model
func initializedSetupModel() setupModel {
	return setupModel{
		cursor:  0,
		options: []string{"Vanilla", "Bukkit", "Spigot", "Paper", "Purpur"},
	}
}

// Bubble Tea States ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// Root state -----------------------------------------------------------------------------------------------
func (m rootModel) Init() tea.Cmd {
	return nil
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state {
	case stateSetup:
		// Pass the message to the setup model
		newSetup, newCmd := m.setup.Update(msg)
		m.setup = newSetup
		cmd = newCmd

		//Check for the `server.jar` file's existance in the background
		if _, err := os.Stat("server.jar"); err == nil {
			m.state = stateDashboard
		}

	case stateDashboard:
		newDash, newCmd := m.dashboard.Update(msg)
		m.dashboard = newDash
		cmd = newCmd
	}

	return m, cmd
}

func (m rootModel) View() string {
	if m.state == stateSetup {
		return m.setup.View()
	}
	return m.dashboard.View()
}

// Setup State ----------------------------------------------------------------------------------
// Handles the setup model's data and all actions
func (m setupModel) Init() tea.Cmd {
	return nil
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

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
			} // assuming 4 options
		case "enter":
			switch m.step {
			case 0:
				m.jarType = m.options[m.cursor] // Save the type

				// Now change the options to Versions
				m.step = 1
				m.cursor = 0
				m.options = []string{"1.21.11", "1.21", "1.20", "1.19"}

			case 1:
				m.jarVersion = m.options[m.cursor] // Save the version

				// Move to download state
				m.step = 2
				// return m, downloadServerJar(m.serverType, m.version)
			case 2:
				downloadJar(m.jarType, m.jarVersion)
			}
		}

	}
	return m, nil
}

// Basically a big print function huh
func (m setupModel) View() string {
	// Style your header with Lip Gloss
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#63f456ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM SERVER INITIALIZATION ") + "\n\n"
	s += "There appears to be no server initialized in the current folder!" + "\n"
	s += "This setup wizard will be guiding you through the creation of the server." + "\n\n"

	switch m.step {
	case 0:

	}
	// Create a simple list
	// serverTypes := [length]string{"Vanilla", "Bukkit", "Spigot", "Paper", "Purpur"}
	for i := 0; i < len(m.options); i++ {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		s += fmt.Sprintf("%s %s\n", cursor, m.options[i])
	}

	s += "\n\n" + "Navigate using arrow keys. Press 'q' to exit.\n\n"
	return s
}

// Dashboard State --------------------------------------------------------------------------------------------------------
// Handles the dashboard model's data and all actions
func (m dashboardModel) Init() tea.Cmd {
	return nil
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < 5 {
				m.cursor++
			} // assuming 5 options
		}
	}
	return m, nil
}

// Basically a big print function huh
func (m dashboardModel) View() string {
	// Style your header with Lip Gloss
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#c256f4ff")).
		Padding(0, 1)

	s := headerStyle.Render(" OSMIUM DASHBOARD ") + "\n\n"
	s += "Navigate using arrow keys. Press 'q' to exit.\n\n"

	// Create a simple list
	for i := 0; i < 6; i++ {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		s += fmt.Sprintf("%s Option %d\n", cursor, i)
	}

	return s
}

// Cobra and CLI stuff ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "osmium",
	Short: "A full-screen TUI app for managing minecraft servers.",
	Run: func(cmd *cobra.Command, args []string) {

		// Determine the starting state
		initialState := stateDashboard
		if _, err := os.Stat("server.jar"); os.IsNotExist(err) {
			fmt.Println("No server.jar found! Starting setup...")
			initialState = stateSetup
		}

		// Initialize the container with both "scenes" set
		mainModel := rootModel{
			state:     initialState,
			setup:     initializedSetupModel(),
			dashboard: initializedDashboardModel(),
		}

		mainProcess := tea.NewProgram(mainModel, tea.WithAltScreen())
		if _, err := mainProcess.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.osmium.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
