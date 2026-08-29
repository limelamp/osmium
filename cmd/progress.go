package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
	"golang.org/x/term"
)

// reporter draws what a long command is doing: its output as it arrives, and
// underneath it one live line - a spinner while it waits, a progress bar while
// something downloads.
//
// The live line is only drawn on a terminal. Piped into a file or a CI log it
// would be a stream of half-erased bars, so there it prints nothing and the
// ordinary output stands on its own.
type reporter struct {
	mu sync.Mutex

	tty   bool
	frame int

	label string
	done  int64
	total int64
	shown bool

	// finished is the download whose bar has already been committed, so the
	// same one is not printed twice.
	finished string

	stop chan struct{}
	once sync.Once
}

// liveInterval is how often the live line is redrawn.
const liveInterval = 100 * time.Millisecond

// newReporter starts a reporter for the current command.
func newReporter() *reporter {
	r := &reporter{tty: isTerminal(), stop: make(chan struct{})}

	if r.tty {
		go r.animate()
	}
	return r
}

// Scope returns the scope to hand to an operation, pointed at a directory.
func (r *reporter) Scope(dir string) scope.Scope {
	return scope.Scope{
		Dir:      dir,
		Log:      r.Log,
		Progress: r.Progress,
	}
}

// Log prints one line of output above the live line.
func (r *reporter) Log(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.clear()
	fmt.Println(line)
}

// Progress records how far a download has got; the live line shows it.
func (r *reporter) Progress(label string, done, total int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.label, r.done, r.total = label, done, total

	// A finished download leaves nothing to animate, but its bar stays on
	// screen: on the command line it is the only record of what was fetched
	// and how big it was.
	//
	// A download usually says it is finished more than once — the last chunk
	// counted, and then the total reported again by the code that fetched it —
	// so the bar is committed once for each file rather than once for each
	// report that it is over.
	if total > 0 && done >= total {
		if r.finished != label {
			r.finished = label
			r.commit()
		}

		r.label, r.done, r.total = "", 0, 0
		return
	}

	r.finished = ""
}

// Close removes the live line and stops animating it.
func (r *reporter) Close() {
	r.once.Do(func() {
		close(r.stop)

		r.mu.Lock()
		defer r.mu.Unlock()
		r.clear()
	})
}

// animate redraws the live line until the command is done.
func (r *reporter) animate() {
	ticker := time.NewTicker(liveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.mu.Lock()
			r.frame++
			r.draw()
			r.mu.Unlock()
		}
	}
}

// draw renders the live line in place. The caller holds the mutex.
//
// It has to fit on one row: a line wider than the terminal wraps, and then the
// carriage return that should have overwritten it only returns to the start of
// its last piece, leaving a trail of half-drawn bars behind.
func (r *reporter) draw() {
	if !r.tty || r.label == "" {
		return
	}

	width := terminalWidth()
	line := styles.Spinner(r.frame) + " " + trimmed(r.label, width/3)

	if r.total > 0 {
		line += "  " + styles.ProgressBar(width-lipgloss.Width(line)-2, r.done, r.total)
	}

	fmt.Print("\r\033[K" + lipgloss.NewStyle().MaxWidth(width-1).Render(line))
	r.shown = true
}

// commit leaves the live line on screen as a finished one, with a mark in
// place of the spinner, and moves off it so the next output starts on a clean
// row. The caller holds the mutex.
func (r *reporter) commit() {
	if !r.tty || r.label == "" || r.total <= 0 {
		return
	}

	width := terminalWidth()

	line := lipgloss.NewStyle().Foreground(theme.Accent).Render("✓") + " " +
		trimmed(r.label, width/3)
	line += "  " + styles.ProgressBar(width-lipgloss.Width(line)-2, r.done, r.total)

	fmt.Println("\r\033[K" + lipgloss.NewStyle().MaxWidth(width-1).Render(line))
	r.shown = false
}

// terminalWidth is how wide the terminal is, falling back to a conservative
// width when it cannot be asked.
func terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	return width
}

// clear wipes the live line so ordinary output lands on a clean row. The caller
// holds the mutex.
func (r *reporter) clear() {
	if !r.tty || !r.shown {
		return
	}

	fmt.Print("\r\033[K")
	r.shown = false
}

// isTerminal reports whether standard output is a terminal rather than a pipe
// or a file.
func isTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// busy shows a spinner with a label for work that reports nothing until it is
// finished - resolving a version, asking Modrinth. The returned function ends
// it.
func (r *reporter) busy(label string) func() {
	r.mu.Lock()
	r.label, r.done, r.total = label, 0, 0
	r.mu.Unlock()

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		r.label = ""
		r.clear()
	}
}

// trimmed keeps a label short enough to leave room for the bar beside it.
func trimmed(label string, width int) string {
	if width < 8 || len(label) <= width {
		return label
	}
	return "…" + strings.TrimSpace(label[len(label)-(width-1):])
}
