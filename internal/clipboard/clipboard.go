// Package clipboard puts text on the system clipboard.
//
// Two routes are used together. The first is OSC 52, an escape sequence the
// terminal itself acts on: it needs nothing installed and it works over SSH,
// where no local tool could reach the clipboard you are actually looking at.
// Some terminals refuse it, so the second route is whichever clipboard command
// the platform has — and since neither can be relied on alone, both are tried.
package clipboard

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// ErrNoTool says the machine has no clipboard command, so the copy rests on
// whether the terminal honoured OSC 52.
var ErrNoTool = errors.New("no clipboard tool found")

// tool is one way of putting text on the clipboard.
type tool struct {
	name string
	args []string
}

// tools are the commands each platform might have, in the order they are tried.
func tools() []tool {
	switch runtime.GOOS {
	case "darwin":
		return []tool{{name: "pbcopy"}}

	case "windows":
		// clip.exe has shipped with Windows since Vista.
		return []tool{{name: "clip.exe"}, {name: "clip"}}

	default:
		return []tool{
			{name: "wl-copy"}, // Wayland
			{name: "xclip", args: []string{"-selection", "clipboard"}}, // X11
			{name: "xsel", args: []string{"--clipboard", "--input"}},   // X11, the other one
		}
	}
}

// Write hands text to the platform's clipboard command, and reports which one
// took it. ErrNoTool means there was none to try.
func Write(text string) (string, error) {
	for _, candidate := range tools() {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}

		cmd := exec.Command(path, candidate.args...)
		cmd.Stdin = strings.NewReader(text)

		if err := cmd.Run(); err != nil {
			return candidate.name, err
		}
		return candidate.name, nil
	}

	return "", ErrNoTool
}
