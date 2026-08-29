package steps

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
	"github.com/limelamp/osmium/internal/util"
)

// VersionsLoadedMsg delivers the Minecraft release list, which comes from the
// network and so must not be fetched inside Update.
type VersionsLoadedMsg struct {
	Versions []string
	Err      error
}

// LoadVersions fetches the released Minecraft versions in the background.
func LoadVersions() tea.Cmd {
	return func() tea.Msg {
		versions, err := util.GetVersionStrings("release")
		return VersionsLoadedMsg{Versions: versions, Err: err}
	}
}

// VersionStep picks the Minecraft version, grouped into one column per release
// line (1.21.x, 1.20.x, …) so the long flat list stays navigable.
type VersionStep struct {
	config *Config
	layout core.Layout

	grid    Grid
	loaded  bool
	loading bool
	err     error
}

// NewVersionStep returns the step that picks the Minecraft version.
func NewVersionStep(cfg *Config) *VersionStep {
	return &VersionStep{config: cfg, loading: true}
}

func (s *VersionStep) Init() tea.Cmd {
	// The manifest is fetched once and then kept: revisiting the step must not
	// hit the network again or throw away the version the user was looking at.
	if s.loaded {
		return nil
	}

	s.loading = true
	return LoadVersions()
}

func (s *VersionStep) Title() string { return "Select Version" }

func (s *VersionStep) Subtitle() string {
	return "Columns group releases by their Minecraft line."
}

func (s *VersionStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case VersionsLoadedMsg:
		s.loading = false
		s.loaded = msg.Err == nil
		s.err = msg.Err
		s.grid = NewGrid(groupVersions(msg.Versions)).SetLayout(s.layout)
		return s, nil
	}

	if s.loading || s.err != nil {
		return s, nil
	}

	grid, activated := s.grid.Update(msg)
	s.grid = grid

	if !activated {
		return s, nil
	}

	if _, version := s.grid.Selected(); version != "" {
		s.config.Version = version
		return s, Next()
	}
	return s, nil
}

func (s *VersionStep) View() tea.View {
	switch {
	case s.err != nil:
		return tea.NewView(styles.Error(s.err))
	case s.loading:
		return tea.NewView(lipgloss.NewStyle().Foreground(theme.Inactive).
			Render("Fetching the Minecraft version manifest…"))
	default:
		return tea.NewView(s.grid.View())
	}
}

func (s *VersionStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.grid = s.grid.SetLayout(l)
	return s
}

// groupVersions buckets flat version strings into one column per release line,
// newest line first and newest patch first inside each column.
func groupVersions(versions []string) []GridColumn {
	order := make([]string, 0, 16)
	byLine := make(map[string][]string)

	for _, version := range versions {
		line := releaseLine(version)
		if _, seen := byLine[line]; !seen {
			order = append(order, line)
		}
		byLine[line] = append(byLine[line], version)
	}

	// The manifest already lists newest first, so the discovery order of the
	// lines is the order we want; only the versions inside need sorting.
	columns := make([]GridColumn, 0, len(order))
	for _, line := range order {
		items := byLine[line]
		sort.SliceStable(items, func(i, j int) bool {
			return comparePatch(items[i], items[j]) > 0
		})
		columns = append(columns, GridColumn{Title: line, Items: items})
	}
	return columns
}

// releaseLine reduces "1.20.4" to its "1.20.x" column title.
func releaseLine(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1] + ".x"
}

// comparePatch orders two versions of the same line by their patch number.
func comparePatch(a, b string) int {
	return patchNumber(a) - patchNumber(b)
}

// patchNumber extracts the trailing patch component, treating "1.20" as 0.
func patchNumber(version string) int {
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return 0
	}

	number := 0
	for _, r := range parts[2] {
		if r < '0' || r > '9' {
			break
		}
		number = number*10 + int(r-'0')
	}
	return number
}

// Scroll reports the version grid's position.
func (s *VersionStep) Scroll() core.Scroll { return s.grid.Scroll() }

// Editing reports that the version grid takes no typed input.
func (s *VersionStep) Editing() bool { return false }
