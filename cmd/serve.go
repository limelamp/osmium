package cmd

import (
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/spf13/cobra"
)

type serveFlags struct {
	dir  string
	name string
}

var serveflags serveFlags

// serveCmd is the supervisor Osmium spawns for a server; it is not meant to be
// run by hand, which is why it is hidden from the help.
//
// It holds the server's stdin, writes everything the server says to
// <server>/.osmium/server.log, and answers control requests on the server's
// socket until the server exits. Keeping this in its own detached process is
// what lets a server outlive the TUI or the shell that started it.
var serveCmd = &cobra.Command{
	Use:    "serve",
	Short:  "Run the supervisor process for a server (internal).",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return supervisor.Serve(serveflags.dir, serveflags.name)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVar(&serveflags.dir, "dir", ".", "Server directory to supervise")
	serveCmd.Flags().StringVar(&serveflags.name, "name", "", "Display name of the server")
}
