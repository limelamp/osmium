package cmd

import (
	"context"
	"fmt"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/expose"
	"github.com/limelamp/osmium/internal/properties"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
	"github.com/spf13/cobra"
)

// exposeflags are what the expose command was asked to do.
var exposeflags struct {
	open  bool
	off   bool
	check bool
}

var exposeCmd = &cobra.Command{
	Use:   "expose",
	Short: "Show how to reach a server, and open its port if you ask.",
	Long: `Says which addresses reach this server and whether anything answers on
them.

With --open it asks the router to send the server's port through to this
machine, which is what lets people outside your network join. That is a
change to the router, not to this computer, and --off takes it back. Nothing
is installed either way.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := requireServerDir()
		if err != nil {
			return err
		}

		props, err := properties.Read(dir)
		if err != nil {
			return fmt.Errorf("this server has no %s yet — start it once first", properties.FileName)
		}

		port := props.Port()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		switch {
		case exposeflags.off:
			if err := expose.Close(ctx, port); err != nil {
				return err
			}
			fmt.Printf("✓ the router is no longer forwarding %d\n", port)
			return nil

		case exposeflags.open:
			progress := newReporter()
			done := progress.busy("asking the router")

			mapping, err := expose.Open(ctx, port, serverName(dir))
			done()
			progress.Close()

			if err != nil {
				return err
			}

			fmt.Printf("✓ the router is forwarding %d to this machine\n", port)
			if mapping.External != "" {
				fmt.Printf("  players can join at %s:%d\n", mapping.External, port)
			}
			fmt.Println(muted("  the mapping lasts a day; run this again to renew it, or --off to remove it"))
			return nil
		}

		showAddresses(ctx, port)
		return nil
	},
}

// showAddresses prints where a server can be reached, and whether it answers.
func showAddresses(ctx context.Context, port int) {
	progress := newReporter()
	done := progress.busy("looking")

	addresses := expose.Addresses(ctx, port)
	if exposeflags.check {
		addresses = expose.Check(ctx, addresses)
	}

	done()
	progress.Close()

	for _, address := range addresses {
		line := fmt.Sprintf("  %-14s %s", address.Where.Name(), address.String())

		switch {
		case address.Host == "":
			line = fmt.Sprintf("  %-14s %s", address.Where.Name(), muted("unknown"))
		case address.Checked && address.Answers:
			line += " " + lipgloss.NewStyle().Foreground(theme.Accent).Render("✓ answers")
		case address.Checked:
			line += " " + lipgloss.NewStyle().Foreground(styles.ErrorColor).Render("✕ no answer")
		}

		fmt.Println(line)
		if address.Note != "" {
			fmt.Println(muted("                 " + address.Note))
		}
	}

	fmt.Println()
	fmt.Println(muted("--check asks whether anything answers; --open has the router forward the port"))
}

func init() {
	exposeCmd.Flags().BoolVar(&exposeflags.open, "open", false, "Ask the router to forward the port.")
	exposeCmd.Flags().BoolVar(&exposeflags.off, "off", false, "Take the router's port forward back.")
	exposeCmd.Flags().BoolVar(&exposeflags.check, "check", false, "Check whether the server answers.")

	registerServerFlag(exposeCmd)
	rootCmd.AddCommand(exposeCmd)
}
