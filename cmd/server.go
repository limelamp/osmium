package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/spf13/cobra"
)

// serverFlag is the name of the registered server a command acts on. It is a
// persistent flag, so every command that works inside a server folder can also
// be pointed at one by name from anywhere:
//
//	osmium start -s survival
//	osmium add --mod sodium -s survival
//
// Without it, commands keep their original behaviour and act on the working
// directory, which is what running them from inside a server folder means.
var serverFlag string

// store is the registry the --server flag is resolved against.
var store = storage.NewServerStore()

// resolveDir is the directory the current invocation acts on.
func resolveDir() (string, error) {
	if serverFlag == "" {
		return ".", nil
	}

	server, err := findServer(serverFlag)
	if err != nil {
		return "", err
	}
	return server.Path, nil
}

// resolveScope is resolveDir as a scope, which is what the project helpers
// take. The scope reports through the command's reporter, so every operation
// gets the same spinner and progress bar; the caller closes it when done.
func resolveScope() (scope.Scope, *reporter, error) {
	dir, err := resolveDir()
	if err != nil {
		return scope.Scope{}, nil, err
	}

	progress := newReporter()
	return progress.Scope(dir), progress, nil
}

// findServer looks a registered server up by name, case-insensitively, and
// falls back to matching its folder name so tab-completed paths work too.
func findServer(name string) (storage.Server, error) {
	servers, _, err := store.Load()
	if err != nil {
		return storage.Server{}, err
	}

	wanted := strings.ToLower(strings.TrimSpace(name))
	for _, server := range servers {
		if strings.ToLower(server.Name) == wanted || strings.ToLower(filepath.Base(server.Path)) == wanted {
			return server, nil
		}
	}

	return storage.Server{}, fmt.Errorf("no server named %q — run 'osmium list' to see the registered servers", name)
}

// serverName is a display name for whatever directory a command resolved to.
func serverName(dir string) string {
	if serverFlag != "" {
		return serverFlag
	}

	absolute, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return filepath.Base(absolute)
}

// requireServerDir resolves the target directory and refuses one that is not a
// server, so a mistyped --server or a wrong working directory fails with an
// explanation instead of a confusing error further in.
func requireServerDir() (string, error) {
	dir, err := resolveDir()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(filepath.Join(dir, config.OsmiumFileName)); err != nil {
		if serverFlag == "" {
			return "", fmt.Errorf("this folder is not an Osmium server (no %s) — run this from a server folder or pass --server", config.OsmiumFileName)
		}
		return "", fmt.Errorf("%s has no %s", dir, config.OsmiumFileName)
	}
	return dir, nil
}

// runningDir resolves the target directory and refuses one whose server is not
// running, which every control command needs.
func runningDir() (string, error) {
	dir, err := requireServerDir()
	if err != nil {
		return "", err
	}

	if !supervisor.Running(dir) {
		return "", fmt.Errorf("%s is not running — start it with 'osmium start'", serverName(dir))
	}
	return dir, nil
}

// registerServerFlag adds the --server flag to a command, with completion.
func registerServerFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&serverFlag, "server", "s", "",
		"Name of a registered server to act on (defaults to the current folder)")
	_ = cmd.RegisterFlagCompletionFunc("server", completeServerNames)
}
