package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/tui"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the full-screen interface.",
	Long: `Opens Osmium's terminal interface, which is also what running 'osmium' with no
arguments does.

Example:
  osmium tui`,
	Args: cobra.NoArgs,
	RunE: runTui,
}

func init() {
	rootCmd.AddCommand(tuiCmd)

	// Both ways of opening the interface take the same flag, so
	// "osmium -s survival" and "osmium tui -s survival" mean the same thing.
	registerServerFlag(tuiCmd)
	registerServerFlag(rootCmd)
}

// workingDir is where the command was run, or an empty string when that cannot
// be determined - which is not a failure, only one fewer place to look.
func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

// isServerFolder reports whether a folder holds a server manifest.
func isServerFolder(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, config.OsmiumFileName))
	return err == nil
}

// openingServer is the server the interface should open on: the one --server
// names, or the one the current folder is.
//
// Finding neither is not a failure - it is the ordinary case of running Osmium
// from anywhere else, and the app opens on its home page. A --server that names
// nothing is a failure, because it says plainly what was wanted.
func openingServer(store *storage.ServerStore) (*storage.Server, error) {
	if serverFlag != "" {
		server, err := findServer(serverFlag)
		if err != nil {
			return nil, err
		}
		return &server, nil
	}

	dir := workingDir()
	if dir == "" || !isServerFolder(dir) {
		// Not standing in a server is the ordinary case, not a failure.
		return nil, nil
	}

	// The folder is a server. If it is already in the list, that entry is the
	// one to open; the list is the truth about names and ids.
	servers, _, err := store.Load()
	if err != nil {
		return nil, err
	}

	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	for _, server := range servers {
		if server.Path == absolute {
			return &server, nil
		}
	}

	// A server folder Osmium has never seen is registered on the spot, so that
	// opening it once is enough for it to stay in the list.
	server, err := storage.FromManifest(absolute)
	if err != nil {
		return nil, err
	}
	if err := storage.Register(store, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

func runTui(cmd *cobra.Command, args []string) error {
	// The theme is applied for every command by the root's PersistentPreRun.
	store := storage.NewServerStore()

	server, err := openingServer(store)
	if err != nil {
		return err
	}

	p := tea.NewProgram(tui.NewAppModelFor(store, server))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	return nil
}
