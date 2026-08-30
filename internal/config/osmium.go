package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// OsmiumFileName is the per-server manifest Osmium writes next to the jar.
const OsmiumFileName = "osmium.json"

// Newest is what a manifest entry says when it wants whichever build is the
// newest one compatible with the server.
const Newest = "*"

// Want is what the manifest asks for: the newest compatible build, or one
// exact build named by its Modrinth version number.
//
// Minecraft mods do not carry version numbers a range could be written
// against — an author may call a build 1.10.5-fabric, 0.102.0+1.21.1 or
// mc1.21.1-0.5.8 — so there is nothing here to compare or order. A manifest
// entry is therefore one of two things and never a third: "keep this current",
// or "this exact build and no other".
type Want string

// Pinned reports whether this entry names one particular build.
func (w Want) Pinned() bool { return w != "" && string(w) != Newest }

// String is the manifest's own form.
func (w Want) String() string {
	if w == "" {
		return Newest
	}
	return string(w)
}

// Wants is a set of projects a server asks for, keyed by Modrinth slug.
type Wants map[string]Want

// Slugs are the projects asked for, in a stable order so that what is written
// and what is printed do not shuffle between runs.
func (w Wants) Slugs() []string {
	slugs := make([]string, 0, len(w))
	for slug := range w {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs
}

// UnmarshalJSON reads both forms of the manifest.
//
// Osmium used to write an object per project, holding the build that happened
// to be installed along with its file name and hash. That is a record of what
// is installed, which now belongs in the lock, so an old entry is read as
// "keep this current" and the build it names is recovered separately by
// Legacy, to seed a lock for a server that has never had one.
func (w *Wants) UnmarshalJSON(data []byte) error {
	var direct map[string]json.RawMessage
	if err := json.Unmarshal(data, &direct); err != nil {
		return err
	}

	*w = make(Wants, len(direct))
	for slug, raw := range direct {
		var want string
		if err := json.Unmarshal(raw, &want); err == nil {
			(*w)[slug] = Want(want)
			continue
		}

		// The old object form: what it asked for was always "the newest".
		(*w)[slug] = Newest
	}

	return nil
}

// LegacyProject is one entry of the manifest Osmium used to write. It is only
// read, never written, and only to give a server that predates the lock file
// something to lock.
type LegacyProject struct {
	VersionNumber string `json:"version_number"`
	FileName      string `json:"filename"`
	DownloadURL   string `json:"url"`
	SHA1          string `json:"sha1"`
}

// OsmiumConfig is a server's manifest: what it runs, how much memory it is
// given, and which mods and plugins were asked for. It lives beside the jar as
// osmium.json and is the statement of intent; osmium.lock beside it is the
// record of what that intent resolved to.
type OsmiumConfig struct {
	Category     string `json:"category"`
	Loader       string `json:"loader"`
	Version      string `json:"version"`
	AllocatedRAM string `json:"ram"`
	Mods         Wants  `json:"mods"`
	Plugins      Wants  `json:"plugins"`

	// legacy holds what an old manifest recorded as installed, so a lock can
	// be seeded from it once. It is never written back.
	legacyMods    map[string]LegacyProject
	legacyPlugins map[string]LegacyProject
}

// Legacy is what an old manifest said was installed, or nothing for a manifest
// already in the current form.
func (c *OsmiumConfig) Legacy() (mods, plugins map[string]LegacyProject) {
	return c.legacyMods, c.legacyPlugins
}

// Want returns what the manifest asks for in one of its two sets.
func (c *OsmiumConfig) Want(kind Kind) Wants {
	if kind == Plugins {
		if c.Plugins == nil {
			c.Plugins = Wants{}
		}
		return c.Plugins
	}

	if c.Mods == nil {
		c.Mods = Wants{}
	}
	return c.Mods
}

// Kind is which of a server's two sets a project belongs to. They are separate
// because they are installed into different folders and are never
// interchangeable: a Fabric mod in plugins/ is ignored, and a Paper plugin in
// mods/ is ignored just as quietly.
type Kind string

// The two kinds, whose values are also the folders they live in.
const (
	Mods    Kind = "mods"
	Plugins Kind = "plugins"
)

// Folder is where this kind's files belong inside a server.
func (k Kind) Folder() string { return string(k) }

// Noun names one project of this kind, for a sentence about it.
func (k Kind) Noun() string {
	if k == Plugins {
		return "plugin"
	}
	return "mod"
}

// KindFor is the kind of project a server takes, which follows from what it
// runs: a plugin server loads plugins, and everything else loads mods.
func KindFor(loader string) Kind {
	switch strings.ToLower(loader) {
	case "paper", "spigot", "bukkit", "purpur", "folia":
		return Plugins
	default:
		return Mods
	}
}

// WriteConfigIn writes the manifest into the given server directory.
func WriteConfigIn(dir string, config *OsmiumConfig) error {
	bytes, err := json.MarshalIndent(config, "", " ")
	if err != nil {
		return fmt.Errorf("failed to marshal Osmium config: %w", err)
	}

	path := filepath.Join(dir, OsmiumFileName)
	if err := os.WriteFile(path, append(bytes, '\n'), 0o644); err != nil { //nolint:gosec // a manifest is meant to be readable
		return fmt.Errorf("failed to write Osmium config to %s: %w", path, err)
	}

	return nil
}

// ReadConfigIn reads the manifest from the given server directory.
func ReadConfigIn(dir string) (*OsmiumConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, OsmiumFileName))
	if err != nil {
		return nil, err
	}

	var config OsmiumConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.Mods == nil {
		config.Mods = Wants{}
	}
	if config.Plugins == nil {
		config.Plugins = Wants{}
	}

	config.legacyMods, config.legacyPlugins = readLegacy(data)
	return &config, nil
}

// readLegacy digs the old per-project objects out of a manifest, ignoring
// anything already in the current form.
func readLegacy(data []byte) (mods, plugins map[string]LegacyProject) {
	var old struct {
		Mods    map[string]LegacyProject `json:"mods"`
		Plugins map[string]LegacyProject `json:"plugins"`
	}

	// A manifest in the current form has strings where this wants objects, so
	// the decode fails and there is simply nothing to recover.
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, nil
	}

	return prune(old.Mods), prune(old.Plugins)
}

// prune drops entries that carry nothing worth locking.
func prune(entries map[string]LegacyProject) map[string]LegacyProject {
	kept := make(map[string]LegacyProject, len(entries))
	for slug, project := range entries {
		if project.FileName != "" || project.SHA1 != "" {
			kept[slug] = project
		}
	}

	if len(kept) == 0 {
		return nil
	}
	return kept
}

// WriteConfig writes the manifest into the process working directory. The CLI
// commands run from inside a server folder, so this is what they want.
func WriteConfig(config *OsmiumConfig) error {
	return WriteConfigIn(".", config)
}

// ReadConfig reads the manifest from the process working directory.
func ReadConfig() (*OsmiumConfig, error) {
	return ReadConfigIn(".")
}
