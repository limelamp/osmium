// tui/internal/tui/core/messages.go
package core

import (
	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui/storage"
)

// ChangePageMsg is sent by child pages to request routing.
type ChangePageMsg struct {
	Target string
}

// Helper functions to generate the commands
func RouteTo(target string) tea.Cmd {
	//? returns a message to be catched by a parent tea.Model
	return func() tea.Msg {
		return ChangePageMsg{Target: target}
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
