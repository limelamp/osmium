package cmd

import (
	"fmt"
	"time"

	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/spf13/cobra"
)

var forceFlag bool

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a Minecraft server.",
	Long: `Stops a server that is running under Osmium.

The server is asked to shut down the way the console 'stop' command does, so it
saves its world on the way out. --force kills it instead, which does not.

Examples:
  osmium stop
  osmium stop --server survival
  osmium stop --force`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := runningDir()
		if err != nil {
			return err
		}

		if err := supervisor.Stop(dir, forceFlag); err != nil {
			return err
		}

		if forceFlag {
			fmt.Printf("%s was killed.\n", serverName(dir))
			return nil
		}

		fmt.Printf("%s is shutting down", serverName(dir))
		waitUntilStopped(dir, 60*time.Second)
		return nil
	},
}

// waitUntilStopped follows a shutdown to its end, so the command does not
// return while the world is still being saved.
func waitUntilStopped(dir string, limit time.Duration) {
	deadline := time.Now().Add(limit)

	for time.Now().Before(deadline) {
		if !supervisor.Running(dir) {
			fmt.Println("\nStopped.")
			return
		}
		fmt.Print(".")
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\nStill shutting down; it will finish on its own.")
}

func init() {
	rootCmd.AddCommand(stopCmd)

	registerServerFlag(stopCmd)
	stopCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Kill the server instead of asking it to stop")
}
