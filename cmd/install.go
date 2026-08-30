package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/modrinth"
	"github.com/limelamp/osmium/internal/pack"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/spf13/cobra"
)

// installflags are what the install command was told on the command line.
var installflags struct {
	name  string
	dir   string
	ram   string
	force bool
}

var installCmd = &cobra.Command{
	Use:   "install [manifest|modpack|url]",
	Short: "Build a server from a manifest, a lock or a modpack.",
	Long: fmt.Sprintf(`Builds a whole server from a recipe.

With no argument it reads the %s in the current folder. Given a path or a
URL it reads that instead, and given a .mrpack it installs a Modrinth
modpack. With --server it rebuilds a server Osmium already knows about,
which is how a folder that has lost files is repaired.

What it does is what "create" does, from a file instead of from questions:
downloads the engine, accepts the EULA, writes the run script, registers the
server, and then installs everything the recipe lists. Where there is a %s
the exact builds it names are installed, so the same recipe gives the same
server; where a build has since been withdrawn, that project is resolved
again and the change is reported.

Examples:
  osmium install
  osmium install ./packs/skyblock.json --name skyblock
  osmium install https://example.com/pack/osmium.json
  osmium install cobblemon.mrpack --name cobblemon
  osmium install --server survival`, config.OsmiumFileName, config.LockFileName),
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := ""
		if len(args) == 1 {
			source = args[0]
		}
		return runInstall(source)
	},
}

// runInstall is the whole command: work out what is being installed and where,
// make sure that is allowed, build the server, then fill it.
func runInstall(source string) error {
	progress := newReporter()
	defer progress.Close()

	recipe, err := readRecipe(source)
	if err != nil {
		return err
	}
	defer recipe.cleanup()

	target, err := installTarget(recipe)
	if err != nil {
		return err
	}

	if err := roomToBuild(target, recipe); err != nil {
		return err
	}

	sc := progress.Scope(target)

	// A folder that already has a server in it is being repaired, not built.
	if !hasServerJar(target) {
		if err := buildServer(recipe, target, sc); err != nil {
			return err
		}
	} else {
		fmt.Println(muted("A server is already built here; installing what is missing."))
	}

	if recipe.modpack != nil {
		return installModpack(recipe, target, sc)
	}

	return fill(target, sc)
}

// recipe is what an install was pointed at.
type recipe struct {
	manifest *config.OsmiumConfig
	lock     *config.Lock
	modpack  *pack.Modpack

	// dir is the folder the recipe came from, when it came from one; an
	// install with no argument builds where its manifest already is.
	dir string

	// name is what to call the server, taken from the recipe when the command
	// line did not say.
	name string

	cleanup func()
}

// readRecipe works out what is being installed: the current folder, a file, a
// URL, a modpack, or a server Osmium already knows about.
func readRecipe(source string) (*recipe, error) {
	found := &recipe{cleanup: func() {}}

	// --server names a server that is already registered.
	if serverFlag != "" {
		dir, err := resolveDir()
		if err != nil {
			return nil, err
		}
		source = dir
	}

	if source == "" {
		source = "."
	}

	if isURL(source) {
		local, clean, err := fetchRecipe(source)
		if err != nil {
			return nil, err
		}

		found.cleanup = clean
		source = local
	}

	info, err := os.Stat(source)
	switch {
	case err != nil:
		return nil, fmt.Errorf("%s: %w", source, err)

	case info.IsDir():
		found.dir = source
		source = filepath.Join(source, config.OsmiumFileName)

	case strings.HasSuffix(strings.ToLower(source), ".mrpack"):
		modpack, err := pack.ReadModpack(source)
		if err != nil {
			return nil, err
		}

		found.modpack = modpack
		found.name = modpack.Name
		found.manifest = &config.OsmiumConfig{
			Category: "modded",
			Loader:   modpack.Loader,
			Version:  modpack.Game,
			Mods:     config.Wants{},
			Plugins:  config.Wants{},
		}
		return found, nil

	default:
		found.dir = filepath.Dir(source)
	}

	manifest, err := readManifestFile(source)
	if err != nil {
		return nil, err
	}
	found.manifest = manifest

	if found.dir != "" {
		if lock, err := config.ReadLockIn(found.dir); err == nil {
			found.lock = lock
		}
	}

	if found.name == "" && found.dir != "" {
		found.name = filepath.Base(mustAbs(found.dir))
	}

	return found, nil
}

