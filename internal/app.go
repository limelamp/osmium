package internal

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/limelamp/osmium/internal/tui"
	"github.com/limelamp/osmium/internal/util"
)

// Bubble tea state data ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// Root data --------------------------------------------------------
type sessionState int

const (
	stateSetup sessionState = iota
	stateDashboard
	stateRunScript
	stateRunServer
	stateManageConfigs
	stateRemoveFiles
	statePluginManagement
	stateModManagement
)

type RootModel struct {
	state            sessionState
	setup            tui.SetupModel     // The setup "page"
	dashboard        tui.DashboardModel // The main dashboard "page"
	runscript        tui.RunScriptModel // Page to create a run_script
	runserver        tui.RunServerModel
	manageconfigs    tui.ManageConfigsModel
	removefiles      tui.RemoveFilesModel
	pluginmanagement tui.PluginManagementModel
	modmanagement    tui.ModManagementModel
}

func NewRootModel() RootModel {
	// Determine the starting state
	initialState := stateSetup
	if util.FindExecutable() != "" {
		initialState = stateDashboard
	}

	// Initialize the container with both "pages" set
	mainModel := RootModel{
		state:            initialState,
		setup:            tui.NewSetupModel(),     // Setup page for setting up the server if it isn't already
		dashboard:        tui.NewDashboardModel(), // Main dashboard page
		runscript:        tui.NewRunScriptModel(), // Page for creating a run script
		runserver:        tui.NewRunServerModel(),
		manageconfigs:    tui.NewManageConfigsModel(),
		removefiles:      tui.NewRemoveFilesModel(),
		pluginmanagement: tui.NewPluginManagementModel(),
		modmanagement:    tui.NewModManagementModel(),
	}

	return mainModel
}

// Bubble Tea States ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// Root state -----------------------------------------------------------------------------------------------
func (m RootModel) Init() tea.Cmd {
	return nil
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state {
	case stateSetup:
		// Pass the message to the setup model
		newSetup, newCmd := m.setup.Update(msg)
		m.setup = newSetup.(tui.SetupModel)
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
		m.dashboard = newDash.(tui.DashboardModel)

		// All of the states accessed through the Dashboard here
		switch m.dashboard.CurrentAction {
		case 1:
			m.state = stateRunScript
		case 2:
			m.state = stateRunServer
		case 3:
			m.state = stateManageConfigs
		case 4:
			m.state = stateRemoveFiles
		case 5:
			m.state = statePluginManagement
		case 6:
			m.state = stateModManagement
		}

		// Can be safely reset now. The reset is needed for backspacing
		m.dashboard.CurrentAction = 0
		cmd = newCmd

	case stateRunScript:
		newRS, newCmd := m.runscript.Update(msg)
		m.runscript = newRS.(tui.RunScriptModel)

		// Going back scenario
		if m.runscript.GoBack {
			m.runscript.GoBack = false
			m.state = stateDashboard
		}

		cmd = newCmd

	case stateRunServer:
		newRS, newCmd := m.runserver.Update(msg)
		m.runserver = newRS.(tui.RunServerModel)

		cmd = newCmd

	case stateManageConfigs:
		newRS, newCmd := m.manageconfigs.Update(msg)
		m.manageconfigs = newRS.(tui.ManageConfigsModel)

		// Going back scenario
		if m.manageconfigs.GoBack {
			m.manageconfigs.GoBack = false
			m.state = stateDashboard
		}

		cmd = newCmd

	case stateRemoveFiles:
		newRS, newCmd := m.removefiles.Update(msg)
		m.removefiles = newRS.(tui.RemoveFilesModel)

		cmd = newCmd

		if m.removefiles.GoBack {
			m.removefiles.GoBack = false
			m.state = stateDashboard
		}

	case statePluginManagement:
		newRS, newCmd := m.pluginmanagement.Update(msg)
		m.pluginmanagement = newRS.(tui.PluginManagementModel)

		cmd = newCmd

		if m.pluginmanagement.GoBack {
			m.pluginmanagement.GoBack = false
			m.state = stateDashboard
		}
	case stateModManagement:
		newRS, newCmd := m.modmanagement.Update(msg)
		m.modmanagement = newRS.(tui.ModManagementModel)

		cmd = newCmd

		if m.modmanagement.GoBack {
			m.modmanagement.GoBack = false
			m.state = stateDashboard
		}
	}

	return m, cmd
}

func (m RootModel) View() string {
	switch m.state {
	case stateSetup:
		return m.setup.View()
	case stateRunScript:
		return m.runscript.View()
	case stateRunServer:
		return m.runserver.View()
	case stateManageConfigs:
		return m.manageconfigs.View()
	case stateRemoveFiles:
		return m.removefiles.View()
	case statePluginManagement:
		return m.pluginmanagement.View()
	case stateModManagement:
		return m.modmanagement.View()
	}

	return m.dashboard.View()
}
