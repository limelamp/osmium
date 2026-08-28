/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/spf13/cobra"
)

// restartCmd represents the restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart a Minecraft server.",
	Long: `Stops a running server, waits for it to finish saving, and starts it again.

A server that is not running is simply started.

Examples:
  osmium restart
  osmium restart --server survival`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		if supervisor.Running(dir) {
			if err := supervisor.Stop(dir, false); err != nil {
				return err
			}
			fmt.Printf("%s is shutting down", serverName(dir))
			waitUntilStopped(dir, 90*time.Second)
		}

		if supervisor.Running(dir) {
			return fmt.Errorf("%s did not stop in time; it was not restarted", serverName(dir))
		}

		if err := supervisor.Start(dir, serverName(dir)); err != nil {
			return err
		}

		fmt.Printf("Started %s.\n", serverName(dir))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)

	registerServerFlag(restartCmd)
}
