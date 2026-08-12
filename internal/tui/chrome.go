package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/theme"
)

// Chrome styles, sourced from the shared palette.
var (
	// panelBorderStyle / panelTitleStyle draw the rounded action panel and its
	// embedded title label.
	panelBorderStyle = lipgloss.NewStyle().Foreground(theme.SoftWhite)
	panelTitleStyle  = lipgloss.NewStyle().Foreground(theme.Dim)

	// menuSelectedStyle / menuUnselectedStyle render the arrow-navigation menu:
	// the highlighted item is gold with a dark foreground; the rest are soft white.
	menuSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.MenuFg).
				Background(theme.Gold).
				Padding(0, 1)

	menuUnselectedStyle = lipgloss.NewStyle().
				Foreground(theme.SoftWhite).
				Padding(0, 1)

	// menuAccentStyle renders an unselected menu item that a game wants to mark as
	// special — green, the shared "bonus / on the house" accent. The gold selection
	// pill still wins when the item is the selected one.
	menuAccentStyle = lipgloss.NewStyle().
			Foreground(theme.Green).
			Padding(0, 1)
)

// TitledBox draws a rounded panel around content with a label embedded in the
// top border, using the default soft-white border and dim title. It is the
// default-styled shorthand for TitledBoxWith.
func TitledBox(title, content string) string {
	return TitledBoxWith(title, content, panelBorderStyle, panelTitleStyle)
}

// TitledBoxWith is TitledBox with caller-chosen border and title styles, so a
// panel can be tinted per state — e.g. a gold border and bright title for the
// selected lobby tile, soft-white and dim for the rest. Content may contain ANSI
// styling and span multiple lines; widths are measured with lipgloss.Width so
// styled menu pills still align.
func TitledBoxWith(title, content string, border, titleStyle lipgloss.Style) string {
	lines := strings.Split(content, "\n")
	inner := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > inner {
			inner = w
		}
	}
	titleW := lipgloss.Width(title)
	if need := titleW + 2; need > inner { // keep the title from overflowing the top border
		inner = need
	}
	interior := inner + 2 // one space of padding on each side
	dashes := interior - (titleW + 3)
	if dashes < 0 {
		dashes = 0
	}

	var sb strings.Builder
	sb.WriteString(border.Render("╭─ ") + titleStyle.Render(title) +
		border.Render(" "+strings.Repeat("─", dashes)+"╮"))
	sb.WriteString("\n")
	for _, l := range lines {
		pad := inner - lipgloss.Width(l)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(border.Render("│ ") + l + strings.Repeat(" ", pad) +
			border.Render(" │") + "\n")
	}
	sb.WriteString(border.Render("╰" + strings.Repeat("─", interior) + "╯"))
	return sb.String()
}

// RenderMenu lays a set of labels out horizontally, highlighting the selected
// one in gold.
func RenderMenu(labels []string, selected int) string {
	return RenderMenuAccented(labels, selected, nil)
}

// RenderMenuAccented is RenderMenu with a per-item accent flag. An accented item
// that is not the selected one is rendered in the green accent (marking it as
// special, e.g. a free/house-funded action) instead of plain soft white; the
// selected item always takes the gold selection pill regardless of its accent.
// accent may be shorter than labels (or nil) — missing entries are treated false.
func RenderMenuAccented(labels []string, selected int, accent []bool) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		switch {
		case i == selected:
			parts[i] = menuSelectedStyle.Render(l)
		case i < len(accent) && accent[i]:
			parts[i] = menuAccentStyle.Render(l)
		default:
			parts[i] = menuUnselectedStyle.Render(l)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
