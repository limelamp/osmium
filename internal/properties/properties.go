// Package properties reads and writes a Minecraft server's server.properties.
//
// The file is a server's own settings, written by the server and edited by
// hand for years by everyone who has run one. So it is read as it is and
// written back as it was: every comment, every blank line and every key stays
// where it was, and only the values that were changed change. A settings
// editor that reformats somebody's file is one they stop using.
package properties

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileName is what the file is called, in every version of the server.
const FileName = "server.properties"

// Properties is one server's settings.
type Properties struct {
	path string

	// lines is the file as it was read, and index says which line holds each
	// key. Writing walks the lines, not the map, which is what keeps the file
	// looking like the one that was opened.
	lines []string
	index map[string]int
}

// Read loads a server's settings.
func Read(dir string) (*Properties, error) {
	path := filepath.Join(dir, FileName)

	file, err := os.Open(path) //nolint:gosec // the path is a server directory the user chose
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	props := &Properties{path: path, index: map[string]int{}}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	for scanner.Scan() {
		line := scanner.Text()
		props.lines = append(props.lines, line)

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if key, _, found := strings.Cut(trimmed, "="); found {
			props.index[strings.TrimSpace(key)] = len(props.lines) - 1
		}
	}

	return props, scanner.Err()
}

// Get is one setting, or "" when the file does not mention it.
func (p *Properties) Get(key string) string {
	if p == nil {
		return ""
	}

	line, known := p.index[key]
	if !known {
		return ""
	}

	_, value, _ := strings.Cut(p.lines[line], "=")
	return strings.TrimSpace(value)
}

// Int is one setting as a number.
func (p *Properties) Int(key string) int {
	value, err := strconv.Atoi(p.Get(key))
	if err != nil {
		return 0
	}
	return value
}

// Bool is one setting as a flag.
func (p *Properties) Bool(key string) bool { return p.Get(key) == "true" }

// Set changes a setting, adding it to the end when the file has no such key.
func (p *Properties) Set(key, value string) {
	line, known := p.index[key]
	if known {
		p.lines[line] = key + "=" + value
		return
	}

	p.lines = append(p.lines, key+"="+value)
	p.index[key] = len(p.lines) - 1
}

// Save writes the file back.
func (p *Properties) Save() error {
	body := strings.Join(p.lines, "\n")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	return os.WriteFile(p.path, []byte(body), 0o644) //nolint:gosec // the server reads this file itself
}

// Keys of the settings Osmium shows and edits by name.
const (
	KeyPort       = "server-port"
	KeyMOTD       = "motd"
	KeyDifficulty = "difficulty"
	KeyGamemode   = "gamemode"
	KeyMaxPlayers = "max-players"
	KeyWhitelist  = "white-list"
	KeyPVP        = "pvp"
	KeyOnlineMode = "online-mode"
	KeyHardcore   = "hardcore"
	KeyLevelName  = "level-name"
	KeySeed       = "level-seed"
	KeyViewRange  = "view-distance"
	KeyAddress    = "server-ip"
)

// Port is the port the server listens on, or the default when it says nothing.
func (p *Properties) Port() int {
	if port := p.Int(KeyPort); port > 0 {
		return port
	}
	if p == nil || len(p.lines) == 0 {
		return 0
	}
	return 25565
}

// Setting is one editable value, with what it may be set to.
type Setting struct {
	Key     string
	Label   string
	Choices []string // empty means it is typed rather than chosen
}

// Gameplay is the set of settings worth editing from Osmium: what a player
// meets when they join, rather than the whole file.
var Gameplay = []Setting{
	{Key: KeyMOTD, Label: "Message of the day"},
	{Key: KeyDifficulty, Label: "Difficulty", Choices: []string{"peaceful", "easy", "normal", "hard"}},
	{Key: KeyGamemode, Label: "Game mode", Choices: []string{"survival", "creative", "adventure", "spectator"}},
	{Key: KeyMaxPlayers, Label: "Maximum players"},
	{Key: KeyWhitelist, Label: "Whitelist", Choices: []string{"true", "false"}},
	{Key: KeyPVP, Label: "Player versus player", Choices: []string{"true", "false"}},
	{Key: KeyOnlineMode, Label: "Require Minecraft accounts", Choices: []string{"true", "false"}},
	{Key: KeyViewRange, Label: "View distance"},
	{Key: KeyPort, Label: "Port"},
}

// Summary is the handful of values worth showing about a server, in order.
func (p *Properties) Summary() [][2]string {
	if p == nil {
		return nil
	}

	rows := [][2]string{
		{"world", p.Get(KeyLevelName)},
		{"difficulty", p.Get(KeyDifficulty)},
		{"game mode", p.Get(KeyGamemode)},
		{"max players", p.Get(KeyMaxPlayers)},
		{"pvp", p.Get(KeyPVP)},
		{"whitelist", p.Get(KeyWhitelist)},
		{"online mode", p.Get(KeyOnlineMode)},
		{"view distance", p.Get(KeyViewRange)},
		{"port", fmt.Sprint(p.Port())},
	}

	if seed := p.Get(KeySeed); seed != "" {
		rows = append(rows, [2]string{"seed", seed})
	}

	kept := rows[:0]
	for _, row := range rows {
		if row[1] != "" {
			kept = append(kept, row)
		}
	}
	return kept
}
