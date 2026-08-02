package ui

import "github.com/charmbracelet/lipgloss"

// Palette (approved v1.1 colors). Each named var maps to a single UI role so the
// whole scheme lives in one place. Values are true-color hex; lipgloss degrades
// them on limited terminals.
var (
	colorBorder   = lipgloss.Color("#C9CDD2") // card border, all suits (soft white)
	colorPipBlack = lipgloss.Color("#E8EAED") // rank + glyph for ♠ ♣
	colorPipRed   = lipgloss.Color("#D64545") // rank + glyph for ♥ ♦ (also loss text)
	colorBack     = lipgloss.Color("#64748B") // card-back hatch (slate)
	colorFocus    = lipgloss.Color("#E0A82E") // focus: active-hand frame, selected menu, bet value (gold)
	colorMenuFg   = lipgloss.Color("#10151C") // dark foreground behind a selected (gold) menu item
	colorGreen    = lipgloss.Color("#4FB477") // bankroll value & win text
	colorNeutral  = lipgloss.Color("#9AA0A6") // push / neutral (grey)
	colorGold     = lipgloss.Color("#F2C14E") // Blackjack! callout & title (bright gold)
	colorDim      = lipgloss.Color("#8B949E") // dim / secondary text
)

// Card geometry. Every card (face-up or face-down) occupies the same footprint
// so hands line up and "10" never shifts a border.
const (
	cardInnerWidth = 5 // characters between the vertical borders
	rankFieldWidth = 2 // reserved field so "10" and "A" both occupy 2 cells
)

// Card styles. Every card shares the same soft-white rounded border; only the
// pip (rank + suit glyph) color differs by suit. The face-down back keeps the
// same border and fills with a slate hatch.
var (
	cardRedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Foreground(colorPipRed)

	cardDefaultStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Foreground(colorPipBlack)

	// cardBackStyle renders the dealer hole card face-down with a hatch fill.
	cardBackStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Foreground(colorBack)
)

// Layout / chrome styles.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGold)

	areaLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorDim)

	// activeHandStyle frames the hand the player is currently acting on (gold).
	activeHandStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorFocus).
			Padding(0, 1)

	// inactiveHandStyle keeps identical spacing to activeHandStyle (invisible
	// border) so hands do not jump when the active one changes.
	inactiveHandStyle = lipgloss.NewStyle().
				Border(lipgloss.HiddenBorder()).
				Padding(0, 1)

	// panelBorderStyle / panelTitleStyle draw the rounded action panel and its
	// embedded title label.
	panelBorderStyle = lipgloss.NewStyle().Foreground(colorBorder)
	panelTitleStyle  = lipgloss.NewStyle().Foreground(colorDim)

	// menuSelectedStyle / menuUnselectedStyle render the arrow-navigation menu:
	// the highlighted item is gold with a dark foreground; the rest are soft white.
	menuSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorMenuFg).
				Background(colorFocus).
				Padding(0, 1)

	menuUnselectedStyle = lipgloss.NewStyle().
				Foreground(colorBorder).
				Padding(0, 1)

	keyHintStyle = lipgloss.NewStyle().Foreground(colorDim)
	dimStyle     = lipgloss.NewStyle().Foreground(colorDim)

	winStyle       = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	loseStyle      = lipgloss.NewStyle().Bold(true).Foreground(colorPipRed)
	pushStyle      = lipgloss.NewStyle().Bold(true).Foreground(colorNeutral)
	blackjackStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGold)

	moneyStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	betStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorFocus)
)
