package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/spf13/cobra"
)

// The two kinds of project, and the folder each lives in. Everything below is
// written once and registered twice: a mod and a plugin differ by the folder
// they go in and the word used for them, and by nothing else.
var projectKinds = []struct {
	noun   string
	folder string
}{
	{"mod", "mods"},
	{"plugin", "plugins"},
}

// newProjectCommand builds the "osmium mod …" or "osmium plugin …" group.
//
// These replace the old top-level add/remove/install/update/track commands and
// their --mod/--plugin flags: the group already says which kind is meant, so
// the flag had nothing left to say.
func newProjectCommand(noun, folder string) *cobra.Command {
	group := &cobra.Command{
		Use:   noun,
		Short: "Manage the " + noun + "s of a server.",
		Long: fmt.Sprintf(`Adds, removes and updates the %ss of a server.

Every subcommand acts on the current folder, or on the server named with
--server.

Examples:
  osmium %s add sodium
  osmium %s list --server survival
  osmium %s update`, noun, noun, noun, noun),
	}

	add := &cobra.Command{
		Use:               "add [project-id ...]",
		Short:             "Add one or more Modrinth projects.",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeModrinth,
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, progress, err := resolveScope()
			if err != nil {
				return err
			}
			defer progress.Close()

			for _, id := range args {
				done := progress.busy("Resolving " + id + "…")
				err := shared.AddProjectByID(sc, id, folder)
				done()

				if err != nil {
					fmt.Println(err)
				}
			}
			return nil
		},
	}

	remove := &cobra.Command{
		Use:               "remove [project-id ...]",
		Short:             "Remove one or more tracked projects.",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeInstalledIn(folder),
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, progress, err := resolveScope()
			if err != nil {
				return err
			}
			defer progress.Close()

			for _, id := range args {
				if err := shared.RemoveProjectByID(sc, id, folder); err != nil {
					fmt.Println(err)
				}
			}
			return nil
		},
	}

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the tracked " + noun + "s.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := requireServerDir()
			if err != nil {
				return err
			}
			return listProjects(dir, noun, folder)
		},
	}

	install := &cobra.Command{
		Use:   "install",
		Short: "Install everything tracked in osmium.json.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, progress, err := resolveScope()
			if err != nil {
				return err
			}
			defer progress.Close()

			done := progress.busy("Installing tracked " + noun + "s…")
			defer done()

			return shared.InstallProjectsFromConfig(sc)
		},
	}

	update := &cobra.Command{
		Use:               "update [project-id ...]",
		Short:             "Update tracked " + noun + "s to their newest builds.",
		ValidArgsFunction: completeInstalledIn(folder),
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, progress, err := resolveScope()
			if err != nil {
				return err
			}
			defer progress.Close()

			if len(args) == 0 {
				done := progress.busy("Updating every " + noun + "…")
				defer done()

				return shared.UpdateAllProjects(sc, folder)
			}

			for _, id := range args {
				done := progress.busy("Updating " + id + "…")
				err := shared.UpdateProject(sc, id, folder)
				done()

				if err != nil {
					fmt.Println(err)
				}
			}
			return nil
		},
	}

	track := &cobra.Command{
		Use:   "track",
		Short: "Adopt jars already in the folder into osmium.json.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, progress, err := resolveScope()
			if err != nil {
				return err
			}
			defer progress.Close()

			done := progress.busy("Identifying jars…")
			defer done()

			return shared.TrackProjects(sc)
		},
	}

	group.AddCommand(add, remove, list, install, update, track)

	// The flag is on every subcommand rather than on the group, so that
	// "osmium mod add sodium -s survival" reads the way the others do.
	for _, cmd := range group.Commands() {
		registerServerFlag(cmd)
	}
	return group
}

// listProjects prints what a server has tracked, with the version installed.
func listProjects(dir, noun, folder string) error {
	conf, err := config.ReadConfigIn(dir)
	if err != nil {
		return err
	}

	tracked := conf.Mods
	if folder == "plugins" {
		tracked = conf.Plugins
	}

	if len(tracked) == 0 {
		fmt.Printf("No %ss tracked in %s.\n", noun, config.OsmiumFileName)
		return nil
	}

	slugs := make([]string, 0, len(tracked))
	for slug := range tracked {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(table, "%s\tVERSION\tFILE\n", "NAME")

	for _, slug := range slugs {
		project := tracked[slug]
		fmt.Fprintf(table, "%s\t%s\t%s\n", slug, project.VersionNumber, project.FileName)
	}

	return table.Flush()
}

// completeInstalledIn completes the slugs tracked for one kind of project.
func completeInstalledIn(folder string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		dir, err := resolveDir()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		conf, err := config.ReadConfigIn(dir)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		tracked := conf.Mods
		if folder == "plugins" {
			tracked = conf.Plugins
		}

		chosen := make(map[string]bool, len(args))
		for _, arg := range args {
			chosen[arg] = true
		}

		slugs := make([]string, 0, len(tracked))
		for slug, project := range tracked {
			if !chosen[slug] {
				slugs = append(slugs, slug+"\t"+project.VersionNumber)
			}
		}
		return slugs, cobra.ShellCompDirectiveNoFileComp
	}
}

func init() {
	for _, kind := range projectKinds {
		rootCmd.AddCommand(newProjectCommand(kind.noun, kind.folder))
	}
}
