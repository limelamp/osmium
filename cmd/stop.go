/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var forceFlag bool

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "osmium stop",
	Short: "Stop the Minecraft server.",
	Long:  `Stops the Minecraft server that is currently running with Osmium.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Read and get the pid from lock file
		data, err := os.ReadFile(".osmium_process.lock")
		if err != nil {
			fmt.Println("Error reading the lock file:", err)
			return
		}
		pid, _ := strconv.Atoi(string(data)) // Converts data from []byte --> string --> int

		// Check if the process with that pid actually exists
		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("failed to find process: %d", err)
		}

		if forceFlag { // .Kill() is equivalent to SIGKILL (force quit)
			err = process.Kill()
			if err != nil {
				fmt.Printf("failed to kill process: %d", err)
			}
			fmt.Printf("Process %d has been killed.\n", pid)

			// Remove the .lock file once the process is killed.
			err := os.Remove(".osmium_process.lock")
			if err != nil {
				fmt.Println("Error removing file:", err)
			}
		} else { // process.Signal(os.Interrupt) for a cleaner exit(?)
			err = process.Signal(os.Interrupt)
			if err != nil {
				fmt.Printf("failed to stop process: %d", err)
			}
			fmt.Printf("Process %d has been stopped.\n", pid)

			// Remove the .lock file once the process is stopped.
			err := os.Remove(".osmium_process.lock")
			if err != nil {
				fmt.Println("Error removing file:", err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force the server to be killed (SIGKILL)")
}
