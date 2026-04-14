/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

type installFlags struct {
	modFlag    bool
	pluginFlag bool
}

var installflags installFlags

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install all projects from osmium.json.",
	Long: `Installs all tracked mods and plugins listed in osmium.json.

Example:
  osmium install`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := shared.InstallProjectsFromConfig(); err != nil {
			fmt.Println(err)
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
