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
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: completeInstalled,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectType := "mods"
		if removeflags.pluginFlag {
			projectType = "plugins"
		}

		sc, err := resolveScope()
		if err != nil {
			return err
		}

		for _, projectID := range args {
			if err := shared.RemoveProjectByID(sc, projectID, projectType); err != nil {
				fmt.Println(err)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)

	registerServerFlag(removeCmd)

	removeCmd.Flags().BoolVarP(&removeflags.modFlag, "mod", "m", false, "Download as mod")
	removeCmd.Flags().BoolVarP(&removeflags.pluginFlag, "plugin", "p", false, "Download as plugin")

	// make them mutually exclusive (Cobra built‑in)
	removeCmd.MarkFlagsMutuallyExclusive("mod", "plugin")
	removeCmd.MarkFlagsOneRequired("mod", "plugin")
}
