/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
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
	RunE: func(cmd *cobra.Command, args []string) error {
		sc, err := resolveScope()
		if err != nil {
			return err
		}
		return shared.InstallProjectsFromConfig(sc)
	},
}

func init() {
	rootCmd.AddCommand(installCmd)

	registerServerFlag(installCmd)
}
