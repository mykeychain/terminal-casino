package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/theme"
)

// Card geometry. Every card (face-up or face-down) occupies the same footprint
// so hands line up and "10" never shifts a border. These mirror the blackjack
// table's card sizing (the repo accepts this per-game duplication).
const (
	cardInnerWidth = 5 // characters between the vertical borders
	rankFieldWidth = 2 // reserved field so "10" and "A" both occupy 2 cells
	cardHeight     = 5 // rendered card height (full density): 3 body rows + 2 border rows

	// compactCardHeight is the rendered height of a compact (height-density) card:
	// a single body row (rank+suit) between the two border rows. Halving the card
	// row height is the main lever that lets the table fit short terminals.
	compactCardHeight = 3

	// handIndent is how far the free-standing rows (the result banner, the
	// settlement breakdown) sit from the left so they align with the card rows.
	handIndent = "  "
)

// Card styles. Every card shares the same soft-white rounded border; only the
// pip (rank + suit glyph) color differs by suit. The face-down back keeps the
// same border and fills with a slate hatch.
var (
	cardRedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.SoftWhite).
			Foreground(theme.Red)

	cardDefaultStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.SoftWhite).
				Foreground(theme.SoftWhite)

	// cardBackStyle renders a face-down card with a hatch fill.
	cardBackStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.SoftWhite).
			Foreground(theme.Slate)
)

// Layout / chrome styles.
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

	dimStyle = lipgloss.NewStyle().Foreground(theme.Dim)

	winStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Green)
	loseStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.Red)
	pushStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.Neutral)
	bonusStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.BrightGold)

	moneyStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Green)
	betStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Gold)
	nameStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.SoftWhite)
)
