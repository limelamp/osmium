/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Error: project ID required")
			return
		}

		var projectType string

		switch {
		case modFlag:
			projectType = "mods"
		case pluginFlag:
			projectType = "plugins"
		default:
			fmt.Println("Error: you must specify either --mod or --plugin")
			return
		}
		
		for _, projectID := range args {
			if err := shared.RemoveProjectByID(projectID, projectType); err != nil {
				fmt.Println(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)

	removeCmd.Flags().BoolVarP(&modFlag, "mod", "m", false, "Download as mod")
	removeCmd.Flags().BoolVarP(&pluginFlag, "plugin", "p", false, "Download as plugin")

	// make them mutually exclusive (Cobra built‑in)
	removeCmd.MarkFlagsMutuallyExclusive("mod", "plugin")
}
