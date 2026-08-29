package cmd

import (
	"fmt"
	"strings"

	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/spf13/cobra"
)

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec [minecraft command]",
	Short: "Run a command on a running server's console.",
	Long: `Sends a command to a running server, exactly as if it had been typed into the
server console.

The server is reached through its supervisor, so this works whoever started it:
the TUI, another shell, or a previous login.

Examples:
  osmium exec say hello everyone
  osmium exec --server survival "time set day"
  osmium exec -s survival op steve`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := runningDir()
		if err != nil {
			return err
		}

		command := strings.Join(args, " ")
		if err := supervisor.Exec(dir, command); err != nil {
			return err
		}

		fmt.Printf("Sent to %s: %s\n", serverName(dir), command)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(execCmd)

	registerServerFlag(execCmd)
}
