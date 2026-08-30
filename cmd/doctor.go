package cmd

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/health"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check a server for the things that stop it starting.",
	Long: `Looks a server over and reports what it finds: whether the Java on the
path is the one this Minecraft version needs, whether the memory it is
promised exists, whether its port is free, whether the licence has been
accepted, and whether the files the lock names are actually there.

Nothing is changed. Every finding says what to do about it.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		findings := health.Check(dir)

		fmt.Println(lipgloss.NewStyle().Foreground(theme.Primary).Render(serverName(dir)))
		fmt.Println()

		for _, finding := range findings {
			colour := theme.Accent
			switch finding.Level {
			case health.Problem:
				colour = styles.ErrorColor
			case health.Note:
				colour = theme.Primary
			}

			fmt.Println("  " + lipgloss.NewStyle().Foreground(colour).Render(finding.Level.Mark()) +
				" " + finding.Title + "   " + muted(finding.Detail))
		}

		fmt.Println()
		if health.Worst(findings) == health.Problem {
			return fmt.Errorf("this server has problems that will stop it working")
		}

		fmt.Println(muted("Nothing here would stop the server starting."))
		return nil
	},
}

func init() {
	registerServerFlag(doctorCmd)
	rootCmd.AddCommand(doctorCmd)
}
