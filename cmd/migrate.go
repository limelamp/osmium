/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

type migrateFlags struct {
	loader  string
	version string
}

var migrateflags migrateFlags

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate --loader <loader> --version <version>",
	Short: "Migrate server to a different loader or version",
	Long: `Migrate your Minecraft server to a different mod loader or version.

This command will:
  - Replace the server.jar with the new loader/version
  - Migrate existing mods/plugins to compatible versions
  - Keep world data, configs, and other files intact
  - Move incompatible mods/plugins to a backup folder

Example:
  osmium migrate -l Paper -v 1.21.1
  osmium migrate -l Fabric -v 1.20.4`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}
		return shared.MigrateServer(scope.For(dir), migrateflags.loader, migrateflags.version)
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	registerServerFlag(migrateCmd)
	migrateCmd.Flags().StringVarP(&migrateflags.loader, "loader", "l", "", "Minecraft mod or plugin loader")
	migrateCmd.Flags().StringVarP(&migrateflags.version, "version", "v", "", "Minecraft version")
	migrateCmd.MarkFlagRequired("loader")
	migrateCmd.MarkFlagRequired("version")

	_ = migrateCmd.RegisterFlagCompletionFunc("loader", completeLoaders)
	_ = migrateCmd.RegisterFlagCompletionFunc("version", completeVersions)
}
