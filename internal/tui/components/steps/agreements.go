package steps

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// Rows of the agreements step, in display order.
const (
	agreementEula = iota
	agreementAutoRun
	agreementContinue
	agreementCount
)

// AgreementsStep collects the EULA acceptance and the boot-on-setup choice.
type AgreementsStep struct {
	config *Config
	layout core.Layout

	list core.List
	warn string
}

func NewAgreementsStep(config *Config) *AgreementsStep {
	s := &AgreementsStep{config: config}
	s.list = s.list.Resize(agreementCount, agreementCount)
	return s
}

func (s *AgreementsStep) Init() tea.Cmd { return nil }

func (s *AgreementsStep) Title() string { return "Agreements & Options" }

func (s *AgreementsStep) Subtitle() string {
	return "Mojang requires the EULA to be accepted before a server may run."
}

func (s *AgreementsStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		// The rows are spaced out with a link line and blanks, so clicks are
		// mapped through the display layout rather than by row number.
		mouse := msg.Mouse()
		if _, isClick := msg.(tea.MouseClickMsg); !isClick || !s.layout.Hit(mouse.X, mouse.Y) {
			return s, nil
		}

		_, rowIndex := s.rows()
		index, ok := rowIndex[mouse.Y-s.layout.Y]
		if !ok {
			return s, nil
		}
		if index == s.list.Cursor {
			return s.toggle()
		}
		s.list = s.list.MoveTo(index)
		return s, nil

	case tea.KeyPressMsg:
		if list, handled := s.list.HandleKey(msg.String()); handled {
			s.list = list
			return s, nil
		}

		if msg.String() == "enter" || msg.String() == " " {
			return s.toggle()
		}
	}

	return s, nil
}

// toggle flips the highlighted checkbox, or continues from the last row.
func (s *AgreementsStep) toggle() (tea.Model, tea.Cmd) {
	switch s.list.Cursor {
	case agreementEula:
		s.config.Eula = !s.config.Eula
		s.warn = ""

	case agreementAutoRun:
		s.config.AutoRun = !s.config.AutoRun

	case agreementContinue:
		if !s.config.Eula {
			s.warn = "You must accept the Mojang EULA to proceed."
			return s, nil
		}
		return s, Next()
	}

	return s, nil
}

// rows renders the body and reports which display row each option landed on, so
// the view and the click handling can never disagree about the layout.
func (s *AgreementsStep) rows() ([]string, map[int]int) {
	// The link and the blank spacers are the first things to go when the panel
	// is too short to hold every option comfortably.
	spacious := s.layout.Height >= 7

	var rows []string
	index := make(map[int]int, agreementCount)

	add := func(option int, row string) {
		index[len(rows)] = option
		rows = append(rows, row)
	}

	add(agreementEula, styles.ListRow(s.layout.Width, s.list.Cursor == agreementEula,
		checkbox(s.config.Eula)+" Accept the Minecraft EULA", ""))

	if spacious {
		rows = append(rows,
			lipgloss.NewStyle().Foreground(theme.Inactive).PaddingLeft(6).
				Render("https://aka.ms/MinecraftEULA"),
			"")
	}

	add(agreementAutoRun, styles.ListRow(s.layout.Width, s.list.Cursor == agreementAutoRun,
		checkbox(s.config.AutoRun)+" Boot the server once it is built", ""))

	if spacious {
		rows = append(rows, "")
	}

	add(agreementContinue, styles.ListRow(s.layout.Width, s.list.Cursor == agreementContinue,
		"[ Continue ]", ""))

	if s.warn != "" {
		rows = append(rows, "", styles.Error(errString(s.warn)))
	}

	return rows, index
}

func (s *AgreementsStep) View() tea.View {
	rows, _ := s.rows()
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (s *AgreementsStep) SetLayout(l core.Layout) Step {
	s.layout = l
	return s
}

// checkbox renders a tick box in the theme's accent colour when set.
func checkbox(checked bool) string {
	if checked {
		return lipgloss.NewStyle().Foreground(theme.Accent).Render("[✓]")
	}
	return "[ ]"
}

// Scroll reports no scrolling: the options are trimmed to fit instead.
func (s *AgreementsStep) Scroll() core.Scroll { return core.Scroll{} }
