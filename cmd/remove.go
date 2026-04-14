/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

type removeFlags struct {
	modFlag    bool
	pluginFlag bool
}

var removeflags removeFlags

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove [project-id ...]",
	Short: "Remove one or more tracked projects.",
	Long: `Removes one or more tracked projects and deletes their files.

Use --mod for mods or --plugin for plugins.
Examples:
  osmium remove --mod sodium
  osmium remove --plugin luckperms`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectType string

		switch {
		case removeflags.modFlag:
			projectType = "mods"
		case removeflags.pluginFlag:
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

	removeCmd.Flags().BoolVarP(&removeflags.modFlag, "mod", "m", false, "Download as mod")
	removeCmd.Flags().BoolVarP(&removeflags.pluginFlag, "plugin", "p", false, "Download as plugin")

	// make them mutually exclusive (Cobra built‑in)
	removeCmd.MarkFlagsMutuallyExclusive("mod", "plugin")
	removeCmd.MarkFlagsOneRequired("mod", "plugin")
}
