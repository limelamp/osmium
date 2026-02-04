/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/limelamp/osmium/internal/tui"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "osmium start",
	Short: "Start the Minecraft server.",
	Long:  `Starts the \"Run Server\" page inside Osmium, which then starts the Minecraft server.`,
	Run: func(cmd *cobra.Command, args []string) {
		mainProcess := tea.NewProgram(tui.NewRunServerModel(), tea.WithAltScreen())
		if _, err := mainProcess.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
