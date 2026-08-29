package shared

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Player is everything a server records about someone who has played on it, or
// has been told not to.
//
// It is assembled from the files the server keeps beside its jar — ops.json,
// banned-players.json, whitelist.json and usercache.json — so it can be read
// whether the server is running or not. Only Online needs the server itself.
type Player struct {
	Name string
	UUID string

	Op    bool
	Level int

	Banned bool
	Reason string

	Whitelisted bool
	Online      bool

	LastSeen time.Time
}

// The files a server keeps its player records in.
const (
	opsFile       = "ops.json"
	bannedFile    = "banned-players.json"
	whitelistFile = "whitelist.json"
	cacheFile     = "usercache.json"
)

// Players reads what the server folder records about players, newest activity
// first among those it has actually seen.
func Players(dir string) ([]Player, error) {
	players := map[string]*Player{}

	// The user cache is the roll of everyone the server has seen.
	var cached []struct {
		Name      string `json:"name"`
		UUID      string `json:"uuid"`
		ExpiresOn string `json:"expiresOn"`
	}
	if err := readPlayerFile(dir, cacheFile, &cached); err != nil {
		return nil, err
	}
	for _, entry := range cached {
		player := playerIn(players, entry.Name, entry.UUID)
		player.LastSeen = parseCacheTime(entry.ExpiresOn)
	}

	var ops []struct {
		Name  string `json:"name"`
		UUID  string `json:"uuid"`
		Level int    `json:"level"`
	}
	if err := readPlayerFile(dir, opsFile, &ops); err != nil {
		return nil, err
	}
	for _, entry := range ops {
		player := playerIn(players, entry.Name, entry.UUID)
		player.Op, player.Level = true, entry.Level
	}

	var banned []struct {
		Name   string `json:"name"`
		UUID   string `json:"uuid"`
		Reason string `json:"reason"`
	}
	if err := readPlayerFile(dir, bannedFile, &banned); err != nil {
		return nil, err
	}
	for _, entry := range banned {
		player := playerIn(players, entry.Name, entry.UUID)
		player.Banned, player.Reason = true, entry.Reason
	}

	var whitelisted []struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
	}
	if err := readPlayerFile(dir, whitelistFile, &whitelisted); err != nil {
		return nil, err
	}
	for _, entry := range whitelisted {
		playerIn(players, entry.Name, entry.UUID).Whitelisted = true
	}

	list := make([]Player, 0, len(players))
	for _, player := range players {
		list = append(list, *player)
	}

	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
	return list, nil
}

// MarkOnline flags the players a server says are connected right now.
func MarkOnline(players []Player, online []string) []Player {
	connected := make(map[string]bool, len(online))
	for _, name := range online {
		connected[strings.ToLower(name)] = true
	}

	for i := range players {
		players[i].Online = connected[strings.ToLower(players[i].Name)]
	}

	// Somebody connected for the first time is online without being in any of
	// the files yet.
	for _, name := range online {
		if !known(players, name) {
			players = append(players, Player{Name: name, Online: true})
		}
	}
	return players
}

// known reports whether a name is already in the list.
func known(players []Player, name string) bool {
	for _, player := range players {
		if strings.EqualFold(player.Name, name) {
			return true
		}
	}
	return false
}

// playerIn returns the entry for a name, creating it on first mention.
func playerIn(players map[string]*Player, name, uuid string) *Player {
	key := strings.ToLower(name)
	if existing, ok := players[key]; ok {
		return existing
	}

	created := &Player{Name: name, UUID: uuid}
	players[key] = created
	return created
}

// readPlayerFile decodes one of the server's player files. A file that is not
// there is not an error: a server that has never run has none of them.
func readPlayerFile(dir, name string, into any) error {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	return json.Unmarshal(data, into)
}

// parseCacheTime reads the timestamp the user cache uses, which is neither RFC
// 3339 nor quite anything else.
func parseCacheTime(value string) time.Time {
	parsed, err := time.Parse("2006-01-02 15:04:05 -0700", value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// listReply matches what a server answers the "list" command with, which is the
// only way to learn who is connected.
var listReply = regexp.MustCompile(`There are (\d+) of a max of (\d+) players online:?\s*(.*)$`)

// OnlineFromLog finds the newest answer to "list" in a server's log and reads
// the players out of it.
func OnlineFromLog(log string) (names []string, online, max int, ok bool) {
	scanner := bufio.NewScanner(strings.NewReader(log))

	for scanner.Scan() {
		match := listReply.FindStringSubmatch(scanner.Text())
		if match == nil {
			continue
		}

		online, _ = strconv.Atoi(match[1])
		max, _ = strconv.Atoi(match[2])

		names = nil
		for _, name := range strings.Split(match[3], ",") {
			if name = strings.TrimSpace(name); name != "" {
				names = append(names, name)
			}
		}
		ok = true
	}

	return names, online, max, ok
}

// MaxPlayers reads the player limit out of server.properties, or zero when it
// says nothing.
func MaxPlayers(dir string) int {
	file, err := os.Open(filepath.Join(dir, "server.properties"))
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if found && strings.TrimSpace(key) == "max-players" {
			count, _ := strconv.Atoi(strings.TrimSpace(value))
			return count
		}
	}
	return 0
}

// FolderSize is how much disk a server takes, which is mostly its world.
func FolderSize(dir string) int64 {
	var total int64

	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}