// readManifestFile reads an osmium.json from anywhere.
func readManifestFile(path string) (*config.OsmiumConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var manifest config.OsmiumConfig
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%s is not an Osmium manifest: %w", filepath.Base(path), err)
	}

	if manifest.Loader == "" || manifest.Version == "" {
		return nil, fmt.Errorf("%s does not say what to run", filepath.Base(path))
	}
	return &manifest, nil
}

// fetchRecipe downloads a manifest or a modpack to a temporary file.
func fetchRecipe(from string) (path string, cleanup func(), err error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, from, http.NoBody)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "limelamp/osmium (github.com/limelamp/osmium)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("%s answered %s", from, resp.Status)
	}

	name := "recipe.json"
	if strings.HasSuffix(strings.ToLower(from), ".mrpack") {
		name = "recipe.mrpack"
	}

	dir, err := os.MkdirTemp("", "osmium-recipe-")
	if err != nil {
		return "", nil, err
	}

	local := filepath.Join(dir, name)
	file, err := os.Create(local) //nolint:gosec // a temporary directory this process just made
	if err != nil {
		return "", nil, err
	}

	// A recipe from the internet is bounded: 512 MB is a very large modpack
	// and an absurd manifest.
	if _, err := io.Copy(file, io.LimitReader(resp.Body, 512<<20)); err != nil {
		_ = file.Close()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		return "", nil, err
	}

	return local, func() { _ = os.RemoveAll(dir) }, nil
}

// installTarget is the folder the server is built in.
func installTarget(found *recipe) (string, error) {
	switch {
	case installflags.dir != "" && installflags.name != "":
		return filepath.Join(installflags.dir, slugOf(installflags.name)), nil

	case installflags.dir != "":
		return filepath.Join(installflags.dir, slugOf(found.name)), nil

	// A manifest read out of a folder builds in that folder: "install here".
	case found.dir != "" && installflags.name == "":
		return mustAbs(found.dir), nil
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return "", err
	}

	name := installflags.name
	if name == "" {
		name = found.name
	}
	if name == "" {
		return "", fmt.Errorf("say what to call the server with --name")
	}

	return filepath.Join(cfg.ServersDir(), slugOf(name)), nil
}

