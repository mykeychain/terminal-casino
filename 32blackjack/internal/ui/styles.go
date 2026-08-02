package ui

import "github.com/charmbracelet/lipgloss"

// Palette. Colors are chosen to read well on both dark and light terminals; the
// suit colors mirror real cards (red hearts/diamonds, default spades/clubs).
var (
	colorRed     = lipgloss.Color("1")   // hearts/diamonds
	colorGreen   = lipgloss.Color("2")   // wins / positive money
	colorYellow  = lipgloss.Color("3")   // bets / prompts
	colorBlue    = lipgloss.Color("4")   // card-back pattern
	colorMagenta = lipgloss.Color("5")   // active-hand highlight
	colorGray    = lipgloss.Color("240") // muted / dividers
	colorWhite   = lipgloss.Color("15")  // default card face text
)

// Card geometry. Every card (face-up or face-down) occupies the same footprint
// so hands line up and "10" never shifts a border.
const (
	cardInnerWidth = 5 // characters between the vertical borders
	rankFieldWidth = 2 // reserved field so "10" and "A" both occupy 2 cells
)

// Card styles. The border is applied by lipgloss; face color is set per-suit.
var (
	cardRedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRed).
			Foreground(colorRed)

	cardDefaultStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorGray).
				Foreground(colorWhite)

	// cardBackStyle renders the dealer hole card face-down with a hatch fill.
	cardBackStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBlue).
			Foreground(colorBlue)
)

// Layout / chrome styles.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorYellow)

	areaLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGray)

	// activeHandStyle frames the hand the player is currently acting on.
	activeHandStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorMagenta).
			Padding(0, 1)

	// inactiveHandStyle keeps identical spacing to activeHandStyle (invisible
	// border) so hands do not jump when the active one changes.
	inactiveHandStyle = lipgloss.NewStyle().
				Border(lipgloss.HiddenBorder()).
				Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	actionBarStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Padding(0, 1)

	keyHintStyle = lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
	dimStyle     = lipgloss.NewStyle().Foreground(colorGray)

	winStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	loseStyle = lipgloss.NewStyle().Bold(true).Foreground(colorRed)
	pushStyle = lipgloss.NewStyle().Bold(true).Foreground(colorYellow)

	moneyStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	betStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
)
