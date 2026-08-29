package clipboard

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The text has to reach the tool's standard input, which is the whole of what
// this package does. A stand-in on PATH records what it was given, so the test
// never touches the clipboard of whoever is running it.
func TestWriteFeedsTheTool(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}

	dir := t.TempDir()
	record := filepath.Join(dir, "copied.txt")

	// tools() tries wl-copy first on anything but macOS and Windows.
	name := "wl-copy"
	if runtime.GOOS == "darwin" {
		name = "pbcopy"
	}

	// PATH is about to hold nothing but the stand-in, so the script calls cat
	// by its full path.
	script := "#!/bin/sh\n/bin/cat > " + record + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)

	tool, err := Write("server │ [23:04] Steve joined the game")
	if err != nil {
		t.Fatalf("copying failed: %v", err)
	}
	if tool != name {
		t.Errorf("copied with %q, want %q", tool, name)
	}

	written, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the tool was not run: %v", err)
	}
	if got := strings.TrimSpace(string(written)); got != "server │ [23:04] Steve joined the game" {
		t.Errorf("the tool was given %q", got)
	}
}

// With no tool at all, the caller is told so rather than left thinking the copy
// worked — the terminal may still have taken the OSC 52 sequence, which is why
// the app words it the way it does.
func TestWriteWithoutAnyTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if _, err := Write("anything"); !errors.Is(err, ErrNoTool) {
		t.Errorf("with no tool installed the error was %v, want ErrNoTool", err)
	}
}
