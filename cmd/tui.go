package cmd

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/limelamp/osmium/internal/tui"
	"github.com/limelamp/osmium/internal/tui/config"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/theme"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: runTui,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTui(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err == nil {
		theme.SetTheme(cfg.Theme)
	}

	store := storage.NewServerStore()

	//? For testing servers.json in appdata
	// err := store.SaveAll([]storage.Server{
	// 	{
	// 		ID:      "srv-vanilla-survival",
	// 		Name:    "Vanilla SMP",
	// 		Path:    "/home/user/.config/osmium/servers/vanilla-smp",
	// 		Version: "1.21.1",
	// 		Type:    "vanilla",
	// 		Memory:  "4G",
	// 	},
	// 	{
	// 		ID:      "srv-paper-lobby",
	// 		Name:    "Hub/Lobby (Paper)",
	// 		Path:    "/home/user/.config/osmium/servers/paper-lobby",
	// 		Version: "1.20.4",
	// 		Type:    "paper",
	// 		Memory:  "2G",
	// 	},
	// 	{
	// 		ID:      "srv-fabric-creative",
	// 		Name:    "Creative World (Fabric)",
	// 		Path:    "/home/user/.config/osmium/servers/fabric-creative",
	// 		Version: "1.21.1",
	// 		Type:    "fabric",
	// 		Memory:  "3G",
	// 	},
	// 	{
	// 		ID:      "srv-purpur-minigames",
	// 		Name:    "Minigames PURPUR",
	// 		Path:    "/home/user/.config/osmium/servers/purpur-minigames",
	// 		Version: "1.20.2",
	// 		Type:    "purpur",
	// 		Memory:  "6G",
	// 	},
	// 	{
	// 		ID:      "srv-velocity-proxy",
	// 		Name:    "Network Proxy (Velocity)",
	// 		Path:    "/home/user/.config/osmium/servers/velocity-proxy",
	// 		Version: "3.3.0",
	// 		Type:    "velocity",
	// 		Memory:  "1G",
	// 	},
	// })
	// if err != nil {
	// 	return err
	// }

	p := tea.NewProgram(tui.NewAppModel(store))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	return nil
}
