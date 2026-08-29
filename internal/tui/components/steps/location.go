package steps

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// The choices offered on the location step, in display order.
const (
	locationCurrent = iota
	locationChoose
	locationDefault
	locationCount
)

var locationLabels = []string{
	"Current folder",
	"Choose a folder…",
	"Default folder",
}

// LocationStep picks the directory the new server is created in. Choosing
// "Choose a folder…" swaps the step into a browser that can also be driven by
// typing a path directly.
type LocationStep struct {
	config *Config
	layout core.Layout

	list core.List

	// browsing is true once the folder browser has taken over the step.
	browsing bool
	dir      string
	entries  []string

	// path is the editable path bar, opened with tab.
	path core.Field

	defaultDir string
	err        error
}

// NewLocationStep builds the step. defaultDir is the configured servers folder.
func NewLocationStep(cfg *Config, defaultDir string) *LocationStep {
	cwd, _ := os.Getwd()

	s := &LocationStep{
		config:     cfg,
		dir:        cwd,
		path:       core.NewField("").WithLimit(300),
		defaultDir: defaultDir,
	}
	s.list = s.list.Resize(locationCount, locationCount)
	return s
}

func (s *LocationStep) Init() tea.Cmd { return nil }

// Browsing reports whether the folder browser is open, which the wizard checks
// before treating backspace as "go back a step".
func (s *LocationStep) Browsing() bool { return s.browsing }

func (s *LocationStep) Title() string {
	if s.browsing {
		return "Choose a Folder"
	}
	return "Select Location"
}

func (s *LocationStep) Subtitle() string {
	if s.browsing {
		return "Browse with the arrow keys, or press tab to type a path."
	}
	return "Pick where the new server's files should live."
}

func (s *LocationStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if s.browsing {
		return s.updateBrowser(msg)
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		list, activated := s.list.HandleMouse(msg, s.layout, 0)
		s.list = list
		if activated {
			return s.choose()
		}
		return s, nil

	case tea.KeyPressMsg:
		if list, handled := s.list.HandleKey(msg.String()); handled {
			s.list = list
			return s, nil
		}

		switch msg.String() {
		case "enter", "space", "right", "l":
			return s.choose()
		}
	}

	return s, nil
}

// choose acts on the highlighted location option.
func (s *LocationStep) choose() (tea.Model, tea.Cmd) {
	switch s.list.Cursor {
	case locationCurrent:
		cwd, err := os.Getwd()
		if err != nil {
			s.err = err
			return s, nil
		}
		s.config.Dir = cwd
		return s, Next()

	case locationDefault:
		s.config.Dir = s.defaultDir
		return s, Next()

	default:
		s.browsing = true
		cmd := s.openDir(s.dir)
		return s, cmd
	}
}

// updateBrowser handles input while the folder browser is showing.
func (s *LocationStep) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		// The path bar takes the top three rows of the body.
		list, activated := s.list.HandleMouse(msg, s.layout.Inset(0, 3, 0, -3), 0)
		s.list = list
		if activated {
			cmd := s.enter()
			return s, cmd
		}
		return s, nil

	case tea.KeyPressMsg:
		if s.path.IsOpen() {
			// Tab closes the bar the way it opened it, leaving the folder alone.
			if msg.String() == "tab" {
				s.path = s.path.Cancel()
				return s, nil
			}

			field, result, cmd := s.path.HandleKey(msg)
			s.path = field

			if result == core.FieldCommitted {
				cmd := s.openDir(strings.TrimSpace(s.path.Value()))
				return s, cmd
			}
			return s, cmd
		}

		if list, handled := s.list.HandleKey(msg.String()); handled {
			s.list = list
			return s, nil
		}

		switch msg.String() {
		case "tab":
			field, cmd := s.path.Open(s.dir)
			s.path = field
			return s, cmd

		case "enter", "right", "l":
			cmd := s.enter()
			return s, cmd

		case "left", "h":
			cmd := s.openDir(filepath.Dir(s.dir))
			return s, cmd

		case "s":
			s.config.Dir = s.dir
			return s, Next()

		case "backspace":
			// Nested navigation: backspace leaves the browser before it leaves
			// the step. Esc is not handled here — it leaves the wizard.
			s.browsing = false
			s.list = s.list.Resize(locationCount, locationCount)
			s.list.Cursor = locationChoose
			return s, nil
		}
	}

	return s, nil
}

// enter descends into the highlighted directory.
func (s *LocationStep) enter() tea.Cmd {
	if len(s.entries) == 0 {
		return nil
	}
	return s.openDir(filepath.Join(s.dir, s.entries[s.list.Cursor]))
}

// openDir lists a directory's sub-folders, keeping the old listing on failure.
func (s *LocationStep) openDir(path string) tea.Cmd {
	if path == "" {
		return nil
	}

	if expanded, err := filepath.Abs(expandHome(path)); err == nil {
		path = expanded
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		s.err = err
		return nil
	}

	names := []string{".."}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names[1:])

	s.err = nil
	s.dir = path
	s.entries = names
	s.list = core.List{}
	s.list = s.list.Resize(len(names), s.browserRows())
	return nil
}

// browserRows is how many directory rows fit under the path bar.
func (s *LocationStep) browserRows() int {
	return max(1, s.layout.Height-4)
}

// expandHome resolves a leading ~ so typed paths behave like shell paths.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

func (s *LocationStep) View() tea.View {
	if s.browsing {
		return tea.NewView(s.browserView())
	}

	rows := make([]string, 0, locationCount)
	for i, label := range locationLabels {
		suffix := ""
		switch i {
		case locationCurrent:
			cwd, _ := os.Getwd()
			suffix = cwd
		case locationDefault:
			suffix = s.defaultDir
		}
		rows = append(rows, styles.ListRow(s.layout.Width, i == s.list.Cursor, label, suffix))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if s.err != nil {
		body += "\n\n" + styles.Error(s.err)
	}
	return tea.NewView(body)
}

// browserView draws the path bar above the directory listing.
func (s *LocationStep) browserView() string {
	pathLabel := lipgloss.NewStyle().Foreground(theme.Inactive).Render("Path")
	pathValue := lipgloss.NewStyle().Foreground(theme.Accent).Render(s.dir)
	if s.path.IsOpen() {
		pathValue = s.path.View()
	}

	entries := s.list.Render(func(i int) string {
		label := s.entries[i]
		if label != ".." {
			label += "/"
		}
		return styles.ListRow(s.layout.Width, i == s.list.Cursor, label, "")
	})

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		pathLabel+"  "+pathValue,
		lipgloss.NewStyle().Foreground(theme.Inactive).
			Render("      [tab] type a path  •  [enter] open  •  [s] select this folder"),
		"",
		entries,
	)

	if s.err != nil {
		body += "\n\n" + styles.Error(s.err)
	}
	return body
}

func (s *LocationStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.path = s.path.SetWidth(l.Width - 10)

	if s.browsing {
		s.list = s.list.Resize(len(s.entries), s.browserRows())
	} else {
		s.list = s.list.Resize(locationCount, locationCount)
	}
	return s
}

// Editing reports whether the path bar has the keyboard.
func (s *LocationStep) Editing() bool { return s.browsing && s.path.IsOpen() }

// Scroll reports the folder listing's position; the option list always fits.
func (s *LocationStep) Scroll() core.Scroll {
	if !s.browsing {
		return core.Scroll{}
	}
	return s.list.Scroll
}
