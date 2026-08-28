package steps

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
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

	// typing is true while the custom amount field has the keyboard, the same
	// arrangement the location step uses for its path bar.
	typing bool
	input  textinput.Model
	warn   string
}

func NewMemoryStep(config *Config) *MemoryStep {
	ti := textinput.New()
	ti.Placeholder = "e.g. 6G, 10G or 3072M"
	ti.Prompt = "› "
	ti.CharLimit = 12

	s := &MemoryStep{config: config, input: ti}
	s.list = s.list.Resize(len(constants.RamOptions), len(constants.RamOptions))

	// Start on whatever the configuration already carries, so the wizard's
	// default from Settings is the highlighted row.
	for i, option := range constants.RamOptions {
		if option == config.RAM {
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
func (s *MemoryStep) Editing() bool { return s.typing }

func (s *MemoryStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if s.typing {
		return s.updateInput(msg)
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		list, activated := s.list.HandleMouse(msg, s.layout, 0)
		s.list = list
		if activated {
			return s, s.confirm(constants.RamOptions[s.list.Cursor])
		}
		return s, nil

	case tea.KeyPressMsg:
		if list, handled := s.list.HandleKey(msg.String()); handled {
			s.list = list
			return s, nil
		}

		switch msg.String() {
		case "tab":
			s.typing = true
			s.input.SetValue(s.config.RAM)
			s.input.CursorEnd()
			return s, s.input.Focus()

		case "enter", "space":
			return s, s.confirm(constants.RamOptions[s.list.Cursor])
		}
	}

	return s, nil
}

// updateInput handles keys while the custom amount is being typed.
func (s *MemoryStep) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "enter":
			amount, err := NormaliseRAM(s.input.Value())
			if err != nil {
				s.warn = err.Error()
				return s, nil
			}

			s.warn = ""
			s.typing = false
			s.input.Blur()
			return s, s.confirm(amount)

		case "tab", "esc":
			s.warn = ""
			s.typing = false
			s.input.Blur()
			return s, nil
		}
	}

	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

// confirm records an allocation and moves on.
func (s *MemoryStep) confirm(amount string) tea.Cmd {
	s.config.RAM = amount
	return Next()
}

func (s *MemoryStep) View() tea.View {
	start, end := s.list.Window()

	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rows = append(rows, styles.ListRow(s.layout.Width, !s.typing && i == s.list.Cursor,
			constants.RamOptions[i], ""))
	}

	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	custom := muted.Render("Custom amount") + "   " + muted.Render("[tab] type an amount")
	if s.typing {
		custom = muted.Render("Custom amount") + "   " + s.input.View()
	}

	body := []string{
		lipgloss.JoinVertical(lipgloss.Left, rows...),
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
	s.input.SetWidth(max(l.Width/3, 12))
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
