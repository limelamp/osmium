package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/pack"
	"github.com/limelamp/osmium/internal/tui/theme"
	"github.com/spf13/cobra"
)

// The two kinds of project. Everything below is written once and registered
// twice: a mod and a plugin differ by the folder they go in and the word used
// for them, and by nothing else.
var projectKinds = []string{"mod", "plugin"}

// newProjectCommand builds the "osmium mod …" or "osmium plugin …" group.
func newProjectCommand(noun string) *cobra.Command {
	kind := kindOf(noun)

	group := &cobra.Command{
		Use:   noun,
		Short: "Manage the " + noun + "s of a server.",
		Long: fmt.Sprintf(`Adds, removes, updates and pins the %ss of a server.

Every subcommand works out the whole change first — including anything a
%s needs — shows it, and applies it in one go. Nothing is written until
every file has arrived and been checked, so a failure leaves the server as
it was. What is asked for is recorded in %s; what that resolved to is
recorded in %s.

Every subcommand acts on the current folder, or on the server named with
--server.

Examples:
  osmium %s add sodium
  osmium %s add sodium@0.5.8      # a particular build, held there
  osmium %s update                # everything that is not pinned
  osmium %s versions sodium       # what this server could install`,
			noun, noun, config.OsmiumFileName, config.LockFileName, noun, noun, noun, noun),
	}

	add := &cobra.Command{
		Use:               "add <project>[@build] ...",
		Short:             "Add " + noun + "s, with whatever they need.",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeModrinth,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolve, progress, err := resolver()
			if err != nil {
				return err
			}

			done := progress.busy("resolving")
			plan, err := resolve.Add(context.Background(), kind, args)
			done()

			return carryOut(resolve, progress, plan, err)
		},
	}

	remove := &cobra.Command{
		Use:               "remove <project> ...",
		Short:             "Remove " + noun + "s, and anything left needed by nothing.",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeInstalledIn(kind),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolve, progress, err := resolver()
			if err != nil {
				return err
			}

			done := progress.busy("resolving")
			plan, err := resolve.Remove(context.Background(), kind, args)
			done()

			return carryOut(resolve, progress, plan, err)
		},
	}

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the installed " + noun + "s.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := requireServerDir()
			if err != nil {
				return err
			}
			return listProjects(dir, noun, kind)
		},
	}

	install := &cobra.Command{
		Use:   "install",
		Short: "Install exactly what the manifest and lock describe.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolve, progress, err := resolver()
			if err != nil {
				return err
			}

			done := progress.busy("resolving")
			plan, err := resolve.Sync(context.Background(), kind)
			done()

			return carryOut(resolve, progress, plan, err)
		},
	}

	update := &cobra.Command{
		Use:               "update [project ...]",
		Short:             "Move " + noun + "s to their newest builds, leaving pins alone.",
		ValidArgsFunction: completeInstalledIn(kind),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolve, progress, err := resolver()
			if err != nil {
				return err
			}

			done := progress.busy("looking for newer builds")
			plan, err := resolve.Update(context.Background(), kind, args)
			done()

			return carryOut(resolve, progress, plan, err)
		},
	}

	track := &cobra.Command{
		Use:   "track",
		Short: "Adopt jars already in the folder.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolve, progress, err := resolver()
			if err != nil {
				return err
			}
			defer progress.Close()

			done := progress.busy("identifying jars")
			adopted, err := resolve.Track(context.Background(), kind)
			done()

			if err != nil {
				return err
			}
			if len(adopted) == 0 {
				fmt.Println(muted("Every jar in the folder is already accounted for."))
				return nil
			}

			for _, file := range adopted {
				fmt.Println("  " + file.Line())
			}
			return nil
		},
	}

	pin := &cobra.Command{
		Use:               "pin <project>[@build]",
		Short:             "Hold a " + noun + " at one build.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeInstalledIn(kind),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolve, progress, err := resolver()
			if err != nil {
				return err
			}

			slug, build := pack.SplitRef(args[0])

			done := progress.busy("resolving")
			plan, err := resolve.Pin(context.Background(), kind, slug, build)
			done()

			return carryOut(resolve, progress, plan, err)
		},
	}

	unpin := &cobra.Command{
		Use:               "unpin <project>",
		Short:             "Let a " + noun + " move with the rest again.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeInstalledIn(kind),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolve, progress, err := resolver()
			if err != nil {
				return err
			}

			done := progress.busy("resolving")
			plan, err := resolve.Unpin(context.Background(), kind, args[0])
			done()

			return carryOut(resolve, progress, plan, err)
		},
	}

	versions := &cobra.Command{
		Use:               "versions <project>",
		Short:             "List the builds this server could install.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeModrinth,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolve, progress, err := resolver()
			if err != nil {
				return err
			}
			defer progress.Close()

			done := progress.busy("asking Modrinth")
			choices, err := resolve.Versions(context.Background(), kind, args[0])
			done()

			if err != nil {
				return err
			}
			return listVersions(choices)
		},
	}

	search := &cobra.Command{
		Use:   "search <words>",
		Short: "Find " + noun + "s that fit this server.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolve, progress, err := resolver()
			if err != nil {
				return err
			}
			defer progress.Close()

			done := progress.busy("searching")
			found, err := resolve.Search(context.Background(), joinArgs(args), 15)
			done()

			if err != nil {
				return err
			}

			table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(table, "NAME\tSLUG")
			for _, project := range found {
				fmt.Fprintf(table, "%s\t%s\n", project.Title, project.Slug)
			}
			return table.Flush()
		},
	}

	group.AddCommand(add, remove, list, install, update, track, pin, unpin, versions, search)

	for _, cmd := range group.Commands() {
		registerServerFlag(cmd)
	}
	for _, cmd := range []*cobra.Command{add, remove, install, update, pin, unpin} {
		registerPackFlags(cmd, cmd != remove)
	}

	return group
}

