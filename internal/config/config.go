package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const ConfigFileName = "config.json"

// AppConfig stores global application settings.
type AppConfig struct {
	Theme string `json:"theme"`
}

// DefaultConfig returns the fallback configuration.
func DefaultConfig() AppConfig {
	return AppConfig{
		Theme: "Teal", // Default theme matching your current colors
	}
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

	var cfg AppConfig
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
