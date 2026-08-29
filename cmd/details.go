package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/spf13/cobra"
)

// detailsCmd represents the details command
var detailsCmd = &cobra.Command{
	Use:     "details",
	Aliases: []string{"info"},
	Short:   "Show everything about a server.",
	Long: `Prints what a server is, what it is doing and who is on it: its engine and
version, where it lives and how much disk it takes, its players, ops and bans,
and how many mods and plugins it has.

Online players are asked of the server itself, so they only appear while it is
running.

Examples:
  osmium details
  osmium details --server survival`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		conf, err := config.ReadConfigIn(dir)
		if err != nil {
			return err
		}

		players, err := shared.Players(dir)
		if err != nil {
			return err
		}

		maxPlayers := shared.MaxPlayers(dir)

		// Asking who is online means asking the server, whose reply lands in
		// the log a moment later; with it stopped there is nobody to ask.
		if supervisor.Running(dir) {
			_ = supervisor.Exec(dir, "list")
		}
		if supervisor.Running(dir) {
			if names, _, limit, ok := onlineFromLog(dir); ok {
				players = shared.MarkOnline(players, names)
				if limit > 0 {
					maxPlayers = limit
				}
			}
		}

		table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		fmt.Fprintf(table, "%s\n", serverName(dir))
		fmt.Fprintf(table, "  state\t%s\n", state(dir, true))
		fmt.Fprintf(table, "  memory\t%s\n", conf.AllocatedRAM)
		fmt.Fprintf(table, "  engine\t%s %s\n", conf.Loader, conf.Version)
		fmt.Fprintf(table, "  folder\t%s\n", dir)
		fmt.Fprintf(table, "  size on disk\t%s\n", styles.FormatBytes(shared.FolderSize(dir)))
		fmt.Fprintf(table, "  mods\t%d\n", len(conf.Mods))
		fmt.Fprintf(table, "  plugins\t%d\n", len(conf.Plugins))

		online, ops, banned, whitelisted := groupPlayers(players)

		fmt.Fprintf(table, "  online\t%s\n", playerCount(len(online), maxPlayers, online))
		fmt.Fprintf(table, "  ops\t%s\n", playerList(ops))
		fmt.Fprintf(table, "  banned\t%s\n", playerList(banned))
		fmt.Fprintf(table, "  whitelisted\t%s\n", playerList(whitelisted))
		fmt.Fprintf(table, "  known\t%d players seen\n", len(players))

		return table.Flush()
	},
}

// groupPlayers splits the roll into the lists worth printing.
func groupPlayers(players []shared.Player) (online, ops, banned, whitelisted []string) {
	for _, player := range players {
		if player.Online {
			online = append(online, player.Name)
		}
		if player.Op {
			ops = append(ops, player.Name)
		}
		if player.Banned {
			banned = append(banned, player.Name)
		}
		if player.Whitelisted {
			whitelisted = append(whitelisted, player.Name)
		}
	}
	return online, ops, banned, whitelisted
}

// playerCount renders "3 / 20  Steve, Alex, Notch".
func playerCount(online, limit int, names []string) string {
	count := fmt.Sprint(online)
	if limit > 0 {
		count += fmt.Sprintf(" / %d", limit)
	}

	if len(names) == 0 {
		return count
	}
	return count + "  " + strings.Join(names, ", ")
}

// playerList renders a list of names, or a dash.
func playerList(names []string) string {
	if len(names) == 0 {
		return "-"
	}
	return strings.Join(names, ", ")
}

// onlineFromLog reads the newest answer to "list" out of a server's log.
func onlineFromLog(dir string) (names []string, online, limit int, ok bool) {
	lines, err := supervisor.ReadLast(supervisor.LogPath(dir), 200)
	if err != nil {
		return nil, 0, 0, false
	}

	var text strings.Builder
	for _, line := range lines {
		text.WriteString(line.Text)
		text.WriteByte('\n')
	}

	return shared.OnlineFromLog(text.String())
}

func init() {
	rootCmd.AddCommand(detailsCmd)

	registerServerFlag(detailsCmd)
}
