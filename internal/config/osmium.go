package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type OsmiumConfig struct {
	Category string `json:"category"`
	Loader   string `json:"loader"`
	Version  string `json:"version"`
}

func WriteConfig(config *OsmiumConfig) error {
	bytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal Osmium config: %w", err)
	}

	if err := os.WriteFile("osmium.json", bytes, 0644); err != nil {
		return fmt.Errorf("failed to write Osmium config to osmium.json: %w", err)
	}

	return nil
}

func ReadConfig() (*OsmiumConfig, error) {
	// Read entire file
	data, err := os.ReadFile("osmium.json")
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
