package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/limelamp/osmium/internal/config"
)

const ServersFileName = "servers.json"

// Server represents a managed Minecraft server instance.
type Server struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`    // Location of the actual Minecraft server files
	Version string `json:"version"` // e.g., "1.20.4"
	Type    string `json:"type"`    // vanilla, paper, fabric, etc.
	Memory  string `json:"memory"`  // e.g., "2G" or "4G"
}

// Exists reports whether the server's folder is still on disk. An entry whose
// folder has been deleted outside Osmium is stale and is dropped from the list
// rather than shown as something that could be started.
func (s Server) Exists() bool {
	if s.Path == "" {
		return false
	}
	info, err := os.Stat(s.Path)
	return err == nil && info.IsDir()
}

// Present splits a server list into the entries worth keeping and the names of
// those that were dropped.
//
// An entry is dropped when its folder is gone, and when an earlier entry
// already points at the same folder: a server is its folder, so two rows for
// one folder are one row and a leftover from before registration was keyed on
// the path.
func Present(servers []Server) (present []Server, missing []string) {
	present = make([]Server, 0, len(servers))
	seen := make(map[string]bool, len(servers))

	for _, server := range servers {
		switch {
		case !server.Exists():
			missing = append(missing, server.Name)

		case seen[server.Path]:
			missing = append(missing, server.Name+" (duplicate entry)")

		default:
			seen[server.Path] = true
			present = append(present, server)
		}
	}
	return present, missing
}

// FromManifest builds a record for a folder that holds an osmium.json but is
// not in the list — a server created outside Osmium, or one rebuilt by hand.
// The folder names it; the manifest describes it.
func FromManifest(dir string) (Server, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return Server{}, err
	}

	conf, err := config.ReadConfigIn(absolute)
	if err != nil {
		return Server{}, err
	}

	name := filepath.Base(absolute)
	return Server{
		ID:      name,
		Name:    name,
		Path:    absolute,
		Version: conf.Version,
		Type:    strings.ToLower(conf.Loader),
		Memory:  conf.AllocatedRAM,
	}, nil
}

// Register records a server in servers.json.
//
// A server is identified by its folder, so registering one that is already
// listed updates that entry rather than adding a second row for the same
// files. Only a genuinely new folder is appended, with its id made unique
// against the ones already in use.
func Register(store *ServerStore, server *Server) error {
	servers, err := store.LoadAll()
	if err != nil {
		return err
	}

	taken := make(map[string]bool, len(servers))
	for i, existing := range servers {
		if existing.Path == server.Path {
			server.ID = existing.ID
			servers[i] = *server
			return store.SaveAll(servers)
		}
		taken[existing.ID] = true
	}

	base := server.ID
	for i := 2; taken[server.ID]; i++ {
		server.ID = fmt.Sprintf("%s-%d", base, i)
	}

	return store.SaveAll(append(servers, *server))
}

// Sync refreshes each entry from its server's own osmium.json and reports
// whether anything changed.
//
// servers.json keeps a copy of the engine, version and memory so the list can
// be drawn without opening every server folder, and that copy goes stale the
// moment something else edits the manifest — a migration, the CLI, or a hand
// edit. The manifest is the truth, so the copy is refreshed from it whenever
// the list is read rather than only where Osmium happens to remember to.
func Sync(servers []Server) ([]Server, bool) {
	changed := false

	for i, server := range servers {
		conf, err := config.ReadConfigIn(server.Path)
		if err != nil {
			continue // a server with no manifest keeps what it was registered with
		}

		if loader := strings.ToLower(conf.Loader); loader != "" && loader != server.Type {
			servers[i].Type, changed = loader, true
		}
		if conf.Version != "" && conf.Version != server.Version {
			servers[i].Version, changed = conf.Version, true
		}
		if conf.AllocatedRAM != "" && conf.AllocatedRAM != server.Memory {
			servers[i].Memory, changed = conf.AllocatedRAM, true
		}
	}

	return servers, changed
}

// ServerStore manages thread-safe JSON interactions for server data.
type ServerStore struct {
	mu sync.RWMutex
}

// NewServerStore initializes a storage handler.
func NewServerStore() *ServerStore {
	return &ServerStore{}
}

// GetFilePath retrieves the absolute path to servers.json.
func (s *ServerStore) GetFilePath() (string, error) {
	appDir, err := config.EnsureAppDirExists()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, ServersFileName), nil
}

// Load reads the registry and brings it up to date: entries whose folder is
// gone are dropped, the rest are refreshed from their own osmium.json, and the
// file is rewritten only when something actually changed. It returns the names
// of whatever was dropped so the caller can say what happened.
//
// Every reader goes through this — the interface and the commands alike — so
// none of them can show a server as something it stopped being.
func (s *ServerStore) Load() ([]Server, []string, error) {
	servers, err := s.LoadAll()
	if err != nil {
		return nil, nil, err
	}

	present, missing := Present(servers)
	present, changed := Sync(present)

	if len(missing) == 0 && !changed {
		return present, nil, nil
	}
	return present, missing, s.SaveAll(present)
}

// LoadAll reads and decodes the servers list.
func (s *ServerStore) LoadAll() ([]Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath, err := s.GetFilePath()
	if err != nil {
		return nil, err
	}

	// If the servers.json file doesn't exist, return an empty list instead of an error.
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []Server{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Handle case where the file is entirely empty
	if len(data) == 0 {
		return []Server{}, nil
	}

	var servers []Server
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, err
	}

	return servers, nil
}

// SaveAll serializes and saves the complete servers list.
func (s *ServerStore) SaveAll(servers []Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, err := s.GetFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}
