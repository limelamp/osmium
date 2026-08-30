package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// LockFileName is the record of what a server's manifest resolved to. It sits
// beside osmium.json and is what makes an install reproducible: the manifest
// says "the newest Sodium", the lock says which build that turned out to be,
// what its file is called and what it hashes to.
const LockFileName = "osmium.lock"

// lockVersion is the format's own version, so a future change can be
// recognised rather than misread.
const lockVersion = 1

// Locked is one installed project.
type Locked struct {
	Slug          string `json:"slug"`
	Kind          Kind   `json:"kind"`
	ProjectID     string `json:"project"`
	VersionID     string `json:"version"`
	VersionNumber string `json:"version_number"`
	VersionType   string `json:"channel,omitempty"`
	FileName      string `json:"filename"`
	DownloadURL   string `json:"url"`
	SHA1          string `json:"sha1"`
	Size          int64  `json:"size,omitempty"`

	// Explicit marks a project the user asked for by name. Everything else is
	// here because something else needs it, and may be removed again once
	// nothing does — which is the whole reason the flag exists.
	Explicit bool `json:"explicit"`

	// Pinned marks a project held at this exact build until it is unpinned.
	Pinned bool `json:"pinned,omitempty"`

	// RequiredBy names the projects that depend on this one, so removing a
	// mod can tell what it would leave stranded.
	RequiredBy []string `json:"required_by,omitempty"`
}

// Lock is everything installed in a server, and what it was resolved against.
type Lock struct {
	Version int    `json:"lock_version"`
	Loader  string `json:"loader"`
	Game    string `json:"game_version"`

	// Packages is the resolved set, ordered by kind and slug so that two
	// resolutions of the same manifest produce byte-identical files.
	Packages []Locked `json:"packages"`
}

// NewLock returns an empty lock for a server.
func NewLock(loader, game string) *Lock {
	return &Lock{Version: lockVersion, Loader: loader, Game: game}
}

// Find returns the locked entry for a project, if it has one.
func (l *Lock) Find(kind Kind, slug string) (Locked, bool) {
	if l == nil {
		return Locked{}, false
	}

	for _, entry := range l.Packages {
		if entry.Kind == kind && entry.Slug == slug {
			return entry, true
		}
	}
	return Locked{}, false
}

// Of returns the locked entries of one kind, in order.
func (l *Lock) Of(kind Kind) []Locked {
	if l == nil {
		return nil
	}

	entries := make([]Locked, 0, len(l.Packages))
	for _, entry := range l.Packages {
		if entry.Kind == kind {
			entries = append(entries, entry)
		}
	}
	return entries
}

// Put adds or replaces an entry.
func (l *Lock) Put(entry Locked) {
	for i, existing := range l.Packages {
		if existing.Kind == entry.Kind && existing.Slug == entry.Slug {
			l.Packages[i] = entry
			l.sort()
			return
		}
	}

	l.Packages = append(l.Packages, entry)
	l.sort()
}

// Remove drops an entry, and reports whether there was one.
func (l *Lock) Remove(kind Kind, slug string) bool {
	for i, entry := range l.Packages {
		if entry.Kind == kind && entry.Slug == slug {
			l.Packages = append(l.Packages[:i], l.Packages[i+1:]...)
			return true
		}
	}
	return false
}

// Files is every file the lock accounts for in one kind's folder, which is how
// an untracked jar is recognised: it is one the lock does not name.
func (l *Lock) Files(kind Kind) map[string]Locked {
	files := make(map[string]Locked)
	for _, entry := range l.Of(kind) {
		files[entry.FileName] = entry
	}
	return files
}

// sort keeps the lock in a stable order.
func (l *Lock) sort() {
	sort.Slice(l.Packages, func(a, b int) bool {
		if l.Packages[a].Kind != l.Packages[b].Kind {
			return l.Packages[a].Kind < l.Packages[b].Kind
		}
		return l.Packages[a].Slug < l.Packages[b].Slug
	})
}

// ReadLockIn reads a server's lock. A server that has never had one is not an
// error: it simply has nothing locked yet.
func ReadLockIn(dir string) (*Lock, error) {
	data, err := os.ReadFile(filepath.Join(dir, LockFileName))
	if errors.Is(err, os.ErrNotExist) {
		return &Lock{Version: lockVersion}, nil
	}
	if err != nil {
		return nil, err
	}

	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("%s is not readable: %w", LockFileName, err)
	}

	if lock.Version == 0 {
		lock.Version = lockVersion
	}
	return &lock, nil
}

// WriteLockIn writes a server's lock.
func WriteLockIn(dir string, lock *Lock) error {
	lock.Version = lockVersion
	lock.sort()

	data, err := json.MarshalIndent(lock, "", " ")
	if err != nil {
		return fmt.Errorf("failed to marshal the lock: %w", err)
	}

	path := filepath.Join(dir, LockFileName)
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil { //nolint:gosec // a lock is meant to be readable and committed
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

// SeedLock builds a lock from what an old manifest recorded, for a server that
// has one but has never had a lock. Everything it names is treated as asked
// for by name, because an old manifest kept no record of what was pulled in by
// something else — and treating a dependency as explicit only ever means it is
// kept, never that something is deleted.
func SeedLock(conf *OsmiumConfig) *Lock {
	lock := NewLock(conf.Loader, conf.Version)

	mods, plugins := conf.Legacy()
	for kind, entries := range map[Kind]map[string]LegacyProject{Mods: mods, Plugins: plugins} {
		for slug, project := range entries {
			lock.Put(Locked{
				Slug:          slug,
				Kind:          kind,
				VersionNumber: project.VersionNumber,
				FileName:      project.FileName,
				DownloadURL:   project.DownloadURL,
				SHA1:          project.SHA1,
				Explicit:      true,
			})
		}
	}

	return lock
}
