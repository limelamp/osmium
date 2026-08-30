package pack

import (
	"fmt"
	"sort"
	"strings"

	"github.com/limelamp/osmium/internal/config"
)

// Action is what a plan will do to one project.
type Action int

// The actions, in the order they are applied and listed.
const (
	Install Action = iota
	Upgrade
	Downgrade
	Reinstall
	Remove
)

// Verb names an action for a line of output.
func (a Action) Verb() string {
	switch a {
	case Install:
		return "install"
	case Upgrade:
		return "upgrade"
	case Downgrade:
		return "downgrade"
	case Reinstall:
		return "reinstall"
	case Remove:
		return "remove"
	}
	return "?"
}

// Change is one project a plan will act on.
type Change struct {
	Action Action
	Kind   config.Kind
	Slug   string
	Title  string

	// From and To are the build numbers either side of the change, either of
	// which may be empty for an install or a removal.
	From string
	To   string

	// Entry is what the lock will say once this change has been applied. It is
	// empty for a removal.
	Entry config.Locked

	// Why says where the change came from — asked for by name, needed by
	// something else, or no longer needed by anything.
	Why string

	// Size is the download, and Cached says it has been downloaded before and
	// will cost nothing this time.
	Size   int64
	Cached bool
}

// Suggestion is an optional dependency that was not installed.
type Suggestion struct {
	Slug   string
	Title  string
	Wanted string // what suggested it
}

// Plan is everything a command will do, worked out before anything is done.
//
// Nothing about a plan touches the disk. It exists so that the whole of an
// operation can be shown, judged and agreed to as one thing — which is the
// difference between a package manager and a loop that downloads files.
type Plan struct {
	Changes   []Change
	Optional  []Suggestion
	Untracked []string
	Notes     []string

	// Conflicts are the reasons this plan cannot be applied: two mods that
	// refuse to sit together, a pin with no build to satisfy it, a project
	// with nothing published for this server.
	Conflicts []string

	// wants is the manifest this plan would leave behind. It travels with the
	// plan so that applying one writes exactly what was shown and agreed to,
	// rather than working it out a second time from a server that has since
	// been changed underneath it.
	wants *Wanted
}

// downloads are the files the plan needs before it can be applied.
func (p Plan) downloads() []wanted {
	items := make([]wanted, 0, len(p.Changes))
	for _, change := range p.Changes {
		if change.Action == Remove || change.Entry.DownloadURL == "" {
			continue
		}

		items = append(items, wanted{
			name: change.Entry.FileName,
			urls: []string{change.Entry.DownloadURL},
			sha1: change.Entry.SHA1,
			size: change.Size,
		})
	}
	return items
}

// Empty reports whether there is nothing to do.
func (p Plan) Empty() bool { return len(p.Changes) == 0 }

// Blocked reports whether the plan cannot be applied as it stands.
func (p Plan) Blocked() bool { return len(p.Conflicts) > 0 }

// Loses reports whether anything is removed or moved backwards, which is what
// a command asks about before doing it.
func (p Plan) Loses() bool {
	for _, change := range p.Changes {
		if change.Action == Remove || change.Action == Downgrade {
			return true
		}
	}
	return false
}

// Download is how many bytes the plan will fetch, ignoring what is cached.
func (p Plan) Download() int64 {
	var total int64
	for _, change := range p.Changes {
		if change.Action != Remove && !change.Cached {
			total += change.Size
		}
	}
	return total
}

// Counts is how many of each action the plan holds.
func (p Plan) Counts() map[Action]int {
	counts := make(map[Action]int, 5)
	for _, change := range p.Changes {
		counts[change.Action]++
	}
	return counts
}

// Summary is the one-line statement of what the plan does.
func (p Plan) Summary() string {
	counts := p.Counts()

	var parts []string
	for _, action := range []Action{Install, Upgrade, Downgrade, Reinstall, Remove} {
		if counts[action] > 0 {
			parts = append(parts, fmt.Sprintf("%d to %s", counts[action], action.Verb()))
		}
	}

	if len(parts) == 0 {
		return "nothing to do"
	}

	summary := strings.Join(parts, ", ")
	if bytes := p.Download(); bytes > 0 {
		summary += fmt.Sprintf("   %s to download", FormatBytes(bytes))
	} else {
		summary += "   nothing to download"
	}

	return summary
}

// Lines are the plan written out, one project per line, for the command line
// and for the panel to show.
func (p Plan) Lines() []string {
	lines := make([]string, 0, len(p.Changes)+len(p.Optional)+len(p.Untracked)+2)

	for _, change := range p.Changes {
		lines = append(lines, change.Line())
	}

	if len(p.Optional) > 0 {
		lines = append(lines, "", "optional, not installed:")
		for _, suggestion := range p.Optional {
			lines = append(lines, fmt.Sprintf("  %s  (suggested by %s)", suggestion.Name(), suggestion.Wanted))
		}
	}

	if len(p.Untracked) > 0 {
		lines = append(lines, "", "not managed by Osmium:", "  "+strings.Join(p.Untracked, ", "))
	}

	for _, note := range p.Notes {
		lines = append(lines, "  "+note)
	}

	return lines
}

// Line is one change written out.
func (c Change) Line() string {
	name := c.Title
	if name == "" {
		name = c.Slug
	}

	line := fmt.Sprintf("%-9s %s", c.Action.Verb(), name)

	switch c.Action {
	case Remove:
		line += " " + c.From
	case Upgrade, Downgrade:
		line += fmt.Sprintf(" %s → %s", c.From, c.To)
	default:
		line += " " + c.To
	}

	if c.Why != "" {
		line += "   " + c.Why
	}
	if c.Cached && c.Action != Remove {
		line += " (cached)"
	}

	return line
}

// Name is what to call a suggested project.
func (s Suggestion) Name() string {
	if s.Title != "" {
		return s.Title
	}
	return s.Slug
}

// sortChanges puts a plan in the order it reads best and applies safest:
// removals last, so a file is never gone before the thing replacing it is
// known to be in hand, and alphabetical within each action.
func sortChanges(changes []Change) {
	sort.SliceStable(changes, func(a, b int) bool {
		if changes[a].Action != changes[b].Action {
			return changes[a].Action < changes[b].Action
		}
		return changes[a].Slug < changes[b].Slug
	})
}

// FormatBytes renders a byte count the way a download is usually quoted. It
// lives here as well as in the interface's styles because the resolver has no
// business importing anything that draws.
func FormatBytes(count int64) string {
	const unit = 1024

	if count < unit {
		return fmt.Sprintf("%d B", count)
	}

	value, exponent := float64(count)/unit, 0
	for value >= unit && exponent < 3 {
		value, exponent = value/unit, exponent+1
	}

	return fmt.Sprintf("%.1f %cB", value, "KMGT"[exponent])
}