// listProjects prints what a server has, with where each came from.
func listProjects(dir, noun string, kind config.Kind) error {
	server, err := pack.Open(dir)
	if err != nil {
		return err
	}

	installed := server.List(kind)
	if len(installed) == 0 {
		fmt.Printf("No %ss installed.\n", noun)
		return nil
	}

	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tBUILD\tWHY\tFILE")

	for _, entry := range installed {
		why := "asked for"
		switch {
		case entry.Pinned:
			why = "pinned"
		case !entry.Explicit && len(entry.RequiredBy) > 0:
			why = "needed by " + entry.RequiredBy[0]
		case !entry.Explicit:
			why = "needed"
		}

		name := entry.Slug
		if !entry.Present {
			name += " (missing)"
		}

		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", name, entry.VersionNumber, why, entry.FileName)
	}

	if err := table.Flush(); err != nil {
		return err
	}

	if loose := server.Untracked(kind); len(loose) > 0 {
		fmt.Println()
		fmt.Println(muted(fmt.Sprintf("%d file(s) not managed by Osmium: %s", len(loose), joinArgs(loose))))
		fmt.Println(muted("adopt them with: osmium " + noun + " track"))
	}

	return nil
}

// listVersions prints the builds a project has for this server.
func listVersions(choices []pack.VersionChoice) error {
	if len(choices) == 0 {
		fmt.Println("No builds for this server's loader and Minecraft version.")
		return nil
	}

	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "BUILD\tCHANNEL\tSIZE\t")

	for i, choice := range choices {
		mark := ""
		switch {
		case choice.Installed:
			mark = "installed"
		case i == 0:
			mark = "newest"
		}

		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n",
			choice.VersionNumber, choice.Channel, pack.FormatBytes(choice.Size), mark)
	}

	return table.Flush()
}

// completeInstalledIn completes the projects a server has.
func completeInstalledIn(kind config.Kind) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		dir, err := resolveDir()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		server, err := pack.Open(dir)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		chosen := make(map[string]bool, len(args))
		for _, arg := range args {
			chosen[arg] = true
		}

		slugs := make([]string, 0, len(server.Lock.Of(kind)))
		for _, entry := range server.Lock.Of(kind) {
			if !chosen[entry.Slug] {
				slugs = append(slugs, entry.Slug+"\t"+entry.VersionNumber)
			}
		}
		return slugs, cobra.ShellCompDirectiveNoFileComp
	}
}

// muted renders a line that is not the point of the output.
func muted(text string) string {
	return lipgloss.NewStyle().Foreground(theme.Inactive).Render(text)
}

// joinArgs is a comma-separated list, for a sentence.
func joinArgs(values []string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += ", "
		}
		out += value
	}
	return out
}

func init() {
	for _, noun := range projectKinds {
		rootCmd.AddCommand(newProjectCommand(noun))
	}
}
