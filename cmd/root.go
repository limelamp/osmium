package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/limelamp/osmium/cmd/pages"
	"github.com/spf13/cobra"
)

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
			setup:     pages.InitializedSetupModel(),
			dashboard: pages.InitializedDashboardModel(),
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
