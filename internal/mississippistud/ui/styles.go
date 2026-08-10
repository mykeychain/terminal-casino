package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/theme"
)

// Card geometry, card styles, the titled-panel chrome, and the menu styles live
// in internal/tui (shared across games). This file keeps only the styles that are
// specific to the Mississippi Stud table.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.BrightGold)

	areaLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Dim)

	// focusedTileStyle draws the betting-screen Ante tile in a gold rounded border
	// (it shows ◀ $N ▶). Mississippi Stud has a single, mandatory wager, so there
	// is only ever one tile and it is always the focused one.
	focusedTileStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.Gold).
				Padding(0, 1)

	dimStyle = lipgloss.NewStyle().Foreground(theme.Dim)

	winStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.Green)
	loseStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Red)
	pushStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Neutral)

	moneyStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Green)
	betStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Gold)
	nameStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.SoftWhite)

	// riskStyle highlights the running total at risk — the game's core tension, so
	// it gets the bright callout accent to stand out from the plainer ledger cells.
	riskStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.BrightGold)
)
