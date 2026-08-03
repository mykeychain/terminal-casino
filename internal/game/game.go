// Package game defines the neutral Game abstraction the casino lobby launches.
// It imports only Bubble Tea so it can be depended on by both the lobby and each
// concrete game adapter without creating an import cycle.
package game

import tea "github.com/charmbracelet/bubbletea"

// Game is a launchable casino game. The lobby lists games by Title/Description
// and, on selection, calls New with the current terminal size to obtain a fresh
// Bubble Tea model for that game session.
type Game interface {
	// Title is the game's short display name (e.g. "3:2 Blackjack").
	Title() string
	// Description is a one-line summary shown under the title in the lobby.
	Description() string
	// New builds a fresh game model pre-seeded with the given terminal size.
	New(width, height int) tea.Model
}
