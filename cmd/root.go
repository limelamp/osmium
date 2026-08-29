// Package cmd is Osmium's command line: the commands themselves, the flags
// they share, and the entry point that opens the interface.
package cmd

import (
	"os"

	"github.com/limelamp/osmium/internal/build"
	"github.com/spf13/cobra"
)

// Cobra and CLI stuff ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
var rootCmd = &cobra.Command{
	Use:   "osmium",
	Short: "A full-screen TUI app for managing minecraft servers.",
	Long: `Osmium manages Minecraft servers, from the terminal.

Run it with no arguments for the full-screen interface, or use the commands
below to do the same things from a shell or a script. Commands act on the
current folder, or on any registered server with --server.

Tab completion knows your servers, their installed mods and the available
Minecraft versions. Install it with:

  osmium completion bash > /etc/bash_completion.d/osmium     # bash
  osmium completion zsh  > "${fpath[1]}/_osmium"             # zsh
  osmium completion fish > ~/.config/fish/completions/osmium.fish`,

	// A command that fails at runtime has already explained itself; printing
	// the whole usage after it only buries the message.
	SilenceUsage: true,

	Version: build.Full(),
	// Run: func(cmd *cobra.Command, args []string) { // Was used before the refactor
	// 	mainProcess := tea.NewProgram(internal.NewRootModel(), tea.WithAltScreen())
	// 	if _, err := mainProcess.Run(); err != nil {
	// 		fmt.Println(err)
	// 		os.Exit(1)
	// 	}
	// },
	RunE: runTui,
}

// Execute runs the command line, and is where main begins.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Errors are printed by Execute; nothing else belongs on the root command,
	// which only opens the interface.
	rootCmd.SilenceErrors = false

	// "osmium --version" prints the line and nothing else; the default
	// template wraps it in the command's name and a label.
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}
