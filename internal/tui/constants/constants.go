// Package constants holds the fixed lists Osmium offers: the server software
// it can install, and the loaders each kind of project accepts.
package constants

// CategoryOptions maps each category to the server software in it.
var CategoryOptions = map[string][]string{
	"Vanilla/Simple": {"Vanilla"},
	"Plugin-Based":   {"Paper", "Purpur"},
	"Mod Loaders":    {"Fabric", "NeoForge", "Forge", "Quilt"},
	"Hybrid":         {"Youer"},
}

// MOD_LOADERS are the loader names Modrinth uses for mods.
//
//nolint:revive,staticcheck // the names match Modrinth's own vocabulary
var MOD_LOADERS = []string{"fabric", "forge", "neoforge", "quilt", "liteloader", "modloader", "rift"}

// PLUGIN_LOADERS are the loader names Modrinth uses for plugins.
//
//nolint:revive,staticcheck // the names match Modrinth's own vocabulary
var PLUGIN_LOADERS = []string{"paper", "purpur", "spigot", "bukkit", "folia", "bungeecord", "velocity", "waterfall", "sponge"}

// PLUGIN_RESOLVER maps a plugin server to the APIs its plugins may target, in
// order of preference.
//
//nolint:revive,staticcheck // the names match Modrinth's own vocabulary
var PLUGIN_RESOLVER = map[string][]string{
	"purpur": {"purpur", "paper", "spigot", "bukkit"},
	"paper":  {"paper", "spigot", "bukkit"},
	"spigot": {"spigot", "bukkit"},
	"bukkit": {"bukkit"},
}
