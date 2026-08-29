package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
)

// A download says it has finished more than once: the copy counts its last
// chunk, and the code that fetched it reports the total again afterwards. The
// bar it leaves behind is one bar per file, not one per report.
func TestAFinishedDownloadLeavesOneBar(t *testing.T) {
	reporter := &reporter{tty: true, stop: make(chan struct{})}

	printed := captured(t, func() {
		reporter.Progress("spectrum.jar", 0, 90<<20)
		reporter.Progress("spectrum.jar", 45<<20, 90<<20)
		reporter.Progress("spectrum.jar", 90<<20, 90<<20)
		reporter.Progress("spectrum.jar", 90<<20, 90<<20)

		// The next file gets a bar of its own.
		reporter.Progress("dependency.jar", 0, 2<<20)
		reporter.Progress("dependency.jar", 2<<20, 2<<20)
	})

	if bars := strings.Count(printed, "100%"); bars != 2 {
		t.Errorf("two downloads left %d bars behind:\n%s", bars, printed)
	}
	if !strings.Contains(printed, "spectrum.jar") || !strings.Contains(printed, "dependency.jar") {
		t.Errorf("a download left no bar at all:\n%s", printed)
	}
}

// captured runs a function with standard output redirected, and returns what
// it printed.
func captured(t *testing.T, run func()) string {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("no pipe to capture output with: %v", err)
	}

	stdout := os.Stdout
	os.Stdout = write

	run()

	os.Stdout = stdout
	if err := write.Close(); err != nil {
		t.Fatalf("the capture pipe would not close: %v", err)
	}

	printed, err := io.ReadAll(read)
	if err != nil {
		t.Fatalf("the captured output could not be read: %v", err)
	}
	return string(printed)
}
