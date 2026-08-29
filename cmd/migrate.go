/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/spf13/cobra"
)

type migrateFlags struct {
	loader  string
	version string
}

var migrateflags migrateFlags

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate a server to a different loader or version",
	Long: `Migrates a server to a different mod loader or Minecraft version.

This command will:
  - Replace the server.jar with the new loader/version
  - Migrate existing mods/plugins to compatible versions
  - Keep world data, configs, and other files intact
  - Move incompatible mods/plugins to a backup folder

The loader and the version are asked for one at a time when they are not given
as flags, using the same steps the interface uses.

Examples:
  osmium migrate
  osmium migrate -l Paper -v 1.21.1
  osmium migrate --server survival --loader Fabric --version 1.20.4`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		if supervisor.Running(dir) {
			return fmt.Errorf("%s is running — stop it before migrating it", serverName(dir))
		}

		wizardConfig := &steps.Config{
			Dir:      dir,
			Name:     serverName(dir),
			Category: categoryOf(migrateflags.loader),
			Software: canonicalEngine(migrateflags.loader),
			Version:  migrateflags.version,
		}

		if pending := pendingMigrateSteps(wizardConfig); len(pending) > 0 {
			finished, err := runWizard(pending, wizardConfig)
			if err != nil {
				return err
			}
			if !finished {
				fmt.Println("Cancelled. The server was not touched.")
				return nil
			}
		}

		progress := newReporter()
		defer progress.Close()

		return shared.MigrateServer(progress.Scope(dir), wizardConfig.Software, wizardConfig.Version)
	},
}

// pendingMigrateSteps is the migrate wizard reduced to the questions the flags
// left open, ending in the same review the interface shows.
func pendingMigrateSteps(cfg *steps.Config) []steps.Step {
	var pending []steps.Step

	if cfg.Software == "" {
		pending = append(pending, steps.NewEngineStep(cfg))
	}
	if cfg.Version == "" {
		pending = append(pending, steps.NewVersionStep(cfg))
	}

	// Nothing was asked, so both flags said what to do: there is nothing to
	// confirm that the command line did not already state.
	if len(pending) > 0 {
		pending = append(pending, steps.NewMigrateReviewStep(cfg))
	}
	return pending
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	registerServerFlag(migrateCmd)
	migrateCmd.Flags().StringVarP(&migrateflags.loader, "loader", "l", "", "Minecraft mod or plugin loader")
	migrateCmd.Flags().StringVarP(&migrateflags.version, "version", "v", "", "Minecraft version")

	_ = migrateCmd.RegisterFlagCompletionFunc("loader", completeLoaders)
	_ = migrateCmd.RegisterFlagCompletionFunc("version", completeVersions)
}
