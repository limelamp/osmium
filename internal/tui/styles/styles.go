package styles

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/theme"
)

func Title(width int, text string, location string) string {
	// 1 padding left + 1 padding right = 2 characters total
	padding := 1
	innerWidth := max(width-(padding*2), 0)

	styledText := lipgloss.NewStyle().Render(text)

	// Inner text takes up exactly width - 2
	styledLocation := lipgloss.NewStyle().
		Width(innerWidth - lipgloss.Width(styledText)).
		Align(lipgloss.Right).
		Render(location)

	content := lipgloss.JoinHorizontal(0, styledText, styledLocation)

	return lipgloss.NewStyle().
		Foreground(theme.Primary).
		Background(theme.Inactive).
		Border(lipgloss.NormalBorder(), false, true, false, true).
		BorderForeground(theme.Inactive).
		Bold(true).
		Render(content)
}

func Logo(logo string) (string, int) {
	return lipgloss.NewStyle().
		Foreground(theme.Primary).
		Render(logo), lipgloss.Width(logo)
}

func Selection(width int, isSelected bool, page string, key string) string {
	selectionStyle := lipgloss.NewStyle().Width(width).MarginTop(1)

	var pageContent string
	if isSelected {
		selectionStyle = selectionStyle.Foreground(theme.Primary).Bold(true)
		pageContent = fmt.Sprintf("> %s", page)
	} else {
		selectionStyle = selectionStyle.Foreground(lipgloss.Color("252"))
		pageContent = fmt.Sprintf("  %s", page)
	}

	pageStyle := lipgloss.NewStyle().Render(pageContent)
	keyStyle := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Width(width - lipgloss.Width(pageStyle)).
		Align(lipgloss.Right).
		Render(key)

	return selectionStyle.Render(pageStyle + keyStyle)
}

// ContainerOpts describes one bordered panel. Every panel in the app is drawn
// through Container so borders, focus colouring, padding and the scrollbar stay
// identical everywhere.
type ContainerOpts struct {
	Width   int
	Height  int
	Focused bool
	Title   string
	Content string

	// ShowHelp draws the "[?] help" bottom border instead of a plain one.
	ShowHelp bool

	// Scroll* describe the panel's scroll state. The scrollbar is only drawn
	// when there is more content than fits, so panels that always fit never
	// have to opt out explicitly.
	ScrollOffset  int
	ScrollTotal   int
	ScrollVisible int
}

// Container renders a titled, bordered panel.
func Container(o ContainerOpts) string {
	var borderColor color.Color
	if o.Focused {
		borderColor = theme.Primary
	} else {
		borderColor = theme.Inactive
	}

	b := lipgloss.NormalBorder()
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	titleText := borderStyle.Render(" " + o.Title + " ")
	topLineLen := max(o.Width-2-lipgloss.Width(titleText), 0)

	topLine := borderStyle.Render(b.TopLeft) +
		titleText +
		borderStyle.Render(strings.Repeat(b.Top, topLineLen)+b.TopRight)

	innerHeight := max(o.Height-2, 0)
	inner := lipgloss.NewStyle().
		Padding(0, 1).
		Width(max(o.Width-2, 0)).
		Height(innerHeight).
		Render(o.Content)

	// Draw the side borders row by row so the scrollbar thumb can replace the
	// right border on the rows it covers.
	thumb := scrollThumb(innerHeight, o.ScrollTotal, o.ScrollVisible, o.ScrollOffset)
	thumbStyle := lipgloss.NewStyle().Foreground(theme.Primary)

	rows := strings.Split(inner, "\n")

	// Content that outgrew the panel is cut off rather than pushing the bottom
	// border off screen: no panel may ever draw outside its own rectangle.
	if len(rows) > innerHeight {
		rows = rows[:innerHeight]
	}

	for i, row := range rows {
		right := borderStyle.Render(b.Right)
		if i < len(thumb) && thumb[i] {
			right = thumbStyle.Render("┃")
		}
		rows[i] = borderStyle.Render(b.Left) + row + right
	}
	mainBox := strings.Join(rows, "\n")

	bottomLabel := ""
	if o.ShowHelp {
		bottomLabel = lipgloss.NewStyle().Foreground(theme.Accent).Render(" [?] help ")
	}
	bottomLineLen := max(o.Width-2-lipgloss.Width(bottomLabel), 0)
	bottomLine := borderStyle.Render(b.BottomLeft+strings.Repeat(b.Bottom, bottomLineLen)) +
		bottomLabel +
		borderStyle.Render(b.BottomRight)

	return lipgloss.JoinVertical(0, topLine, mainBox, bottomLine)
}

