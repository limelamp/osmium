/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/spf13/cobra"
)

type destroyFlags struct {
	force        bool
	yes          bool
	keepManifest bool
	deleteAll    bool
}

var destroyflags destroyFlags

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Delete a server and everything in it.",
	Long: `Deletes a server's folder — world included — and removes it from the list.

This cannot be undone, so it asks you to type the server's name first. A running
server is refused unless --force is given, which stops it before deleting.

osmium.json is worth keeping: it records the loader, version, memory and every
tracked mod, so the same server can be built again from it. You are asked
whether to save it beside the folder; --keep-manifest and --delete-everything
answer that question up front.

Examples:
  osmium destroy --server old-test
  osmium destroy --server old-test --keep-manifest --yes
  osmium destroy --force`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := targetServer()
		if err != nil {
			return err
		}

		if supervisor.Running(server.Path) {
			if !destroyflags.force {
				return fmt.Errorf("%s is running — stop it first, or pass --force", server.Name)
			}

			fmt.Printf("Stopping %s before deleting it", server.Name)
			_ = supervisor.Stop(server.Path, false)
			waitUntilStopped(server.Path, 60*time.Second)

			if supervisor.Running(server.Path) {
				_ = supervisor.Stop(server.Path, true)
				time.Sleep(time.Second)
			}
		}

		keep, err := keepManifest()
		if err != nil {
			return err
		}

		if !destroyflags.yes && !confirmName(server.Name, server.Path) {
			fmt.Println("Cancelled. Nothing was deleted.")
			return nil
		}

		kept, err := storage.Destroy(store, server, keep)
		if err != nil {
			return err
		}

		fmt.Printf("Deleted %s (%s).\n", server.Name, server.Path)
		if kept != "" {
			fmt.Printf("Kept its manifest at %s — rebuild from it with 'osmium create'.\n", kept)
		}
		return nil
	},
}

// targetServer is the server to delete: the one --server names, or whichever
// registered server the working directory belongs to.
func targetServer() (storage.Server, error) {
	if serverFlag != "" {
		return findServer(serverFlag)
	}

	dir, err := filepath.Abs(".")
	if err != nil {
		return storage.Server{}, err
	}

	servers, err := store.LoadAll()
	if err != nil {
		return storage.Server{}, err
	}

	for _, server := range servers {
		if absolute, err := filepath.Abs(server.Path); err == nil && absolute == dir {
			return server, nil
		}
	}

	return storage.Server{}, fmt.Errorf("this folder is not a registered server — pass --server to name one")
}

// keepManifest settles whether osmium.json is saved before the folder goes,
// asking when neither flag has already decided it.
func keepManifest() (bool, error) {
	switch {
	case destroyflags.keepManifest:
		return true, nil
	case destroyflags.deleteAll:
		return false, nil
	case destroyflags.yes:
		// An unattended run cannot be asked, and keeping a file nobody wanted
		// is the harmless answer.
		return true, nil
	}

	fmt.Print("Keep osmium.json so this server can be rebuilt? [Y/n] ")
	answer, err := readLine()
	if err != nil {
		return false, err
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "" || answer == "y" || answer == "yes", nil
}

// confirmName makes the user type the server's name, which is the one guard
// that cannot be passed by holding down a key.
func confirmName(name, path string) bool {
	fmt.Printf("This deletes %s and everything inside it, world included.\n", path)
	fmt.Printf("Type the server's name (%s) to confirm: ", name)

	typed, err := readLine()
	if err != nil {
		return false
	}
	return strings.TrimSpace(typed) == name
}

// stdin is read through one buffered reader for the life of the command: a
// fresh reader per prompt would buffer the answers to the later ones and then
// throw them away with itself.
var stdin = bufio.NewReader(os.Stdin)

// readLine reads one answer from the terminal.
func readLine() (string, error) {
	return stdin.ReadString('\n')
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	registerServerFlag(destroyCmd)
	destroyCmd.Flags().BoolVarP(&destroyflags.force, "force", "f", false, "Stop the server first if it is running")
	destroyCmd.Flags().BoolVarP(&destroyflags.yes, "yes", "y", false, "Skip the confirmation prompt")
	destroyCmd.Flags().BoolVar(&destroyflags.keepManifest, "keep-manifest", false, "Save osmium.json beside the folder before deleting")
	destroyCmd.Flags().BoolVar(&destroyflags.deleteAll, "delete-everything", false, "Delete osmium.json along with the rest")
	destroyCmd.MarkFlagsMutuallyExclusive("keep-manifest", "delete-everything")
}
