/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

// trackCmd represents the track command
var trackCmd = &cobra.Command{
	Use:   "track",
	Short: "Track existing jar files into osmium.json.",
	Long: `Scans the mods and plugins directories, resolves known projects by hash,
and adds missing entries to osmium.json.

Example:
  osmium track`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sc, progress, err := resolveScope()
		if err != nil {
			return err
		}
		defer progress.Close()
		return shared.TrackProjects(sc)
	},
}

func init() {
	rootCmd.AddCommand(trackCmd)

	registerServerFlag(trackCmd)
}
