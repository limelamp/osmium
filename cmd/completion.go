package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/constants"
	"github.com/limelamp/osmium/internal/util"
	"github.com/spf13/cobra"
)

// Tab completion is expected to be instant, so anything that has to leave the
// machine gets a short deadline and simply offers nothing when it misses it.
const completionTimeout = 800 * time.Millisecond

// completeServerNames offers the registered servers, annotated with what they
// are and whether they are up, which is what the shell shows beside each entry.
func completeServerNames(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	servers, err := store.LoadAll()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	names := make([]string, 0, len(servers))
	for _, server := range servers {
		state := "stopped"
		if supervisor.Running(server.Path) {
			state = "running"
		}
		names = append(names, fmt.Sprintf("%s\t%s %s · %s", server.Name, server.Type, server.Version, state))
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeInstalled offers the mods or plugins the target server has tracked,
// which is what remove and update operate on.
func completeInstalled(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	dir, err := resolveDir()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	conf, err := config.ReadConfigIn(dir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	tracked := conf.Mods
	if plugin, _ := cmd.Flags().GetBool("plugin"); plugin {
		tracked = conf.Plugins
	}

	// Something already named on the command line is not offered twice.
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

// completeLoaders offers the server software Osmium knows how to install.
func completeLoaders(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	loaders := append([]string{"Vanilla"}, constants.PluginOptions...)
	loaders = append(loaders, constants.ModLoaderOptions...)
	return loaders, cobra.ShellCompDirectiveNoFileComp
}

// completeVersions offers released Minecraft versions, narrowed to what has
// been typed so far. The manifest is a network call, so a slow connection
// simply completes nothing rather than hanging the shell.
func completeVersions(_ *cobra.Command, _ []string, typed string) ([]string, cobra.ShellCompDirective) {
	done := make(chan []string, 1)

	go func() {
		versions, err := util.GetVersionStrings("release")
		if err != nil {
			done <- nil
			return
		}
		done <- versions
	}()

	select {
	case versions := <-done:
		matching := make([]string, 0, 24)
		for _, version := range versions {
			if strings.HasPrefix(version, typed) {
				matching = append(matching, version)
			}
			if len(matching) == 24 {
				break
			}
		}
		return matching, cobra.ShellCompDirectiveNoFileComp

	case <-time.After(completionTimeout):
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// modrinthHit is the part of a search result completion needs.
type modrinthHit struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// completeModrinth searches Modrinth for whatever has been typed, so adding a
// mod completes against the real catalogue rather than against nothing.
func completeModrinth(_ *cobra.Command, _ []string, typed string) ([]string, cobra.ShellCompDirective) {
	return searchModrinth(typed), cobra.ShellCompDirectiveNoFileComp
}

// searchModrinth returns candidate slugs for a typed prefix.
func searchModrinth(typed string) []string {
	if len(typed) < 2 {
		return nil
	}

	client := &http.Client{Timeout: completionTimeout}
	endpoint := "https://api.modrinth.com/v2/search?limit=15&query=" + url.QueryEscape(typed)

	response, err := client.Get(endpoint)
	if err != nil {
		return nil
	}
	defer response.Body.Close()

	var body struct {
		Hits []modrinthHit `json:"hits"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return nil
	}

	slugs := make([]string, 0, len(body.Hits))
	for _, hit := range body.Hits {
		slugs = append(slugs, hit.Slug+"\t"+hit.Title)
	}
	return slugs
}
