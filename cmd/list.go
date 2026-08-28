package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the registered servers.",
	Long: `Shows every server Osmium knows about: what it runs, where it lives and
whether it is up right now.

The names in the first column are what --server takes.

Example:
  osmium list`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, _, err := store.Load()
		if err != nil {
			return err
		}

		if len(servers) == 0 {
			fmt.Println("No servers yet. Create one with 'osmium create'.")
			return nil
		}

		table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(table, "NAME\tENGINE\tVERSION\tMEMORY\tSTATE\tPATH")

		for _, server := range servers {
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n",
				server.Name,
				strings.ToUpper(server.Type),
				server.Version,
				server.Memory,
				state(server.Path, server.Exists()),
				server.Path,
			)
		}

		return table.Flush()
	},
}

// state describes what a server is doing, or that it is no longer there.
func state(dir string, exists bool) string {
	switch {
	case !exists:
		return "missing"
	case supervisor.Running(dir):
		return fmt.Sprintf("running (pid %d)", supervisor.PID(dir))
	default:
		return "stopped"
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
}
