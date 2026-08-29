package core

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// ChangePageMsg is sent by child pages to request routing.
//
// Server is optional: a page that acts on one server - Migrate - is handed it
// with the routing request, so there is still exactly one way to change page
// rather than a second mechanism for pages that need an argument.
type ChangePageMsg struct {
	Target string
	Server *storage.Server
}

// ReloadServersMsg asks the servers list to re-read servers.json, after
// something outside it changed what is registered.
type ReloadServersMsg struct{}

// WakeMsg is delivered when a page becomes active again, so anything it owns
// that runs on a timer can restart. A timer only ticks into the page that is on
// screen, so a page left behind stops being told to poll - waking it is what
// keeps the console following its servers after a trip through another page.
type WakeMsg struct{}

// RefreshMsg asks the active action panel to re-read what it shows, after an
// operation changed it on disk.
type RefreshMsg struct{}

// OperationStartedMsg says a background operation has begun, so anything that
// draws its progress can start looking.
type OperationStartedMsg struct{}

// OperationStarted announces the beginning of a background operation.
func OperationStarted() tea.Cmd {
	return func() tea.Msg {
		return OperationStartedMsg{}
	}
}

// OperationDoneMsg reports a finished background operation: what it said, and
// the fact that it finished. The page splits it in two - the lines go to the
// console, the completion refreshes whichever panel started it - so a panel
// cannot go on showing what was installed before the install.
type OperationDoneMsg struct {
	Lines []supervisor.Line
}

// SwitchActionMsg is emitted when a component wants to swap the actions panel.
type SwitchActionMsg struct {
	NewAction Action
}

// CloseActionMsg asks the host page to drop the active action panel and show
// the root Actions list again. Actions cannot import the components package
// that owns that list, so they ask for it by message instead.
type CloseActionMsg struct{}

// LogMsg carries progress lines from a background operation to the panel that
// displays them. Each line is tagged with where it came from so the console can
// keep Osmium's own output apart from a server's.
type LogMsg struct {
	Lines []supervisor.Line
}

// RestartReadyMsg says a server asked to restart has finally exited and may be
// started again.
type RestartReadyMsg struct {
	Dir string
}

// SelectServerMsg announces which server the rest of the page should act on.
type SelectServerMsg struct {
	Server *storage.Server
}

// ServerControl enumerates the process operations the Actions panel can ask
// the Activity panel to perform. Activity owns the running process, so control
// requests travel to it as messages rather than through shared state.
type ServerControl int

// The process operations a panel can ask for.
const (
	ServerStart ServerControl = iota
	ServerStop
	ServerRestart
)

// ServerControlMsg asks the Activity panel to start, stop or restart a server.
type ServerControlMsg struct {
	Control ServerControl
	Server  *storage.Server
}

// RouteTo asks the app to switch to another page.
func RouteTo(target string) tea.Cmd {
	//? returns a message to be catched by a parent tea.Model
	return func() tea.Msg {
		return ChangePageMsg{Target: target}
	}
}

// RouteToServer switches page and tells the page which server to act on.
func RouteToServer(target string, server *storage.Server) tea.Cmd {
	return func() tea.Msg {
		return ChangePageMsg{Target: target, Server: server}
	}
}

// ReloadServers asks the servers list to re-read what is registered.
func ReloadServers() tea.Cmd {
	return func() tea.Msg {
		return ReloadServersMsg{}
	}
}

// Wake tells a page that has just become active to restart what it polls.
func Wake() tea.Cmd {
	return func() tea.Msg {
		return WakeMsg{}
	}
}

// SwitchAction asks the host page to replace its active action panel.
func SwitchAction(action Action) tea.Cmd {
	//? returns a message to be catched by a parent tea.Model
	return func() tea.Msg {
		return SwitchActionMsg{NewAction: action}
	}
}

// CloseAction returns to the root Actions list.
func CloseAction() tea.Cmd {
	return func() tea.Msg {
		return CloseActionMsg{}
	}
}

// Log sends Osmium's own progress lines to the Activity panel.
func Log(lines ...string) tea.Cmd {
	return LogAs(supervisor.TagOsmium, lines...)
}

// LogAs sends progress lines that came from somewhere in particular - a mod
// operation, a failure - so the console can label them.
func LogAs(tag supervisor.Tag, lines ...string) tea.Cmd {
	tagged := make([]supervisor.Line, 0, len(lines))
	for _, line := range lines {
		tagged = append(tagged, supervisor.Line{Tag: tag, Text: line})
	}

	return func() tea.Msg {
		return LogMsg{Lines: tagged}
	}
}

// ControlServer asks the Activity panel to run a process operation.
func ControlServer(control ServerControl, server *storage.Server) tea.Cmd {
	return func() tea.Msg {
		return ServerControlMsg{Control: control, Server: server}
	}
}

// SelectServer announces a new selection to the rest of the page.
func SelectServer(server *storage.Server) tea.Cmd {
	return func() tea.Msg {
		return SelectServerMsg{Server: server}
	}
}

// LoadedServersMsg is dispatched once servers are read from disk.
type LoadedServersMsg struct {
	Servers []storage.Server

	// Missing names the servers dropped from the list because their folder is
	// no longer there, so the panel can report what it removed.
	Missing []string

	Err error
}

// SavedServersMsg is dispatched after write operations complete.
type SavedServersMsg struct {
	Err error
}

// LoadServersCmd creates a Bubble Tea command to read servers asynchronously,
// dropping any whose folder has been deleted behind Osmium's back.
func LoadServersCmd(store *storage.ServerStore) tea.Cmd {
	return func() tea.Msg {
		servers, missing, err := store.Load()
		return LoadedServersMsg{Servers: servers, Missing: missing, Err: err}
	}
}

// SaveServersCmd creates a Bubble Tea command to write servers asynchronously.
func SaveServersCmd(store *storage.ServerStore, servers []storage.Server) tea.Cmd {
	return func() tea.Msg {
		err := store.SaveAll(servers)
		return SavedServersMsg{Err: err}
	}
}
