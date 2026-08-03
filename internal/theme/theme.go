// Package theme holds the shared casino color palette as exported lipgloss
// colors. The values mirror the approved blackjack palette (see
// internal/blackjack32/ui/styles.go); the lobby and future shared chrome use
// them. blackjack32's own styles.go intentionally keeps its private copy for
// now, so a little duplication is expected.
package theme

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Profile is the single color profile every front end pins, so rendering is
// identical whether the game runs locally or over SSH (which makes debugging
// straightforward). ANSI256 is universally supported — including macOS
// Terminal.app, which mis-renders 24-bit truecolor — and is the reliable choice
// over SSH, where the truecolor hint is usually not forwarded. Lip Gloss
// downsamples the hex palette below to the nearest 256-color for every client.
const Profile = termenv.ANSI256

// Apply pins the shared color profile. Call once at program startup, before any
// styles are rendered, from every entry point (local and SSH).
func Apply() { lipgloss.SetColorProfile(Profile) }

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
