package core

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// ChangePageMsg is sent by child pages to request routing.
type ChangePageMsg struct {
	Target string
}

// SwitchActionMsg is emitted when a component wants to swap the actions panel.
type SwitchActionMsg struct {
	NewAction Action
}

// CloseActionMsg asks the host page to drop the active action panel and show
// the root Actions list again. Actions cannot import the components package
// that owns that list, so they ask for it by message instead.
type CloseActionMsg struct{}

// ShutdownMsg is delivered just before the program quits, so a page holding a
// child process can shut it down instead of orphaning it.
type ShutdownMsg struct{}

// LogMsg carries progress lines from a background operation to the panel that
// displays them.
type LogMsg struct {
	Lines []string
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

// Shutdown asks the pages to release anything they own before the app quits.
func Shutdown() tea.Cmd {
	return func() tea.Msg {
		return ShutdownMsg{}
	}
}

// Log sends progress lines to the Activity panel.
func Log(lines ...string) tea.Cmd {
	return func() tea.Msg {
		return LogMsg{Lines: lines}
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
	Err     error
}

// SavedServersMsg is dispatched after write operations complete.
type SavedServersMsg struct {
	Err error
}

// LoadServersCmd creates a Bubble Tea command to read servers asynchronously.
func LoadServersCmd(store *storage.ServerStore) tea.Cmd {
	return func() tea.Msg {
		servers, err := store.LoadAll()
		return LoadedServersMsg{Servers: servers, Err: err}
	}
}

// SaveServersCmd creates a Bubble Tea command to write servers asynchronously.
func SaveServersCmd(store *storage.ServerStore, servers []storage.Server) tea.Cmd {
	return func() tea.Msg {
		err := store.SaveAll(servers)
		return SavedServersMsg{Err: err}
	}
}
