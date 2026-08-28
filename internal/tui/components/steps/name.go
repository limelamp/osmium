package steps

import (
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// NameStep names the server. The name is what the Manage Servers list shows,
// and for the default location it also becomes the folder name.
type NameStep struct {
	config *Config
	layout core.Layout

	input textinput.Model
	warn  string
}

func NewNameStep(config *Config) *NameStep {
	ti := textinput.New()
	ti.Placeholder = "My survival server"
	ti.Prompt = "› "
	ti.CharLimit = 60

	return &NameStep{config: config, input: ti}
}

func (s *NameStep) Init() tea.Cmd { return s.input.Focus() }

func (s *NameStep) Title() string { return "Name the Server" }

func (s *NameStep) Subtitle() string {
	return "How the server is listed in Manage Servers."
}

func (s *NameStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "enter" {
		name := strings.TrimSpace(s.input.Value())
		if name == "" {
			s.warn = "Give the server a name to continue."
			return s, nil
		}

		s.warn = ""
		s.config.Name = name
		return s, Next()
	}

	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

func (s *NameStep) View() tea.View {
	rows := []string{s.input.View()}

	if s.config.Dir != "" {
		rows = append(rows, "",
			lipgloss.NewStyle().Foreground(theme.Inactive).Render("Will be created in"),
			lipgloss.NewStyle().Foreground(theme.Accent).Render(s.targetDir()),
		)
	}

	if s.warn != "" {
		rows = append(rows, "", styles.Error(errString(s.warn)))
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// targetDir previews where the files will land.
func (s *NameStep) targetDir() string {
	name := strings.TrimSpace(s.input.Value())
	if name == "" {
		return s.config.Dir
	}
	return filepath.Join(s.config.Dir, slug(name))
}

func (s *NameStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.input.SetWidth(max(l.Width-4, 10))
	return s
}

// slug turns a display name into a filesystem-friendly folder name.
func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '.':
			b.WriteRune('-')
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "server"
	}
	return out
}

// errString lets a plain warning reuse the shared error styling.
type errString string

func (e errString) Error() string { return string(e) }

// Scroll reports no scrolling: the name step is a single field.
func (s *NameStep) Scroll() core.Scroll { return core.Scroll{} }
