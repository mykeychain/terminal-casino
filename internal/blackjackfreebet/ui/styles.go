package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/theme"
)

// Card geometry, card styles, the titled-panel chrome, and the menu styles live
// in internal/tui (shared across games). This file keeps only the styles that are
// specific to the 3:2 Blackjack table, all sourced from the shared palette.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.BrightGold)

	areaLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Dim)

	// activeHandStyle frames the hand the player is currently acting on (gold).
	activeHandStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.Gold).
			Padding(0, 1)

	// inactiveHandStyle keeps identical spacing to activeHandStyle (invisible
	// border) so hands do not jump when the active one changes.
	inactiveHandStyle = lipgloss.NewStyle().
				Border(lipgloss.HiddenBorder()).
				Padding(0, 1)

	// focusedTileStyle / plainTileStyle draw the betting-screen bet tiles: the
	// focused spot gets a gold rounded border (and shows ◀ $ ▶); the rest sit in a
	// plain soft-white rounded border. Same padding keeps the row from jumping.
	focusedTileStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.Gold).
				Padding(0, 1)

	plainTileStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.SoftWhite).
			Padding(0, 1)

	keyHintStyle = lipgloss.NewStyle().Foreground(theme.Dim)
	dimStyle     = lipgloss.NewStyle().Foreground(theme.Dim)

	winStyle       = lipgloss.NewStyle().Bold(true).Foreground(theme.Green)
	loseStyle      = lipgloss.NewStyle().Bold(true).Foreground(theme.Red)
	pushStyle      = lipgloss.NewStyle().Bold(true).Foreground(theme.Neutral)
	blackjackStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.BrightGold)

	moneyStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Green)
	betStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Gold)

	// freeBetStyle marks a house-funded free bet (from a free double or split):
	// green, to read as bonus money at no risk, and distinct from the gold real bet.
	freeBetStyle = lipgloss.NewStyle().Foreground(theme.Green)

	// insuranceCostStyle highlights the insurance cost in the prompt (gold pill:
	// gold background, dark text) so the money on the line stands out.
	insuranceCostStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.MenuFg).Background(theme.Gold)
)
