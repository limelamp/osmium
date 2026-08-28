package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/limelamp/osmium/internal/config"
)

// ManifestSuffix is what a kept manifest is named after the server it came
// from, e.g. "survival.osmium.json".
const ManifestSuffix = ".osmium.json"

// Destroy deletes a server: its folder and everything in it, and its entry in
// servers.json.
//
// keepManifest saves the server's osmium.json beside the folder before the
// folder goes, under the server's own name. It is the one part of a server that
// is worth anything once the world is gone — the loader, version, memory and
// the whole mod list — so keeping it means the same server can be built again.
// The path it was saved to is returned, empty when nothing was kept.
func Destroy(store *ServerStore, server Server, keepManifest bool) (kept string, err error) {
	if server.Path == "" {
		return "", fmt.Errorf("%s has no folder recorded", server.Name)
	}

	if keepManifest {
		kept, err = saveManifest(server)
		if err != nil {
			return "", err
		}
	}

	if err := os.RemoveAll(server.Path); err != nil {
		return kept, err
	}

	return kept, Unregister(store, server.Path)
}

// saveManifest copies a server's manifest out of the folder about to be
// deleted, and returns where it went.
func saveManifest(server Server) (string, error) {
	source := filepath.Join(server.Path, config.OsmiumFileName)

	data, err := os.ReadFile(source)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // nothing to keep is not a failure
		}
		return "", err
	}

	absolute, err := filepath.Abs(server.Path)
	if err != nil {
		return "", err
	}

	target := filepath.Join(filepath.Dir(absolute), filepath.Base(absolute)+ManifestSuffix)
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", err
	}
	return target, nil
}

// Unregister drops whatever entry points at a folder, leaving the folder alone.
func Unregister(store *ServerStore, path string) error {
	servers, err := store.LoadAll()
	if err != nil {
		return err
	}

	remaining := make([]Server, 0, len(servers))
	for _, server := range servers {
		if server.Path != path {
			remaining = append(remaining, server)
		}
	}

	if len(remaining) == len(servers) {
		return nil
	}
	return store.SaveAll(remaining)
}
