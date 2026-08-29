package cmd

import (
	"fmt"
	"strings"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/spf13/cobra"
)

type editFlags struct {
	name string
	ram  string
}

var editflags editFlags

// editCmd represents the edit command
var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Change a server's name or memory.",
	Long: `Changes the settings Osmium keeps for a server: what it is called in the list,
and how much memory it is started with.

This is not migrate: the loader, the version and the mods are untouched. With no
flags it prints what the server is set to now.

Examples:
  osmium edit --server survival
  osmium edit --server survival --ram 8G
  osmium edit --name "Survival SMP" --ram 6G`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := targetServer()
		if err != nil {
			return err
		}

		conf, err := config.ReadConfigIn(server.Path)
		if err != nil {
			return err
		}

		// Nothing to change means nothing to do but say how things stand.
		if editflags.name == "" && editflags.ram == "" {
			fmt.Printf("%s\n  name    %s\n  memory  %s\n  engine  %s %s\n\n",
				server.Name, server.Name, conf.AllocatedRAM, conf.Loader, conf.Version)
			fmt.Println("Change it with --name or --ram.")
			return nil
		}

		if editflags.ram != "" {
			amount, err := steps.NormaliseRAM(editflags.ram)
			if err != nil {
				return err
			}

			previous := conf.AllocatedRAM
			conf.AllocatedRAM = amount
			if err := config.WriteConfigIn(server.Path, conf); err != nil {
				return err
			}

			fmt.Printf("%s: memory %s → %s\n", server.Name, previous, amount)
			if supervisor.Running(server.Path) {
				fmt.Println("It is running with the old amount until it is restarted.")
			}
		}

		if name := strings.TrimSpace(editflags.name); name != "" {
			previous := server.Name
			server.Name = name

			if err := storage.Register(store, &server); err != nil {
				return err
			}
			fmt.Printf("%s: name → %s\n", previous, name)
		}

		// The registry keeps its own copy of the memory, which is read back
		// from the manifest the next time the list is loaded.
		_, _, err = store.Load()
		return err
	},
}

func init() {
	rootCmd.AddCommand(editCmd)

	registerServerFlag(editCmd)
	editCmd.Flags().StringVarP(&editflags.name, "name", "n", "", "New name for the server")
	editCmd.Flags().StringVar(&editflags.ram, "ram", "", "Memory to start the server with, e.g. 8G")
}
