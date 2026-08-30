package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/cache"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/modrinth"
	"github.com/limelamp/osmium/internal/pack"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
	"github.com/spf13/cobra"
)

// Flags shared by every command that changes what is installed.
var (
	assumeYes  bool
	prerelease bool
	noCache    bool
)

// registerPackFlags puts the package-manager flags on a command.
func registerPackFlags(cmd *cobra.Command, channel bool) {
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Do not ask before applying the plan.")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Ignore the download cache for this run.")

	if channel {
		cmd.Flags().BoolVar(&prerelease, "pre", false, "Allow beta and alpha builds.")
	}
}

// downloadCache is the file cache this run should use.
func downloadCache() *cache.Store {
	if noCache {
		return cache.Off()
	}

	opened, err := cache.Open()
	if err != nil {
		return cache.Off()
	}
	return opened
}

// resolver opens the server a command is about to act on and gives back
// everything needed to plan and apply against it.
func resolver() (*pack.Resolver, *reporter, error) {
	dir, err := requireServerDir()
	if err != nil {
		return nil, nil, err
	}

	server, err := pack.Open(dir)
	if err != nil {
		return nil, nil, err
	}

	shelf := downloadCache()
	progress := newReporter()

	return &pack.Resolver{
		Client:     modrinth.New(shelf),
		Server:     server,
		Cache:      shelf,
		Prerelease: prerelease,
	}, progress, nil
}

// carryOut shows a plan, asks about it when something would be lost, and
// applies it.
//
// Every command that changes a server ends here, so they all read the same
// way: what will happen, then a question if anything is at stake, then the
// work, then what happened.
func carryOut(resolver *pack.Resolver, progress *reporter, plan pack.Plan, err error) error {
	defer progress.Close()

	if err != nil {
		return err
	}

	show(plan)

	if plan.Blocked() {
		return fmt.Errorf("nothing was changed")
	}
	if plan.Empty() {
		// The manifest may still have moved — a pin added, a mod named that
		// was already installed — so it is written even with nothing to fetch.
		return resolver.Server.Apply(context.Background(), plan, pack.Options{
			Cache: resolver.Cache, Scope: progress.Scope(resolver.Server.Dir),
		})
	}

	if plan.Loses() && !assumeYes && !confirm() {
		return fmt.Errorf("nothing was changed")
	}

	return resolver.Server.Apply(context.Background(), plan, pack.Options{
		Cache: resolver.Cache,
		Scope: progress.Scope(resolver.Server.Dir),
	})
}

// show prints a plan the way a package manager does: the summary first,
// because that is the part that is read, then the detail.
func show(plan pack.Plan) {
	accent := lipgloss.NewStyle().Foreground(theme.Accent)
	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	if plan.Blocked() {
		for _, conflict := range plan.Conflicts {
			fmt.Println(lipgloss.NewStyle().Foreground(styles.ErrorColor).Render("✕ " + conflict))
		}
		return
	}

	if plan.Empty() {
		fmt.Println(muted.Render("Nothing to do."))
		printExtras(plan, muted)
		return
	}

	fmt.Println(accent.Render(plan.Summary()))
	fmt.Println()

	for _, change := range plan.Changes {
		fmt.Println("  " + change.Line())
	}

	printExtras(plan, muted)
	fmt.Println()
}

// printExtras writes what a plan noticed but is not doing.
func printExtras(plan pack.Plan, muted lipgloss.Style) {
	for _, note := range plan.Notes {
		fmt.Println(muted.Render("  " + note))
	}

	if len(plan.Optional) > 0 {
		fmt.Println()
		fmt.Println(muted.Render("optional, not installed:"))
		for _, suggestion := range plan.Optional {
			fmt.Println(muted.Render(fmt.Sprintf("  %s  (suggested by %s)", suggestion.Name(), suggestion.Wanted)))
		}
	}

	if len(plan.Untracked) > 0 {
		fmt.Println()
		fmt.Println(muted.Render("not managed by Osmium:"))
		fmt.Println(muted.Render("  " + strings.Join(plan.Untracked, ", ")))
		fmt.Println(muted.Render("  adopt them with: osmium mod track"))
	}
}

// confirm asks before something is taken away.
func confirm() bool {
	fmt.Print("Continue? [Y/n] ")

	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "", "y", "yes":
		return true
	default:
		return false
	}
}

// kindOf is the folder a noun means.
func kindOf(noun string) config.Kind {
	if noun == "plugin" {
		return config.Plugins
	}
	return config.Mods
}
