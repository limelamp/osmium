package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/limelamp/osmium-refactor/tui/internal/tui/config"
)

const ServersFileName = "servers.json"

// Server represents a managed Minecraft server instance.
type Server struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Path      string            `json:"path"`    // Location of the actual Minecraft server files
	Version   string            `json:"version"` // e.g., "1.20.4"
	Type      string            `json:"type"`    // vanilla, paper, fabric, etc.
	Memory    string            `json:"memory"`  // e.g., "2G" or "4G"
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
