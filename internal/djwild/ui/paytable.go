package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/djwild/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// The paytable overlay is a reference the player can pull up at any time with
// `?` (it toggles). Its numbers are read straight from the engine's exported
// pay tables, so the display can never drift from what the engine pays.

// paytableOrder lists every category that can pay, best hand first, for a
// stable top-to-bottom column order.
var paytableOrder = []engine.HandCategory{
	engine.FiveWilds,
	engine.RoyalFlush,
	engine.FiveOfAKind,
	engine.StraightFlush,
	engine.FourOfAKind,
	engine.FullHouse,
	engine.Flush,
	engine.Straight,
	engine.ThreeOfAKind,
}

// viewPaytable renders the paytable as a full anchored frame: the usual title +
// header on top, the centered paytable panel in the middle, and a close hint
// pinned to the bottom — sized to exactly m.height × m.width like the table view.
func (m Model) viewPaytable(f revealFrame) string {
	top := m.renderTop(f)
	panel := m.renderPaytable()
	hint := " " + dimStyle.Render("? close · q lobby · Q quit")

	if m.width == 0 || m.height == 0 {
		return top + "\n\n" + panel + "\n\n" + hint
	}

	middleH := m.height - lipgloss.Height(top) - lipgloss.Height(hint)
	if middleH < 0 {
		middleH = 0
	}
	middle := lipgloss.Place(m.width, middleH, lipgloss.Center, lipgloss.Center, panel)

	body := lipgloss.JoinVertical(lipgloss.Left, top, middle, hint)
	body = tui.FitHeight(body, m.height)
	return lipgloss.NewStyle().Width(m.width).Render(body)
}

// renderPaytable builds the reference panel: the Blind table, then the Trips
// table with its natural and wild columns, then the rules in brief.
func (m Model) renderPaytable() string {
	header := fmt.Sprintf("%-16s  %6s  %13s  %10s", "Hand", "Blind", "Trips natural", "Trips wild")
	rows := []string{betStyle.Render(header)}
	for _, c := range paytableOrder {
		rows = append(rows, fmt.Sprintf("%-16s  %s  %s  %s", c.String(),
			cell(engine.BlindPayouts, c, 6, "push"),
			cell(engine.TripsNaturalPayouts, c, 13, "—"),
			cell(engine.TripsWildPayouts, c, 10, "—")))
	}

	notes := lipgloss.JoinVertical(lipgloss.Left,
		dimStyle.Render("Deuces and the joker are wild. Ante = Blind; Play is 2× the Ante."),
		dimStyle.Render("The dealer always plays. Beat the dealer: Ante and Play pay 1:1,"),
		dimStyle.Render("the Blind pays its table (pushes below a Straight). Ties push all."),
		dimStyle.Render("Natural and wild hands of the same rank tie each other."),
		dimStyle.Render("Trips pays on your five cards alone, even if you fold."),
	)
	return tui.TitledBox("Paytable", lipgloss.JoinVertical(lipgloss.Left, rows...)+"\n\n"+notes)
}

// cell renders one pay-table entry right-aligned to width: "N:1" in the bonus
// accent when the category pays, otherwise the dim fallback.
func cell(table map[engine.HandCategory]int, c engine.HandCategory, width int, fallback string) string {
	if m, ok := table[c]; ok {
		return bonusStyle.Render(fmt.Sprintf("%*s", width, fmt.Sprintf("%d:1", m)))
	}
	return dimStyle.Render(fmt.Sprintf("%*s", width, fallback))
}
