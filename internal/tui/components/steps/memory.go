package steps

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/constants"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// MemoryStep allocates the JVM heap the server runs with, either from the
// presets or as a typed amount.
type MemoryStep struct {
	config *Config
	layout core.Layout

	list core.List

	// The custom amount is typed into the app's shared field, the same one the
	// settings page and the other steps use.
	field core.Field
	warn  string
}

// NewMemoryStep returns the step that allocates memory.
func NewMemoryStep(cfg *Config) *MemoryStep {
	s := &MemoryStep{
		config: cfg,
		field:  core.NewField("e.g. 6G, 10G or 3072M").WithLimit(12),
	}
	s.list = s.list.Resize(len(constants.RamOptions), len(constants.RamOptions))

	// Start on whatever the configuration already carries, so the wizard's
	// default from Settings is the highlighted row.
	for i, option := range constants.RamOptions {
		if option == cfg.RAM {
			s.list.Cursor = i
			break
		}
	}
	return s
}

func (s *MemoryStep) Init() tea.Cmd { return nil }

func (s *MemoryStep) Title() string { return "Allocate Memory" }

func (s *MemoryStep) Subtitle() string {
	return "How much RAM the server may use. 4G suits most small servers."
}

// Editing reports whether the custom amount field has the keyboard.
func (s *MemoryStep) Editing() bool { return s.field.IsOpen() }

func (s *MemoryStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if s.field.IsOpen() {
		return s.updateInput(msg)
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		list, activated := s.list.HandleMouse(msg, s.layout, 0)
		s.list = list
		if activated {
			cmd := s.confirm(constants.RamOptions[s.list.Cursor])
			return s, cmd
		}
		return s, nil

	case tea.KeyPressMsg:
		if list, handled := s.list.HandleKey(msg.String()); handled {
			s.list = list
			return s, nil
		}

		switch msg.String() {
		case "tab":
			field, cmd := s.field.Open(s.config.RAM)
			s.field = field
			return s, cmd

		case "enter", "space":
			cmd := s.confirm(constants.RamOptions[s.list.Cursor])
			return s, cmd
		}
	}

	return s, nil
}

// updateInput handles keys while the custom amount is being typed.
func (s *MemoryStep) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	// Tab closes the field the way it opened it, leaving the value alone.
	if key.String() == "tab" {
		s.warn = ""
		s.field = s.field.Cancel()
		return s, nil
	}

	field, result, cmd := s.field.HandleKey(key)
	s.field = field

	switch result {
	case core.FieldCommitted:
		amount, err := NormaliseRAM(s.field.Value())
		if err != nil {
			s.warn = err.Error()

			reopened, cmd := s.field.Open(s.field.Value())
			s.field = reopened
			return s, cmd
		}

		s.warn = ""
		cmd := s.confirm(amount)
		return s, cmd

	case core.FieldCancelled:
		s.warn = ""
	}

	return s, cmd
}

// confirm records an allocation and moves on.
func (s *MemoryStep) confirm(amount string) tea.Cmd {
	s.config.RAM = amount
	return Next()
}

func (s *MemoryStep) View() tea.View {
	options := s.list.Render(func(i int) string {
		return styles.ListRow(s.layout.Width, !s.field.IsOpen() && i == s.list.Cursor,
			constants.RamOptions[i], "")
	})

	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	custom := muted.Render("Custom amount") + "   " + muted.Render("[tab] type an amount")
	if s.field.IsOpen() {
		custom = muted.Render("Custom amount") + "   " + s.field.View()
	}

	body := []string{
		options,
		"",
		custom,
	}

	if s.warn != "" {
		body = append(body, "", styles.Error(errString(s.warn)))
	} else {
		body = append(body, "", muted.Render("Both -Xms and -Xmx are set to this value."))
	}

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, body...))
}

func (s *MemoryStep) SetLayout(l core.Layout) Step {
	s.layout = l
	s.field = s.field.SetWidth(l.Width / 3)
	// The custom field and the note sit under the list, so the list gets what
	// is left of the body.
	s.list = s.list.Resize(len(constants.RamOptions), max(l.Height-4, 1))
	return s
}

// Scroll reports the allocation list's position.
func (s *MemoryStep) Scroll() core.Scroll { return s.list.Scroll }

// NormaliseRAM turns a typed amount into the "4G" / "3072M" form the JVM flags
// expect, defaulting a bare number to gigabytes. The settings page uses it too,
// so a custom amount is validated the same way wherever it is entered.
func NormaliseRAM(input string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(input))
	if value == "" {
		return "", fmt.Errorf("enter an amount, for example 6G")
	}

	unit := "G"
	if last := value[len(value)-1]; last == 'G' || last == 'M' {
		unit = string(last)
		value = value[:len(value)-1]
	}

	amount, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || amount <= 0 {
		return "", fmt.Errorf("%q is not a valid amount — try 6G or 3072M", input)
	}

	return fmt.Sprintf("%d%s", amount, unit), nil
}
