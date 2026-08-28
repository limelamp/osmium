package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// OsmiumFileName is the per-server manifest Osmium writes next to the jar.
const OsmiumFileName = "osmium.json"

type Project struct {
	VersionNumber string `json:"version_number"`
	FileName      string `json:"filename"` // files[0].filename
	DownloadURL   string `json:"url"`      // files[0].download_url
	SHA1          string `json:"sha1"`
}

type OsmiumConfig struct {
	Category     string             `json:"category"`
	Loader       string             `json:"loader"`
	Version      string             `json:"version"`
	AllocatedRAM string             `json:"ram"`
	Mods         map[string]Project `json:"mods"`
	Plugins      map[string]Project `json:"plugins"`
}

// WriteConfigIn writes the manifest into the given server directory.
func WriteConfigIn(dir string, config *OsmiumConfig) error {
	bytes, err := json.MarshalIndent(config, "", " ")
	if err != nil {
		return fmt.Errorf("failed to marshal Osmium config: %w", err)
	}

	path := filepath.Join(dir, OsmiumFileName)
	if err := os.WriteFile(path, bytes, 0644); err != nil {
		return fmt.Errorf("failed to write Osmium config to %s: %w", path, err)
	}

	return nil
}

// ReadConfigIn reads the manifest from the given server directory.
func ReadConfigIn(dir string) (*OsmiumConfig, error) {
	// Read entire file
	data, err := os.ReadFile(filepath.Join(dir, OsmiumFileName))
	if err != nil {
		return nil, err
	}

	// Prepare a variable to hold the parsed data
	var config OsmiumConfig

	// Unmarshal (parse) the JSON into the struct
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
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
