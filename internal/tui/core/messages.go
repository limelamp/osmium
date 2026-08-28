package core

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// ChangePageMsg is sent by child pages to request routing.
//
// Server is optional: a page that acts on one server — Migrate — is handed it
// with the routing request, so there is still exactly one way to change page
// rather than a second mechanism for pages that need an argument.
type ChangePageMsg struct {
	Target string
	Server *storage.Server
}

// ReloadServersMsg asks the servers list to re-read servers.json, after
// something outside it changed what is registered.
type ReloadServersMsg struct{}

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

// Helper functions to generate the commands
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

// LogAs sends progress lines that came from somewhere in particular — a mod
// operation, a failure — so the console can label them.
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
		servers, err := store.LoadAll()
		if err != nil {
			return LoadedServersMsg{Err: err}
		}

		present, missing := storage.Present(servers)
		if len(missing) == 0 {
			return LoadedServersMsg{Servers: present}
		}

		return LoadedServersMsg{
			Servers: present,
			Missing: missing,
			Err:     store.SaveAll(present),
		}
	}
}

// SaveServersCmd creates a Bubble Tea command to write servers asynchronously.
func SaveServersCmd(store *storage.ServerStore, servers []storage.Server) tea.Cmd {
	return func() tea.Msg {
		err := store.SaveAll(servers)
		return SavedServersMsg{Err: err}
	}
}
