/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

type addFlags struct {
	modFlag    bool
	pluginFlag bool
}

var addflags addFlags

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add [project-id ...]",
	Short: "Add one or more Modrinth projects.",
	Long: `Downloads and installs one or more Modrinth projects.

Use --mod for mods or --plugin for plugins.
Examples:
  osmium add --mod sodium
  osmium add --plugin luckperms viaversion`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectType string

		switch {
		case addflags.modFlag:
			projectType = "mods"
		case addflags.pluginFlag:
			projectType = "plugins"
		default:
			fmt.Println("Error: you must specify either --mod or --plugin")
			return
		}

		for _, projectID := range args {
			if err := shared.AddProjectByID(projectID, projectType); err != nil {
				fmt.Println(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().BoolVarP(&addflags.modFlag, "mod", "m", false, "Download as mod")
	addCmd.Flags().BoolVarP(&addflags.pluginFlag, "plugin", "p", false, "Download as plugin")

	// make them mutually exclusive (Cobra built‑in)
	addCmd.MarkFlagsMutuallyExclusive("mod", "plugin")
	addCmd.MarkFlagsOneRequired("mod", "plugin")
}
