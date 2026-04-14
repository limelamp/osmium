/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

var forceFlag bool

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Minecraft server.",
	Long:  `Stops the Minecraft server that is currently running with Osmium.`,
	Run: func(cmd *cobra.Command, args []string) {
		pid, err := shared.ReadLockPID()
		if err != nil {
			fmt.Println("No active lock file found. Server may already be stopped.")
			return
		}

		if !shared.IsPIDRunning(pid) {
			if err := shared.RemoveLockFile(); err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("Found stale lock file for PID %d. Lock file removed.\n", pid)
			return
		}

		// Check if the process with that pid actually exists
		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("failed to find process: %v\n", err)
			return
		}

		if forceFlag { // .Kill() is equivalent to SIGKILL (force quit)
			err = process.Kill()
			if err != nil {
				fmt.Printf("failed to kill process: %v\n", err)
				return
			}
			fmt.Printf("Server process %d has been force-killed.\n", pid)

			// Remove the .lock file once the process is killed.
			if err := shared.RemoveLockFile(); err != nil {
				fmt.Println(err)
			}
		} else { // process.Signal(os.Interrupt) for a cleaner exit(?)
			err = process.Signal(os.Interrupt)
			if err != nil {
				fmt.Printf("failed to stop process: %v\n", err)
				return
			}
			fmt.Printf("Server process %d has been asked to stop gracefully.\n", pid)

			// Remove the .lock file once the process is stopped.
			if err := shared.RemoveLockFile(); err != nil {
				fmt.Println(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force the server to be killed (SIGKILL)")
}
