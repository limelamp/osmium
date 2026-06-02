package constants

var Categories = []string{"Vanilla", "Plugin", "Mod Loader"}
var PluginOptions = []string{"Paper", "Purpur", "Spigot", "Bukkit"}
var ModLoaderOptions = []string{"Forge", "NeoForge", "Fabric", "Quilt"}

var SoftwareDesc = map[string]string{
	"Vanilla":  "The official, unmodified Minecraft server software.",
	"Fabric":   "A lightweight, modular modding toolchain for modern versions.",
	"Forge":    "The classic, heavy-duty modding platform for legacy and custom packs.",
	"NeoForge": "A cleaned-up, modern successor designed for next-generation modding.",
	"Quilt":    "An open, community-driven ecosystem compatible with Fabric mods.",
	"Paper":    "High-performance server built for plugins and public play.",
	"Purpur":   "A Paper drop-in replacement designed for ultimate customizability.",
	"Spigot":   "The classic, modified high-performance plugin server API.",
	"Bukkit":   "The original and foundational plugin standard wrapper.",
}

func GetVersions(software string) []string {
	switch software {
	case "Vanilla":
		return []string{"1.21", "1.20.6", "1.20.4", "1.19.4"}
	case "Fabric":
		return []string{"1.21 (Fabric)", "1.20.4 (Fabric)", "1.20.1 (Fabric)"}
	case "Forge":
		return []string{"1.20.1 (Forge)", "1.19.2 (Forge)", "1.12.2 (Forge)"}
	case "NeoForge":
		return []string{"1.21 (NeoForge)", "1.20.4 (NeoForge)", "1.20.1 (NeoForge)"}
	case "Quilt":
		return []string{"1.21 (Quilt)", "1.20.4 (Quilt)", "1.20.1 (Quilt)"}
	case "Paper":
		return []string{"1.21 (Paper)", "1.20.4 (Paper)", "1.19.4 (Paper)"}
	case "Purpur":
		return []string{"1.21 (Purpur)", "1.20.4 (Purpur)", "1.19.4 (Purpur)"}
	case "Spigot":
		return []string{"1.21 (Spigot)", "1.20.4 (Spigot)", "1.19.4 (Spigot)"}
	case "Bukkit":
		return []string{"1.21 (Bukkit)", "1.20.4 (Bukkit)", "1.19.4 (Bukkit)"}
	default:
		return []string{"1.21", "1.20.4", "1.19.4"}
	}
}

var RamOptions = []string{"2 GB", "4 GB", "6 GB", "8 GB", "12 GB", "16 GB"}
