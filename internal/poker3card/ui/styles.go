package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/theme"
)

// Card geometry, card styles, the titled-panel chrome, and the menu styles live
// in internal/tui (shared across games). This file keeps only the styles that are
// specific to the Three-Card Poker table.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.BrightGold)

	areaLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Dim)

	// focusedTileStyle / plainTileStyle draw the betting-screen wager tiles: the
	// focused wager gets a gold rounded border (and shows ◀ $ ▶); the other sits
	// in a plain soft-white rounded border. Same padding keeps the row from
	// jumping when focus moves.
	focusedTileStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.Gold).
				Padding(0, 1)

	plainTileStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.SoftWhite).
			Padding(0, 1)

	dimStyle = lipgloss.NewStyle().Foreground(theme.Dim)

	winStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Green)
	loseStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.Red)
	pushStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.Neutral)
	bonusStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.BrightGold)

	moneyStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Green)
	betStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Gold)
	nameStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.SoftWhite)
)
