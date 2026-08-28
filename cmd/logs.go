package cmd

import (
	"fmt"
	"time"

	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/spf13/cobra"
)

// defaultLogLines is how much history the log commands show before following.
const defaultLogLines = 200

type logsFlags struct {
	follow bool
	lines  int
}

var logsflags logsFlags

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show a server's log.",
	Long: `Prints what a server has said, and optionally keeps printing as it says more.

Each line is labelled with where it came from — the server itself, Osmium, a
mod operation, or a command someone typed — so the sources stay apart.

Examples:
  osmium logs
  osmium logs --server survival --follow
  osmium logs -n 1000`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		if logsflags.follow {
			return followLog(dir, logsflags.lines)
		}

		lines, err := supervisor.ReadLast(supervisor.LogPath(dir), logsflags.lines)
		if err != nil {
			return err
		}
		for _, line := range lines {
			fmt.Println(formatLogLine(line))
		}
		return nil
	},
}

// followLog prints a server's log and keeps printing until interrupted.
func followLog(dir string, history int) error {
	tail, lines := supervisor.NewTailer(supervisor.LogPath(dir), history)
	for _, line := range lines {
		fmt.Println(formatLogLine(line))
	}

	for {
		for _, line := range tail.Read() {
			fmt.Println(formatLogLine(line))
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// formatLogLine renders one log line with its source in a fixed-width column,
// the same shape the Activity pane shows.
func formatLogLine(line supervisor.Line) string {
	return fmt.Sprintf("%-6s │ %s", line.Tag, line.Text)
}

func init() {
	rootCmd.AddCommand(logsCmd)

	registerServerFlag(logsCmd)
	logsCmd.Flags().BoolVarP(&logsflags.follow, "follow", "f", false, "Keep printing as new output arrives")
	logsCmd.Flags().IntVarP(&logsflags.lines, "lines", "n", defaultLogLines, "How many past lines to show")
}