// roomToBuild refuses to build into a folder that already holds something
// other than an Osmium recipe.
//
// Building on top of somebody's files is how a world gets lost. A folder with
// nothing but osmium.json and osmium.lock in it is a recipe waiting to be
// built and is fine; anything else has to be said out loud with --force.
func roomToBuild(target string, found *recipe) error {
	// Building where the recipe already lives is a repair, not an import.
	if found.dir != "" && mustAbs(found.dir) == mustAbs(target) {
		return nil
	}

	entries, err := os.ReadDir(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var occupied []string
	for _, entry := range entries {
		switch entry.Name() {
		case config.OsmiumFileName, config.LockFileName, ".osmium":
			continue
		}
		occupied = append(occupied, entry.Name())
	}

	if len(occupied) == 0 {
		return nil
	}

	fmt.Printf("%s is not empty: %s\n", target, joinArgs(occupied[:min(len(occupied), 6)]))

	if installflags.force || assumeYes {
		return nil
	}

	fmt.Println("Installing here would write into it.")
	if !confirm() {
		return fmt.Errorf("nothing was changed")
	}
	return nil
}

// buildServer downloads the engine and writes everything a server needs to
// start, then registers it.
func buildServer(found *recipe, target string, sc scope.Scope) error {
	name := installflags.name
	if name == "" {
		name = found.name
	}
	if name == "" {
		name = filepath.Base(target)
	}

	ram := installflags.ram
	if ram == "" {
		ram = found.manifest.AllocatedRAM
	}
	if ram == "" {
		ram = "4G"
	}

	cfg := &steps.Config{
		Name:     name,
		Dir:      filepath.Dir(target),
		Software: found.manifest.Loader,
		Category: found.manifest.Category,
		Version:  found.manifest.Version,
		RAM:      ram,
		Eula:     true,
	}

	fmt.Printf("Building %s: %s %s, %s\n", name, cfg.Software, cfg.Version, ram)

	if _, err := steps.Build(cfg, storage.NewServerStore(), sc); err != nil {
		return err
	}

	// The recipe's own list is what makes this a rebuild rather than an empty
	// server of the same version.
	built, err := config.ReadConfigIn(target)
	if err != nil {
		return err
	}

	built.Mods, built.Plugins = found.manifest.Mods, found.manifest.Plugins
	if err := config.WriteConfigIn(target, built); err != nil {
		return err
	}

	if found.lock != nil && len(found.lock.Packages) > 0 {
		if err := config.WriteLockIn(target, found.lock); err != nil {
			return err
		}
	}

	return nil
}

// fill installs the mods and plugins the recipe asks for: exactly what the
// lock names where there is one, resolved afresh where there is not.
func fill(target string, sc scope.Scope) error {
	server, err := pack.Open(target)
	if err != nil {
		return err
	}

	store := downloadCache()
	resolve := &pack.Resolver{Client: modrinth.New(store), Server: server, Cache: store}

	for _, kind := range []config.Kind{config.Mods, config.Plugins} {
		if len(server.Conf.Want(kind)) == 0 && !resolve.Locked(kind) {
			continue
		}

		if resolve.Locked(kind) {
			plan := resolve.Restore(kind)
			show(plan)

			err := server.Apply(context.Background(), plan, pack.Options{Cache: store, Scope: sc})
			if err == nil {
				continue
			}

			fmt.Println(muted("the lock could not be satisfied (" + err.Error() + "), resolving instead"))
		}

		plan, err := resolve.Sync(context.Background(), kind)
		if err != nil {
			return err
		}

		show(plan)

		if err := server.Apply(context.Background(), plan, pack.Options{Cache: store, Scope: sc}); err != nil {
			return err
		}
	}

	return nil
}

// installModpack puts a Modrinth modpack's files in place and then works out
// what they are, so the result is a properly locked server rather than a
// folder of anonymous jars.
func installModpack(found *recipe, target string, sc scope.Scope) error {
	store := downloadCache()

	written, err := found.modpack.ExtractOverrides(target)
	if err != nil {
		return err
	}
	if written > 0 {
		fmt.Printf("Wrote %d of the pack's own files.\n", written)
	}

	installed, err := found.modpack.InstallInto(context.Background(), target,
		pack.Options{Cache: store, Scope: sc})
	if err != nil {
		return err
	}
	fmt.Printf("Installed %d files from %s.\n", installed, found.modpack.Name)

	server, err := pack.Open(target)
	if err != nil {
		return err
	}

	resolve := &pack.Resolver{Client: modrinth.New(store), Server: server, Cache: store}

	adopted, err := resolve.Track(context.Background(), server.Kind())
	if err != nil {
		return err
	}

	fmt.Printf("Identified %d of them.\n", len(adopted))
	return nil
}

// hasServerJar reports whether a folder already holds a built server.
func hasServerJar(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
			return true
		}
	}
	return false
}

// isURL reports whether a source is something to download.
func isURL(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}

// mustAbs is an absolute path, or the path as given when it cannot be made one.
func mustAbs(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absolute
}

// slugOf is the folder name a server name becomes.
func slugOf(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")

	kept := make([]rune, 0, len(slug))
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			kept = append(kept, r)
		}
	}

	if len(kept) == 0 {
		return "server"
	}
	return string(kept)
}

func init() {
	installCmd.Flags().StringVar(&installflags.name, "name", "", "What to call the server.")
	installCmd.Flags().StringVar(&installflags.dir, "dir", "", "Where to build it.")
	installCmd.Flags().StringVar(&installflags.ram, "ram", "", "How much memory to give it.")
	installCmd.Flags().BoolVar(&installflags.force, "force", false, "Build even if the folder is not empty.")

	registerServerFlag(installCmd)
	registerPackFlags(installCmd, true)

	rootCmd.AddCommand(installCmd)
}
