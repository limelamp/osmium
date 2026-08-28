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
	Use:   "update [project-id ...]",
	Short: "Update tracked mods/plugins.",
	Long: `Updates tracked projects to the latest compatible versions.

Without args, updates all tracked projects (or all within --mod/--plugin).
With args, updates only specified IDs and requires --mod or --plugin.
Examples:
  osmium update
  osmium update --mod
  osmium update --plugin luckperms`,
	ValidArgsFunction: completeInstalled,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectType := "all"
		switch {
		case updateflags.modFlag:
			projectType = "mods"
		case updateflags.pluginFlag:
			projectType = "plugins"
		case len(args) != 0:
			return fmt.Errorf("you must specify either --mod or --plugin")
		}

		sc, err := resolveScope()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return shared.UpdateAllProjects(sc, projectType)
		}

		for _, projectID := range args {
			if err := shared.UpdateProject(sc, projectID, projectType); err != nil {
				fmt.Println(err)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	registerServerFlag(updateCmd)

	updateCmd.Flags().BoolVarP(&updateflags.modFlag, "mod", "m", false, "Download as mod")
	updateCmd.Flags().BoolVarP(&updateflags.pluginFlag, "plugin", "p", false, "Download as plugin")

	// make them mutually exclusive (Cobra built‑in)
	updateCmd.MarkFlagsMutuallyExclusive("mod", "plugin")
}
