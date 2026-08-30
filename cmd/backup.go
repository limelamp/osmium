package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/limelamp/osmium/internal/backup"
	"github.com/limelamp/osmium/internal/pack"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup [label]",
	Short: "Take a copy of a server's world and settings.",
	Long: `Copies the parts of a server that cannot be downloaded again: its world
folders, its manifest and lock, its settings, and who is opped, whitelisted
or banned. Mods and the engine are left out — they are named in the lock and
can be fetched — which is what keeps a backup small enough to take often.

Backups are kept in .osmium/backups inside the server, so moving the server
moves them with it.

The server may be running, in which case the world is a few seconds behind.
For an exact copy, stop it first, or run "save-off" and "save-all" in the
console before taking one.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		label := ""
		if len(args) == 1 {
			label = args[0]
		}

		if supervisor.Running(dir) {
			fmt.Println(muted("The server is running; the world will be as of a moment ago."))
		}

		progress := newReporter()
		defer progress.Close()

		done := progress.busy("copying the world")
		taken, err := backup.Take(dir, label)
		done()

		if err != nil {
			return err
		}

		fmt.Printf("✓ %s   %s\n", taken.Name, pack.FormatBytes(taken.Size))
		return nil
	},
}

var backupListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List a server's backups.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		backups := backup.List(dir)
		if len(backups) == 0 {
			fmt.Println("No backups yet.")
			return nil
		}

		table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(table, "NAME\tSIZE\tTAKEN")

		for _, taken := range backups {
			fmt.Fprintf(table, "%s\t%s\t%s\n", taken.Name, pack.FormatBytes(taken.Size), taken.Age())
		}
		return table.Flush()
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore [name]",
	Short: "Put a backup back.",
	Long: `Restores a backup over the server, newest first when no name is given.

The server has to be stopped: writing a world out from under a running
server corrupts it.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		backups := backup.List(dir)
		if len(backups) == 0 {
			return fmt.Errorf("there are no backups of this server")
		}

		chosen := backups[0]
		if len(args) == 1 {
			found := false
			for _, candidate := range backups {
				if candidate.Name == args[0] {
					chosen, found = candidate, true
					break
				}
			}
			if !found {
				return fmt.Errorf("there is no backup called %s", args[0])
			}
		}

		fmt.Printf("Restoring %s, taken %s.\n", chosen.Name, chosen.Age())
		fmt.Println("This overwrites the world as it stands now.")

		if !assumeYes && !confirm() {
			return fmt.Errorf("nothing was changed")
		}

		if err := backup.Restore(dir, chosen); err != nil {
			return err
		}

		fmt.Println("✓ restored")
		return nil
	},
}

func init() {
	backupCmd.AddCommand(backupListCmd)

	for _, cmd := range []*cobra.Command{backupCmd, backupListCmd, restoreCmd} {
		registerServerFlag(cmd)
	}
	restoreCmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Do not ask before overwriting the world.")

	rootCmd.AddCommand(backupCmd, restoreCmd)
}
