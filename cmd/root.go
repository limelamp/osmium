package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// Cobra and CLI stuff ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
var rootCmd = &cobra.Command{
	Use:   "osmium",
	Short: "A full-screen TUI app for managing minecraft servers.",
	// Run: func(cmd *cobra.Command, args []string) { // Was used before the refactor
	// 	mainProcess := tea.NewProgram(internal.NewRootModel(), tea.WithAltScreen())
	// 	if _, err := mainProcess.Run(); err != nil {
	// 		fmt.Println(err)
	// 		os.Exit(1)
	// 	}
	// },
	RunE: runTui,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
