/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

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
	Run: func(cmd *cobra.Command, args []string) {
		if err := shared.MigrateServer(migrateflags.loader, migrateflags.version); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().StringVarP(&migrateflags.loader, "loader", "l", "", "Minecraft mod or plugin loader")
	migrateCmd.Flags().StringVarP(&migrateflags.version, "version", "v", "", "Minecraft version")
	migrateCmd.MarkFlagRequired("loader")
	migrateCmd.MarkFlagRequired("version")
}
