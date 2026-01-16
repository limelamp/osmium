package constants

// Category to server types mapping
var CategoryOptions = map[string][]string{
	"Vanilla/Simple": {"Vanilla"},
	"Plugin-Based":   {"Paper", "Purpur"},
	"Mod Loaders":    {"Fabric", "NeoForge", "Forge", "Quilt"},
	"Hybrid":         {"Youer"},
}

var MOD_LOADERS = []string{"fabric", "forge", "neoforge", "quilt", "liteloader", "modloader", "rift"}

var PLUGIN_LOADERS = []string{"paper", "purpur", "spigot", "bukkit", "folia", "bungeecord", "velocity", "waterfall", "sponge"}
