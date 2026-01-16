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
	"Forge":    {"1.21.11", "1.21", "1.20.6"},
	"Quilt":    {"1.21.11", "1.21", "1.20.6"},
	// Hybrid
	"Youer": {"1.21.4", "1.21.1"},
}

// Category to server types mapping
var CategoryOptions = map[string][]string{
	"Vanilla/Simple": {"Vanilla"},
	"Plugin-Based":   {"Paper", "Purpur"},
	"Mod Loaders":    {"Fabric", "NeoForge", "Forge", "Quilt"},
	"Hybrid":         {"Youer"},
}

var MOD_LOADERS = []string{"fabric", "forge", "neoforge", "quilt", "liteloader", "modloader", "rift"}

var PLUGIN_LOADERS = []string{"paper", "purpur", "spigot", "bukkit", "folia", "bungeecord", "velocity", "waterfall", "sponge"}
