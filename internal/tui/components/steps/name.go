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
//
// The field is opened before it is typed into, the same way a config value or
// the default memory is: enter opens it, enter saves it, esc cancels the edit.
// While it is closed the step is ordinary navigation, so esc steps back through
// the wizard instead of leaving it.
type NameStep struct {
	config *Config
	layout core.Layout

	typing bool
	input  textinput.Model
	warn   string
}

func NewNameStep(config *Config) *NameStep {
	ti := textinput.New()
	ti.Placeholder = "My survival server"
	ti.Prompt = "› "
	ti.CharLimit = 60

	return &NameStep{config: config, input: ti}
}

func (s *NameStep) Init() tea.Cmd { return nil }

func (s *NameStep) Title() string { return "Name the Server" }

func (s *NameStep) Subtitle() string {
	return "How the server is listed in Manage Servers."
}

// Editing reports whether the field is open for typing.
func (s *NameStep) Editing() bool { return s.typing }

func (s *NameStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	if s.typing {
		return s.updateInput(key)
	}

	switch key.String() {
	case "enter", "space":
		return s, s.open()

	case "esc":
		// Nothing is being typed, so esc is navigation: back one step.
		return s, Prev()
	}

	return s, nil
}

// open puts the keyboard in the field, on the end of whatever is already there.
func (s *NameStep) open() tea.Cmd {
	s.typing = true
	s.input.SetValue(s.config.Name)
	s.input.CursorEnd()
	return s.input.Focus()
}

// updateInput handles keys while the name is being typed.
func (s *NameStep) updateInput(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "enter":
		name := strings.TrimSpace(s.input.Value())
		if name == "" {
			s.warn = "Give the server a name to continue."
			return s, nil
		}

		s.warn = ""
		s.typing = false
		s.input.Blur()
		s.config.Name = name
		return s, Next()

	case "esc":
		// The edit is abandoned: the field goes back to the saved name.
		s.warn = ""
		s.typing = false
		s.input.SetValue(s.config.Name)
		s.input.Blur()
		return s, nil
	}

	var cmd tea.Cmd
	s.input, cmd = s.input.Update(key)
	return s, cmd
}

func (s *NameStep) View() tea.View {
	field := styles.ListRow(s.layout.Width, true, s.name(), "")
	if s.typing {
		field = s.input.View()
	}

	rows := []string{field}

	if !s.typing {
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
	name := strings.TrimSpace(s.input.Value())
	if !s.typing {
		name = s.config.Name
	}
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
