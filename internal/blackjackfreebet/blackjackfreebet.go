// Package blackjackfreebet adapts the Free Bet Blackjack engine + UI to the
// neutral game.Game interface the casino lobby consumes. The engine stays
// clock-free; the time-based seed lives here at the UI/adapter layer (seeding is
// a caller concern).
package blackjackfreebet

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/blackjackfreebet/engine"
	"github.com/mykeychain/terminal-casino/internal/blackjackfreebet/ui"
	"github.com/mykeychain/terminal-casino/internal/game"
)

// Game returns the blackjackfreebet adapter as a game.Game for the lobby registry.
func Game() game.Game { return adapter{} }

// adapter implements game.Game for Free Bet Blackjack.
type adapter struct{}

func (adapter) Title() string { return "Free Bet Blackjack" }

func (adapter) Description() string {
	return "6-deck blackjack — free doubles & splits, dealer pushes on 22. Up to 3 hands. Fresh $1000 bankroll."
}

// New builds a fresh Free Bet Blackjack model over a newly-seeded game and
// pre-delivers the current terminal size so the first frame renders at the right
// width.
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
