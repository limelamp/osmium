package components

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// KeyMap defines the system-wide key bindings.
type KeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Help  key.Binding
	Quit  key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns keybindings to be shown in the expanded help menu.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Help, k.Quit},
	}
}

// DefaultKeys represents the global system-wide keybindings.
var DefaultKeys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move right"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// HelpModel encapsulates the layout, state, and rendering of the help overlay modal.
type HelpModel struct {
	help     help.Model
	keys     KeyMap
	showHelp bool
	width    int
	height   int
}

// NewHelpModel returns an initialized HelpModel with specified key mappings.
func NewHelpModel(keys KeyMap) HelpModel {
	return HelpModel{
		help: help.New(),
		keys: keys,
	}
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
		m.help.SetWidth(msg.Width)

	case tea.KeyPressMsg:
		if m.showHelp {
			if key.Matches(msg, m.keys.Help) || msg.String() == "esc" {
				m.showHelp = false
			}
		} else {
			if key.Matches(msg, m.keys.Help) {
				m.showHelp = true
			}
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

// Render takes the background text content and composites the help modal centered over it, if visible.
func (m HelpModel) Render(baseView string) string {
	if !m.showHelp {
		return baseView
	}

	// 1. Build and configure dynamic theme elements based on the active theme
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true)

	footerStyle := lipgloss.NewStyle().
		Foreground(theme.Inactive)

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(1, 2).
		Width(45)

	// 2. Generate the formatted content inside the modal
	helpContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Help Menu"),
		"",
		m.help.FullHelpView(m.keys.FullHelp()),
		"",
		footerStyle.Render("Press ? or ESC to close"),
	)

	modal := modalStyle.Render(helpContent)

	// 3. Compute structural horizontal and vertical centering coordinates
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	x := (m.width - modalWidth) / 2
	y := (m.height - modalHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	// Normalize the size of the base background layer to guarantee proper positioning
	baseStyledContent := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(baseView)

	// 4. Composite the base layout and centered modal layer
	baseLayer := lipgloss.NewLayer(baseStyledContent)
	modalLayer := lipgloss.NewLayer(modal).X(x).Y(y).Z(1)

	comp := lipgloss.NewCompositor(baseLayer, modalLayer)
	return comp.Render()
}
