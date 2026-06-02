package styles

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium-refactor/tui/internal/tui/theme"
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

func Container(
	width int,
	height int,
	isFocus bool,
	title string,
	content string,
	isHelp bool,
) string {
	var borderColor color.Color
	if isFocus {
		borderColor = theme.Primary
	} else {
		borderColor = theme.Inactive
	}

	b := lipgloss.NormalBorder()
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	titleText := borderStyle.Render(" " + title + " ")
	topLineLen := max(width-2-lipgloss.Width(titleText), 0)

	topLine := borderStyle.Render(b.TopLeft) +
		titleText +
		borderStyle.Render(strings.Repeat(b.Top, topLineLen)+b.TopRight)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, !isHelp, true).
		BorderForeground(borderColor)

	container := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 2).
		Height(height - 2).
		Render(content)

	mainBox := boxStyle.Render(container)

	if isHelp {
		helpText := lipgloss.NewStyle().Foreground(theme.Accent).Render(" [?] help ")
		bottomLineLen := max(width-2-lipgloss.Width(helpText), 0)

		bottomLine := borderStyle.Render(b.BottomLeft+strings.Repeat(b.Bottom, bottomLineLen)) +
			helpText +
			borderStyle.Render(b.BottomRight)

		return lipgloss.JoinVertical(0, topLine, mainBox, bottomLine)
	}

	return lipgloss.JoinVertical(0, topLine, mainBox)
}
