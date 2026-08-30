// Package pack is Osmium's package manager: what a server asks for, what that
// resolves to, and how the two are brought together.
//
// It works the way the package managers people already know do. The manifest
// states intent — these mods, this one held at that build — and the lock
// records exactly what the intent resolved to, so the same manifest installs
// the same files tomorrow. Nothing is written until the whole graph resolves
// and every file has arrived and been checked, so a failure halfway leaves a
// server exactly as it was rather than half upgraded. And what Osmium put
// there is recorded, so what it did not put there is never touched.
package pack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/limelamp/osmium/internal/config"
)

// Server is one server as the package manager sees it: a directory, what it
// asks for, and what that has resolved to.
type Server struct {
	Dir  string
	Conf *config.OsmiumConfig
	Lock *config.Lock
}

// Open reads a server's manifest and lock.
//
// A server whose manifest predates the lock file gets one seeded from what the
// manifest recorded as installed, so the first command run against it starts
// from what is actually on disk instead of deciding everything is missing.
func Open(dir string) (*Server, error) {
	conf, err := config.ReadConfigIn(dir)
	if err != nil {
		return nil, fmt.Errorf("no %s in %s: %w", config.OsmiumFileName, dir, err)
	}

	lock, err := config.ReadLockIn(dir)
	if err != nil {
		return nil, err
	}

	if len(lock.Packages) == 0 {
		if seeded := config.SeedLock(conf); len(seeded.Packages) > 0 {
			lock = seeded
		}
	}

	lock.Loader, lock.Game = conf.Loader, conf.Version

	return &Server{Dir: dir, Conf: conf, Lock: lock}, nil
}

// Kind is the sort of project this server takes: plugins for a plugin server,
// mods for everything else.
func (s *Server) Kind() config.Kind { return config.KindFor(s.Conf.Loader) }

// Folder is where a kind's files live in this server.
func (s *Server) Folder(kind config.Kind) string {
	return filepath.Join(s.Dir, kind.Folder())
}

// Loaders are the loader names a project must declare to fit this server, in
// order of preference. A Paper server also loads Spigot and Bukkit plugins,
// and saying so is what lets a plugin written for Bukkit in 2016 install.
func (s *Server) Loaders() []string {
	loader := strings.ToLower(s.Conf.Loader)

	switch loader {
	case "purpur":
		return []string{"purpur", "paper", "spigot", "bukkit"}
	case "folia":
		return []string{"folia", "paper", "spigot", "bukkit"}
	case "paper":
		return []string{"paper", "spigot", "bukkit"}
	case "spigot":
		return []string{"spigot", "bukkit"}
	case "bukkit":
		return []string{"bukkit"}
	case "quilt":
		// Quilt loads Fabric mods, and most mods only ever publish for Fabric.
		return []string{"quilt", "fabric"}
	case "":
		return nil
	default:
		return []string{loader}
	}
}

// Save writes the manifest and the lock back.
func (s *Server) Save() error {
	if err := config.WriteConfigIn(s.Dir, s.Conf); err != nil {
		return err
	}
	return config.WriteLockIn(s.Dir, s.Lock)
}

// Installed reports the file a locked project has in the server, and whether
// it is actually there. A lock entry whose file has been deleted by hand is
// what makes an install non-empty when nothing about the manifest changed.
func (s *Server) Installed(entry config.Locked) (string, bool) {
	if entry.FileName == "" {
		return "", false
	}

	path := filepath.Join(s.Folder(entry.Kind), entry.FileName)
	info, err := os.Stat(path)

	return path, err == nil && !info.IsDir()
}

// Untracked are the jars in a kind's folder that the lock knows nothing about
// — dropped in by hand, or left by something other than Osmium.
//
// They are never touched. Reporting them is the whole of the policy: a server
// folder belongs to the person running it, and a package manager that deletes
// what it did not install is one that eventually deletes something that
// mattered.
func (s *Server) Untracked(kind config.Kind) []string {
	entries, err := os.ReadDir(s.Folder(kind))
	if err != nil {
		return nil
	}

	known := s.Lock.Files(kind)

	var loose []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".jar") {
			continue
		}
		if _, tracked := known[name]; !tracked {
			loose = append(loose, name)
		}
	}

	return loose
}

// Dependants are the projects that need the given one, which is what a
// removal has to know before it takes something away.
func (s *Server) Dependants(kind config.Kind, slug string) []string {
	entry, locked := s.Lock.Find(kind, slug)
	if !locked {
		return nil
	}
	return entry.RequiredBy
}

// ErrNoServer says a directory holds no Osmium server.
var ErrNoServer = errors.New("no osmium.json here")

// IsServer reports whether a directory holds a manifest.
func IsServer(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, config.OsmiumFileName))
	return err == nil
}