// scrollThumb marks which of the track's rows the scrollbar thumb covers.
// It returns nil when the content fits, which is what keeps the scrollbar from
// showing up on panels that do not need it.
func scrollThumb(track, total, visible, offset int) []bool {
	if track <= 0 || visible <= 0 || total <= visible {
		return nil
	}

	thumbHeight := max(1, track*visible/total)
	thumbTop := 0
	if maxOffset := total - visible; maxOffset > 0 {
		thumbTop = (track - thumbHeight) * offset / maxOffset
	}
	thumbTop = min(thumbTop, track-thumbHeight)

	marks := make([]bool, track)
	for i := thumbTop; i < thumbTop+thumbHeight; i++ {
		marks[i] = true
	}
	return marks
}

// ListRow renders one row of a selectable list: an arrow marker on the cursor
// row plus an optional muted suffix pushed to the right edge.
func ListRow(width int, selected bool, label string, suffix string) string {
	marker := "  "
	style := lipgloss.NewStyle()
	if selected {
		marker = "➔ "
		style = style.Foreground(theme.Primary).Bold(true)
	}

	left := style.Render(marker + label)
	if suffix == "" {
		return lipgloss.NewStyle().MaxWidth(max(width, 1)).Render(left)
	}

	// Keep both halves inside the row: the suffix may take at most half of it,
	// and the label is trimmed to whatever is left. Without this the two run
	// together and then wrap onto the next line in narrow panels.
	right := lipgloss.NewStyle().
		Foreground(theme.Inactive).
		MaxWidth(max(width/2, 1)).
		Render(suffix)

	available := max(width-lipgloss.Width(right)-1, 1)
	left = lipgloss.NewStyle().MaxWidth(available).Render(left)

	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + strings.Repeat(" ", gap) + right
}

// StepHeading renders a wizard step heading. The step number carries the
// theme's accent, the title its primary colour, and the subtitle explains the
// step in the muted tone.
func StepHeading(step int, title string, subtitle string) string {
	number := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true).
		Render(fmt.Sprintf("Step %d", step))

	name := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Render(title)

	heading := number + lipgloss.NewStyle().Foreground(theme.Inactive).Render(" — ") + name
	if subtitle == "" {
		return heading
	}

	return heading + "\n" + lipgloss.NewStyle().Foreground(theme.Inactive).Render(subtitle)
}

// Body fits a panel's main content to exactly height rows: padded when it is
// short, clipped when it is long. Panels use it so whatever they pin below —
// a hint bar, a command prompt — is never pushed out of the frame.
func Body(content string, height int) string {
	height = max(height, 0)
	return lipgloss.NewStyle().Height(height).MaxHeight(height).Render(content)
}

// Hint is a single key/description pair shown in a hint bar.
type Hint struct {
	Key  string
	Desc string
}

// Hints renders the "[key] description • [key] description" bar that every
// interactive panel puts at the bottom of its content. Hints that would not fit
// in width are dropped rather than wrapped, so the bar always stays one line.
func Hints(width int, hints ...Hint) string {
	keyStyle := lipgloss.NewStyle().Foreground(theme.Accent)
	descStyle := lipgloss.NewStyle().Foreground(theme.Inactive)
	separator := descStyle.Render("  •  ")

	bar := ""
	for _, h := range hints {
		part := keyStyle.Render("["+h.Key+"]") + " " + descStyle.Render(h.Desc)
		candidate := part
		if bar != "" {
			candidate = bar + separator + part
		}

		if lipgloss.Width(candidate) > width {
			break
		}
		bar = candidate
	}

	return bar
}

// Error renders an error line in a consistently alarming colour.
func Error(err error) string {
	if err == nil {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555")).
		Bold(true).
		Render("Error: " + err.Error())
}
