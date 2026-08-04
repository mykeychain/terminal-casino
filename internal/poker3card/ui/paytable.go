package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/poker3card/engine"
)

// The paytable overlay is a reference the player can pull up at any time with
// `?` (it toggles). Its numbers are read straight from the engine's exported
// pay tables, so the display can never drift from what the engine actually pays.

// paytableOrder lists the categories that earn a payout, best hand first, for a
// stable top-to-bottom column order.
var paytableOrder = []engine.HandCategory{
	engine.StraightFlush,
	engine.ThreeOfAKind,
	engine.Straight,
	engine.Flush,
	engine.Pair,
}

// viewPaytable renders the paytable as a full anchored frame: the usual title +
// header on top, the centered paytable panel in the middle, and a close hint
// pinned to the bottom — sized to exactly m.height × m.width like the table view.
func (m Model) viewPaytable(f revealFrame) string {
	top := m.renderTop(f)
	panel := m.renderPaytable()
	hint := " " + dimStyle.Render("? close · q quit")

	if m.width == 0 || m.height == 0 {
		return top + "\n\n" + panel + "\n\n" + hint
	}

	topH := lipgloss.Height(top)
	bottomH := lipgloss.Height(hint)
	middleH := m.height - topH - bottomH
	if middleH < 0 {
		middleH = 0
	}
	middle := lipgloss.Place(m.width, middleH, lipgloss.Center, lipgloss.Center, panel)

	body := lipgloss.JoinVertical(lipgloss.Left, top, middle, hint)
	body = fitHeight(body, m.height)
	return lipgloss.NewStyle().Width(m.width).Render(body)
}

// renderPaytable builds the two-column paytable panel (Pair Plus side bet and
// the Ante Bonus), wrapped in the same titled rounded box the action bar uses.
func (m Model) renderPaytable() string {
	pairPlus := paytableColumn("Pair Plus (side bet)", engine.PairPlusMultipliers)
	anteBonus := paytableColumn("Ante Bonus (on your Ante)", engine.AnteBonusMultipliers)

	cols := lipgloss.JoinHorizontal(lipgloss.Top, pairPlus, "     ", anteBonus)
	notes := lipgloss.JoinVertical(lipgloss.Left,
		dimStyle.Render("Pair Plus pays on your three cards alone — win or lose the hand."),
		dimStyle.Render("The Ante Bonus pays these strong hands even if the dealer beats you."),
	)
	return titledBox("Paytable", cols+"\n\n"+notes)
}

// paytableColumn renders one titled payout column: its heading, then each paying
// category (best first) with its "N:1" multiplier pulled from table. Categories
// absent from table are skipped.
func paytableColumn(heading string, table map[engine.HandCategory]int) string {
	rows := []string{betStyle.Render(heading)}
	for _, c := range paytableOrder {
		mult, ok := table[c]
		if !ok {
			continue
		}
		name := fmt.Sprintf("  %-16s", c.String())
		rows = append(rows, name+" "+bonusStyle.Render(fmt.Sprintf("%d:1", mult)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
