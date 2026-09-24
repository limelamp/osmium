// Package build carries what the binary knows about itself: which version it
// is, which commit it was built from and when.
//
// The values are empty in a plain "go build" and are stamped in by the Makefile
// and by goreleaser with -ldflags -X. A binary that cannot say what it is makes
// a bug report a guessing game, which is the whole reason this exists.
package build

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

var (
	// Version is the release this was built from, e.g. "0.1.0".
	Version = ""

	// Commit is the git revision, shortened.
	Commit = ""

	// Date is when it was built, in RFC 3339.
	Date = ""
)

// devVersion is what an unstamped build calls itself.
const devVersion = "dev"

// String is the one-line description shown by "osmium --version".
func String() string {
	line := "osmium " + Info()

	if detail := details(); detail != "" {
		line += " (" + detail + ")"
	}
	return line
}

// Info is the version on its own, falling back to what the go tool recorded
// when nothing was stamped in - a binary installed straight from a tag knows
// its own version even though nobody passed any flags.
func Info() string {
	if Version != "" {
		return Version
	}

	if tagged := moduleVersion(); tagged != "" {
		return tagged
	}
	return devVersion
}

// moduleVersion is the version the go tool recorded, unless it is one of the
// pseudo-versions it invents for an untagged commit - "v0.0.0-20260829…" says
// less about a build than the word "dev" does.
func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return ""
	}

	if strings.HasPrefix(info.Main.Version, "v0.0.0-") || strings.Contains(info.Main.Version, "+dirty") {
		return ""
	}
	return strings.TrimPrefix(info.Main.Version, "v")
}

// vcs reads what the go tool recorded about the checkout, which is how an
// unstamped build still knows which commit it came from.
func vcs() (commit, date string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", ""
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			commit = setting.Value
			if len(commit) > 7 {
				commit = commit[:7]
			}
		case "vcs.time":
			date, _, _ = strings.Cut(setting.Value, "T")
		}
	}
	return commit, date
}

// details is the commit and date, whichever of them is known.
func details() string {
	commit, date := Commit, Date
	if commit == "" && date == "" {
		commit, date = vcs()
	}

	switch {
	case commit != "" && date != "":
		return commit + ", " + date
	case commit != "":
		return commit
	default:
		return date
	}
}

// Full is the version with the toolchain and platform, for a bug report.
func Full() string {
	return fmt.Sprintf("%s\n%s on %s/%s", String(), runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
