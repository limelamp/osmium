package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/tui/styles"
)

// reporter draws what a long command is doing: its output as it arrives, and
// underneath it one live line — a spinner while it waits, a progress bar while
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

	// A finished download leaves nothing to animate.
	if total > 0 && done >= total {
		r.label, r.done, r.total = "", 0, 0
		r.clear()
	}
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
func (r *reporter) draw() {
	if !r.tty || r.label == "" {
		return
	}

	line := styles.Spinner(r.frame) + " " + r.label
	if r.total > 0 {
		line += "  " + styles.ProgressBar(78, r.done, r.total)
	}

	fmt.Print("\r\033[K" + line)
	r.shown = true
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
// finished — resolving a version, asking Modrinth. The returned function ends
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

// trimmed keeps a label short enough for one line.
func trimmed(label string) string {
	if len(label) <= 48 {
		return label
	}
	return "…" + strings.TrimSpace(label[len(label)-47:])
}
