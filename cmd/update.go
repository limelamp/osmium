/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

type updateFlags struct {
	modFlag    bool
	pluginFlag bool
}

var updateflags updateFlags

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
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
		case updateflags.modFlag:
			projectType = "mods"
		case updateflags.pluginFlag:
			projectType = "plugins"
		default:
			fmt.Println("Error: you must specify either --mod or --plugin")
			return
		}

		for _, projectID := range args {
			if err := shared.UpdateProject(projectID, projectType); err != nil {
				fmt.Println(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().BoolVarP(&updateflags.modFlag, "mod", "m", false, "Download as mod")
	updateCmd.Flags().BoolVarP(&updateflags.pluginFlag, "plugin", "p", false, "Download as plugin")

	// make them mutually exclusive (Cobra built‑in)
	updateCmd.MarkFlagsMutuallyExclusive("mod", "plugin")
}
