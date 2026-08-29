package cmd

import (
	"fmt"

	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/spf13/cobra"
)

var startFollow bool

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a Minecraft server.",
	Long: `Starts a server under a supervisor process of its own.

The server keeps running after this command returns, and after Osmium's TUI is
closed. Send it commands with 'osmium exec', watch it with 'osmium logs -f',
and stop it with 'osmium stop'.

Examples:
  osmium start
  osmium start --server survival --follow`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		if err := supervisor.Start(dir, serverName(dir)); err != nil {
			return err
		}
		fmt.Printf("Started %s.\n", serverName(dir))

		if startFollow {
			return followLog(dir, defaultLogLines)
		}

		fmt.Println("Follow its output with 'osmium logs -f'.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	registerServerFlag(startCmd)
	startCmd.Flags().BoolVarP(&startFollow, "follow", "f", false, "Follow the server log after starting")
}
