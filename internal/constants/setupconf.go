package constants

// Version maps for different server types
var ServerVersions = map[string][]string{
	// Vanilla/Simple
	"Vanilla": {"1.21.11", "1.21", "1.20.6", "1.20.4"},
	// Plugin-Based
	"Paper":  {"1.21.11", "1.21", "1.20.6", "1.20.4"},
	"Purpur": {"1.21.11", "1.21", "1.20.6", "1.20.4"},
	// Mod Loaders
	"Fabric":   {"1.21.11", "1.21", "1.20.6"},
	"NeoForge": {"1.21.11", "1.21.4", "1.21.1"},
	// Hybrid
	"Youer": {"1.21.4", "1.21.1"},
}

// Category to server types mapping
var CategoryOptions = map[string][]string{
	"Vanilla/Simple": {"Vanilla"},
	"Plugin-Based":   {"Paper", "Purpur"},
	"Mod Loaders":    {"Fabric", "NeoForge"},
	"Hybrid":         {"Youer"},
}
