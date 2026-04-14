/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

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
	Run: func(cmd *cobra.Command, args []string) {
		err := shared.TrackProjects()
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(trackCmd)
}
