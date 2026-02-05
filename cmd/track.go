/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

// Todo:
/*
#1 Get SHA1 or SHA512 sums (SHA1 is faster, you only need them for lookup anyway)
	Get-FileHash -Algorithm SHA1 "absolute_path_to_mod" (Windows)
	sha1sum "absolute_path_to_mod" (Linux)
#2 Send request using the hash sum
	https://api.modrinth.com/v2/version_file/{hash_sum}
#3 Pull necessary to osmium.json info from this endpoint
	...
*/

// trackCmd represents the track command
var trackCmd = &cobra.Command{
	Use:   "track",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := shared.TrackProjects();
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(trackCmd)
}
