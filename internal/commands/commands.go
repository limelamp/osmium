// Package commands is the library of server commands worth keeping.
//
// Anyone running a server ends up typing the same handful of lines over and
// over — a message to everyone nearby, the weather cleared, the time set — and
// some of them are long enough to get wrong. They are kept in one place for
// every server rather than per server, because a command written for one
// server almost always works on the next.
package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/limelamp/osmium/internal/config"
)

// FileName is where the library lives, beside the app's own settings.
const FileName = "commands.json"

// Command is one saved line.
type Command struct {
	Name string `json:"name"`
	Line string `json:"line"`
}

// blankPattern finds the parts of a command to be filled in when it is run.
var blankPattern = regexp.MustCompile(`\{([a-zA-Z][a-zA-Z0-9_ -]*)\}`)

// Blanks are the names a command asks for, in the order they appear.
//
// A saved command with blanks is a small form rather than a fixed string:
// "tell {player} {message}" is worth keeping where "tell Steve hello" is not.
func (c Command) Blanks() []string {
	found := blankPattern.FindAllStringSubmatch(c.Line, -1)

	var names []string
	for _, match := range found {
		name := match[1]

		seen := false
		for _, existing := range names {
			if existing == name {
				seen = true
				break
			}
		}

		if !seen {
			names = append(names, name)
		}
	}

	return names
}

// Fill puts the answers into a command, ready to send.
func (c Command) Fill(answers map[string]string) string {
	return blankPattern.ReplaceAllStringFunc(c.Line, func(match string) string {
		name := strings.Trim(match, "{}")
		if answer, given := answers[name]; given {
			return answer
		}
		return match
	})
}

// Library is the saved commands, in the order they were added.
type Library struct {
	Commands []Command `json:"commands"`
}

// Load reads the library, which is empty on a first run rather than an error.
func Load() Library {
	path, err := libraryPath()
	if err != nil {
		return starter()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return starter()
	}

	var library Library
	if err := json.Unmarshal(data, &library); err != nil {
		return starter()
	}

	return library
}

// Save writes the library back.
func (l Library) Save() error {
	path, err := libraryPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(l, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// Add puts a command in the library, replacing one of the same name.
func (l Library) Add(command Command) Library {
	for i, existing := range l.Commands {
		if strings.EqualFold(existing.Name, command.Name) {
			l.Commands[i] = command
			return l
		}
	}

	l.Commands = append(l.Commands, command)
	return l
}

// Remove takes one out.
func (l Library) Remove(name string) Library {
	kept := make([]Command, 0, len(l.Commands))
	for _, existing := range l.Commands {
		if !strings.EqualFold(existing.Name, name) {
			kept = append(kept, existing)
		}
	}

	l.Commands = kept
	return l
}

// Find returns the command with a name, if there is one.
func (l Library) Find(name string) (Command, bool) {
	for _, existing := range l.Commands {
		if strings.EqualFold(existing.Name, name) {
			return existing, true
		}
	}
	return Command{}, false
}

// Matching are the commands whose name begins with a prefix, which is what the
// console's command line completes against.
func (l Library) Matching(prefix string) []Command {
	prefix = strings.ToLower(strings.TrimSpace(prefix))

	var found []Command
	for _, existing := range l.Commands {
		if prefix == "" || strings.HasPrefix(strings.ToLower(existing.Name), prefix) {
			found = append(found, existing)
		}
	}
	return found
}

// starter is what a new library holds: a few commands that show what one is
// for, including one with a blank in it.
func starter() Library {
	return Library{Commands: []Command{
		{Name: "day", Line: "time set day"},
		{Name: "clear", Line: "weather clear"},
		{Name: "nearby", Line: "tell @a[distance=..10] {message}"},
		{Name: "whisper", Line: "tell {player} {message}"},
		{Name: "save", Line: "save-all"},
	}}
}

// libraryPath is where the file lives.
func libraryPath() (string, error) {
	dir, err := config.EnsureAppDirExists()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}
