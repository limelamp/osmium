package components

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// HelpModel is the help overlay. It draws whatever the active page says is
// live right now, so the keys it lists are the keys that actually work — down
// to the pane and the nested screen in front of the user — rather than one
// fixed list for the whole app.
type HelpModel struct {
	showHelp bool
	width    int
	height   int
}

// NewHelpModel returns an initialised HelpModel.
func NewHelpModel() HelpModel {
	return HelpModel{}
}

// Init initializes the help component.
func (m HelpModel) Init() tea.Cmd {
	return nil
}

// Update handles standard TUI messages, including toggling open/close states on key inputs.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		switch key := msg.String(); {
		case m.showHelp && (key == "?" || key == "esc"):
			m.showHelp = false
		case !m.showHelp && key == "?":
			m.showHelp = true
		}
	}
	return m, nil
}

// SetVisible explicitly sets whether the help overlay is shown.
func (m HelpModel) SetVisible(visible bool) HelpModel {
	m.showHelp = visible
	return m
}

// IsVisible returns whether the help overlay is currently shown.
func (m HelpModel) IsVisible() bool {
	return m.showHelp
}

// helpColumns is how many key/description pairs are put on one row.
const helpColumns = 2

// Render composites the help modal over the page, listing the sections the page
// reported plus the keys that work everywhere.
func (m HelpModel) Render(baseView string, sections []core.HelpSection) string {
	if !m.showHelp {
		return baseView
	}

	modalWidth := min(max(m.width-8, 20), 76)

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(1, 2).
		Width(modalWidth)

	sections = append(sections, core.HelpSection{Title: "Anywhere", Hints: core.GlobalHints})

	blocks := make([]string, 0, len(sections)*2)
	for i, section := range sections {
		if i > 0 {
			blocks = append(blocks, "")
		}
		blocks = append(blocks, m.section(section, modalWidth-4))
	}

	blocks = append(blocks, "",
		lipgloss.NewStyle().Foreground(theme.Inactive).Render("Press ? or esc to close"))

	modal := modalStyle.Render(lipgloss.JoinVertical(lipgloss.Left, blocks...))

	// Centre the modal over a background normalised to the terminal size, so it
	// lands in the same place whatever the page under it drew.
	x := max((m.width-lipgloss.Width(modal))/2, 0)
	y := max((m.height-lipgloss.Height(modal))/2, 0)

	base := lipgloss.NewStyle().Width(m.width).Height(m.height).Render(baseView)

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(modal).X(x).Y(y).Z(1),
	).Render()
}

// section renders one titled block of bindings in even columns.
func (m HelpModel) section(section core.HelpSection, width int) string {
	title := lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render(section.Title)
	if len(section.Hints) == 0 {
		return title
	}

	keyStyle := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(theme.Inactive)

	// Every key column is as wide as the widest key, so the descriptions line
	// up down the modal instead of stepping in and out.
	keyWidth := 0
	for _, hint := range section.Hints {
		keyWidth = max(keyWidth, lipgloss.Width(hint.Key))
	}

	cellWidth := max(width/helpColumns, 12)

	rows := make([]string, 0, (len(section.Hints)+helpColumns-1)/helpColumns)
	for i := 0; i < len(section.Hints); i += helpColumns {
		cells := make([]string, 0, helpColumns)

		for _, hint := range section.Hints[i:min(i+helpColumns, len(section.Hints))] {
			cell := keyStyle.Width(keyWidth+2).Render(hint.Key) + descStyle.Render(hint.Desc)
			cells = append(cells, lipgloss.NewStyle().Width(cellWidth).MaxWidth(cellWidth).Render(cell))
		}

		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, append([]string{title}, rows...)...)
}
