package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/mississippistud/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// The paytable overlay is a reference the player can pull up at any time with
// `?` (it toggles). Its numbers are read straight from the engine's exported
// pay table, so the display can never drift from what the engine actually pays —
// including a regional variant that overrides a multiplier.

// paytableRow is one line of the flat pay table: a category and the exported
// payout var that resolves its multiplier.
type paytableRow struct {
	label string
	cat   engine.HandCategory
}

// paytableOrder lists the flat-paying categories, best hand first, for a stable
// top-to-bottom column order. Royal Flush pays distinctly from a lower Straight
// Flush, so both appear.
var paytableOrder = []paytableRow{
	{"Royal Flush", engine.RoyalFlush},
	{"Straight Flush", engine.StraightFlush},
	{"Four of a Kind", engine.FourOfAKind},
	{"Full House", engine.FullHouse},
	{"Flush", engine.Flush},
	{"Straight", engine.Straight},
	{"Three of a Kind", engine.ThreeOfAKind},
	{"Two Pair", engine.TwoPair},
}

// viewPaytable renders the paytable as a full anchored frame: the usual title +
// header on top, the centered paytable panel in the middle, and a close hint
// pinned to the bottom — sized to exactly m.height × m.width like the table view.
func (m Model) viewPaytable() string {
	top := m.renderTop()
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
	body = tui.FitHeight(body, m.height)
	return lipgloss.NewStyle().Width(m.width).Render(body)
}

// renderPaytable builds the pay-table panel: the flat "X:1" payouts (best first)
// followed by the pair tiers, all read from the live engine vars and wrapped in
// the same titled rounded box the action bar uses. The payouts apply to the TOTAL
// amount wagered — the point that distinguishes Mississippi Stud — so the note
// says so.
func (m Model) renderPaytable() string {
	rows := []string{betStyle.Render("Pays on your total amount wagered")}
	for _, r := range paytableOrder {
		mult := engine.HandPayout(r.cat)
		name := fmt.Sprintf("  %-16s", r.label)
		rows = append(rows, name+" "+nameStyle.Render(fmt.Sprintf("%d:1", mult)))
	}

	// Pair tiers: a single pair does not pay flat; it is resolved by rank.
	rows = append(rows, "")
	rows = append(rows, betStyle.Render("Pairs"))
	rows = append(rows,
		fmt.Sprintf("  %-16s %s", "Jacks or better", winStyle.Render(fmt.Sprintf("%d:1 win", engine.PairWinPayout))),
		fmt.Sprintf("  %-16s %s", "Sixes – Tens", pushStyle.Render("push")),
		fmt.Sprintf("  %-16s %s", "Fives or lower", loseStyle.Render("loss")),
	)

	panel := lipgloss.JoinVertical(lipgloss.Left, rows...)
	note := dimStyle.Render("2 hole + 3 community · fold or raise 1×/2×/3× each street.")
	return tui.TitledBox("Paytable", panel+"\n\n"+note)
}
