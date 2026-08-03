// Package theme holds the shared casino color palette as exported lipgloss
// colors. The values mirror the approved blackjack palette (see
// internal/blackjack32/ui/styles.go); the lobby and future shared chrome use
// them. blackjack32's own styles.go intentionally keeps its private copy for
// now, so a little duplication is expected.
package theme

import "github.com/charmbracelet/lipgloss"

// Palette (true-color hex; lipgloss degrades on limited terminals).
var (
	// Gold — focus color: selected menu item, highlighted cursor, bet value.
	Gold = lipgloss.Color("#E0A82E")
	// BrightGold — title / callout accent.
	BrightGold = lipgloss.Color("#F2C14E")
	// Green — positive values (bankroll, wins).
	Green = lipgloss.Color("#4FB477")
	// Red — negative values / losses.
	Red = lipgloss.Color("#D64545")
	// SoftWhite — primary borders and body text.
	SoftWhite = lipgloss.Color("#C9CDD2")
	// Dim — secondary / hint text.
	Dim = lipgloss.Color("#8B949E")
	// Slate — muted fills (e.g. card-back hatch).
	Slate = lipgloss.Color("#64748B")
	// Neutral — push / neutral grey.
	Neutral = lipgloss.Color("#9AA0A6")
	// MenuFg — dark foreground behind a selected (gold) menu item.
	MenuFg = lipgloss.Color("#10151C")
)
