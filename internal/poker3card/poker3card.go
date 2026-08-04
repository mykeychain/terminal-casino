// Package poker3card adapts the Three-Card Poker engine + UI to the neutral
// game.Game interface the casino lobby consumes. The engine stays clock-free;
// the time-based seed lives here at the UI/adapter layer (seeding is a caller
// concern), matching the blackjack32 adapter.
package poker3card

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/game"
	"github.com/mykeychain/terminal-casino/internal/poker3card/engine"
	"github.com/mykeychain/terminal-casino/internal/poker3card/ui"
)

// Game returns the poker3card adapter as a game.Game for the lobby registry.
func Game() game.Game { return adapter{} }

// adapter implements game.Game for Three-Card Poker.
type adapter struct{}

func (adapter) Title() string { return "3-Card Poker" }

func (adapter) Description() string {
	return "Three-card poker — Ante/Play vs. the dealer plus a Pair Plus side bet. Fresh $1000 bankroll."
}

// New builds a fresh Three-Card Poker model over a newly-seeded game and
// pre-delivers the current terminal size so the first frame renders at the
// right width.
func (adapter) New(width, height int) tea.Model {
	newGame := func() *engine.Game {
		return engine.NewGame(time.Now().UnixNano())
	}
	m := ui.New(newGame(), newGame)

	// Pre-seed the size so the model renders correctly on its first View,
	// before the program forwards its own WindowSizeMsg.
	seeded, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return seeded
}
