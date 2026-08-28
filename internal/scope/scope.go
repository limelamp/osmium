package scope

import (
	"fmt"
	"path/filepath"
)

// Scope tells a project operation which server directory it runs against and
// where its progress output goes.
//
// The mod/plugin helpers used to assume the process working directory and print
// straight to stdout, which only worked for the CLI. Threading a Scope through
// them lets the TUI point the same code at whichever server the user selected
// and capture the progress lines into a log panel instead of corrupting the
// terminal.
type Scope struct {
	// Dir is the server directory every relative path is resolved against.
	Dir string

	// Log receives progress lines. A nil Log discards them.
	Log func(string)
}

// For returns a scope for dir that prints progress to stdout, which is
// what the Cobra commands want.
func For(dir string) Scope {
	return Scope{Dir: dir, Log: func(line string) { fmt.Println(line) }}
}

// Path resolves elements relative to the scope's server directory.
func (s Scope) Path(elem ...string) string {
	return filepath.Join(append([]string{s.dir()}, elem...)...)
}

// Logf reports one line of progress.
func (s Scope) Logf(format string, a ...any) {
	if s.Log == nil {
		return
	}
	s.Log(fmt.Sprintf(format, a...))
}

func (s Scope) dir() string {
	if s.Dir == "" {
		return "."
	}
	return s.Dir
}
