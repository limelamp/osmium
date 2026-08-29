package steps

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// NameStep names the server. The name is what the Manage Servers list shows,
// and for the default location it also becomes the folder name.
//
// The field is opened before it is typed into, the same way a config value or
// the default memory is: enter opens it, enter saves it, esc cancels the edit.
// While it is closed the step is ordinary navigation, so esc steps back through
// the wizard instead of leaving it.
type NameStep struct {
	config *Config
	layout core.Layout

	field core.Field
	warn  string
}

func NewNameStep(config *Config) *NameStep {
	return &NameStep{
		config: config,
		field:  core.NewField("My survival server").WithLimit(60),
	}
}

func (s *NameStep) Init() tea.Cmd { return nil }

func (s *NameStep) Title() string { return "Name the Server" }

func (s *NameStep) Subtitle() string {
	return "How the server is listed in Manage Servers."
}

// Editing reports whether the field is open for typing.
func (s *NameStep) Editing() bool { return s.field.IsOpen() }

func (s *NameStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	if s.field.IsOpen() {
		field, result, cmd := s.field.HandleKey(key)
		s.field = field

		if result == core.FieldCommitted {
			return s, s.commit()
		}
		s.warn = ""
		return s, cmd
	}

	if key.String() == "enter" || key.String() == "space" {
		field, cmd := s.field.Open(s.config.Name)
		s.field = field
		return s, cmd
	}

	return s, nil
}

// commit records the typed name, refusing an empty one.
func (s *NameStep) commit() tea.Cmd {
	name := strings.TrimSpace(s.field.Value())
	if name == "" {
		s.warn = "Give the server a name to continue."

		field, cmd := s.field.Open("")
		s.field = field
		return cmd
	}

	s.warn = ""
	s.config.Name = name
	return Next()
}

func (s *NameStep) View() tea.View {
	field := styles.ListRow(s.layout.Width, true, s.name(), "")
	if s.field.IsOpen() {
		field = s.field.View()
	}

	rows := []string{field}

	if !s.field.IsOpen() {
		rows = append(rows, "",
			lipgloss.NewStyle().Foreground(theme.Inactive).Render("[enter] type a name"))
	}

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

// name is what the closed field shows.
func (s *NameStep) name() string {
	if s.config.Name == "" {
		return lipgloss.NewStyle().Foreground(theme.Inactive).Render("Unnamed server")
	}
	return s.config.Name
}

// targetDir previews where the files will land.
func (s *NameStep) targetDir() string {
	name := strings.TrimSpace(s.field.Value())
	if !s.field.IsOpen() {
		name = s.config.Name
	}
	if name == "" {
		return s.config.Dir
	}
	return filepath.Join(s.config.Dir, slug(name))
}

func (s *NameStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.field = s.field.SetWidth(l.Width - 4)
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
