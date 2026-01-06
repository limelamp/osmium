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
	stateRunScript
	stateRunServer
	_
	stateRemoveFiles
)

type rootModel struct {
	state       sessionState
	setup       pages.SetupModel     // The setup "page"
	dashboard   pages.DashboardModel // The main dashboard "page"
	runscript   pages.RunScriptModel // Page to create a run_script
	runserver   pages.RunServerModel
	removefiles pages.RemoveFilesModel
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
		m.setup = newSetup.(pages.SetupModel)
		cmd = newCmd

		//Check for the `server.jar` file's existance in the background
		// if _, err := os.Stat("server.jar"); err == nil {
		// 	m.state = stateDashboard
		// }

		// Going back scenario
		if m.setup.GoBack {
			m.setup.GoBack = false
			m.state = stateDashboard
		}

	case stateDashboard:
		newDash, newCmd := m.dashboard.Update(msg)
		m.dashboard = newDash.(pages.DashboardModel)

		// All of the states accessed through the Dashboard here
		switch m.dashboard.CurrentAction {
		case 1:
			m.dashboard.CurrentAction = 0 // Can be safely reset now. The reset is needed for backspacing
			m.state = stateRunScript
		case 2:
			m.dashboard.CurrentAction = 0
			m.state = stateRunServer
		case 4:
			m.dashboard.CurrentAction = 0
			m.state = stateRemoveFiles
		}

		cmd = newCmd

	case stateRunScript:
		newRS, newCmd := m.runscript.Update(msg)
		m.runscript = newRS.(pages.RunScriptModel)

		// Going back scenario
		if m.runscript.GoBack {
			m.runscript.GoBack = false
			m.state = stateDashboard
		}

		cmd = newCmd

	case stateRunServer:
		newRS, newCmd := m.runserver.Update(msg)
		m.runserver = newRS.(pages.RunServerModel)

		cmd = newCmd
	case stateRemoveFiles:
		newRS, newCmd := m.removefiles.Update(msg)
		m.removefiles = newRS.(pages.RemoveFilesModel)

		cmd = newCmd
	}

	return m, cmd
}

func (m rootModel) View() string {
	switch m.state {
	case stateSetup:
		return m.setup.View()
	case stateRunScript:
		return m.runscript.View()
	case stateRunServer:
		return m.runserver.View()
	case stateRemoveFiles:
		return m.removefiles.View()
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

		// Initialize the container with both "pages" set
		mainModel := rootModel{
			state:       initialState,
			setup:       pages.InitializedSetupModel(),     // Setup page for setting up the server if it isn't already
			dashboard:   pages.InitializedDashboardModel(), // Main dashboard page
			runscript:   pages.InitializedRunScriptModel(), // Page for creating a run script
			runserver:   pages.InitializedRunServerModel(),
			removefiles: pages.InitializedRemoveFilesModel(),
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
