package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const ConfigFileName = "config.json"

// ServersFolderName is the sub-directory of the app directory that holds
// servers created with the "Default folder" option.
const ServersFolderName = "servers"

// AppConfig stores global application settings.
type AppConfig struct {
	Theme string `json:"theme"`

	// DefaultServerDir is where the Create Server wizard puts a server when the
	// user picks "Default folder". Empty means "use the app's servers folder",
	// which keeps existing config files working without a migration.
	DefaultServerDir string `json:"default_server_dir"`

	// DefaultRAM preselects the memory step of the Create Server wizard.
	DefaultRAM string `json:"default_ram"`
}

// DefaultConfig returns the fallback configuration.
func DefaultConfig() AppConfig {
	return AppConfig{
		Theme:      "Teal", // Default theme matching your current colors
		DefaultRAM: "4G",
	}
}

// ServersDir resolves the configured default server directory, falling back to
// the app directory's servers folder when nothing has been configured.
func (c AppConfig) ServersDir() string {
	if c.DefaultServerDir != "" {
		return c.DefaultServerDir
	}

	appDir, err := GetAppDir()
	if err != nil {
		return ServersFolderName
	}
	return filepath.Join(appDir, ServersFolderName)
}

// LoadConfig reads the config.json file or returns defaults if it doesn't exist.
func LoadConfig() (AppConfig, error) {
	appDir, err := EnsureAppDirExists()
	if err != nil {
		return DefaultConfig(), err
	}
	filePath := filepath.Join(appDir, ConfigFileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}
	return cfg, nil
}

// SaveConfig writes current configuration settings back to the config directory.
func SaveConfig(cfg AppConfig) error {
	appDir, err := EnsureAppDirExists()
	if err != nil {
		return err
	}
	filePath := filepath.Join(appDir, ConfigFileName)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}
