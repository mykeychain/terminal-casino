package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/iluvsuits/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// The paytable overlay is a reference the player can pull up at any time with
// `?` (it toggles). Its numbers are read straight from the engine's exported
// pay tables, so the display can never drift from what the engine pays.

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

// renderPaytable builds the reference panel: the two side-bet tables side by
// side, then the main-game rules (Play limits and dealer qualification).
func (m Model) renderPaytable() string {
	flushRush := lengthColumn("Flush Rush", "%d-card flush", engine.FlushRushPayouts, 4)
	superRush := lengthColumn("Super Flush Rush", "%d-card str. flush", engine.SuperFlushRushPayouts, 3)
	cols := lipgloss.JoinHorizontal(lipgloss.Top, flushRush, "     ", superRush)

	rules := lipgloss.JoinVertical(lipgloss.Left,
		betStyle.Render("Play wager"),
		fmt.Sprintf("  %-20s %s", "6- or 7-card flush", bonusStyle.Render("up to 3× Ante")),
		fmt.Sprintf("  %-20s %s", "5-card flush", bonusStyle.Render("up to 2× Ante")),
		fmt.Sprintf("  %-20s %s", "4 cards or fewer", bonusStyle.Render("1× Ante")),
	)
	notes := lipgloss.JoinVertical(lipgloss.Left,
		dimStyle.Render("More cards in one suit wins; equal lengths compare high cards."),
		dimStyle.Render(fmt.Sprintf("Dealer qualifies with a 3-card %s-high flush or better;", engine.QualifyHigh.Name())),
		dimStyle.Render("if not, the Ante pays 1:1 and the Play is returned."),
		dimStyle.Render("Side bets pay on your seven cards alone, even if you fold."),
	)
	return tui.TitledBox("Paytable", cols+"\n\n"+rules+"\n\n"+notes)
}

// lengthColumn renders one side-bet column: its heading, then each paying length
// from 7 down to min with its "N:1" multiplier pulled from table.
func lengthColumn(heading, rowFormat string, table map[int]int, min int) string {
	rows := []string{betStyle.Render(heading)}
	for n := engine.HandSize; n >= min; n-- {
		mult, ok := table[n]
		if !ok {
			continue
		}
		name := fmt.Sprintf("  %-19s", fmt.Sprintf(rowFormat, n))
		rows = append(rows, name+" "+bonusStyle.Render(fmt.Sprintf("%d:1", mult)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
