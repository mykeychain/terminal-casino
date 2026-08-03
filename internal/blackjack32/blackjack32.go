// Package blackjack32 adapts the existing 3:2 Blackjack engine + UI to the
// neutral game.Game interface the casino lobby consumes. The engine stays
// clock-free; the time-based seed lives here at the UI/adapter layer (locked
// decision: seeding is a caller concern).
package blackjack32

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/blackjack32/engine"
	"github.com/mykeychain/terminal-casino/internal/blackjack32/ui"
	"github.com/mykeychain/terminal-casino/internal/game"
)

// Game returns the blackjack32 adapter as a game.Game for the lobby registry.
func Game() game.Game { return adapter{} }

// adapter implements game.Game for 3:2 Blackjack.
type adapter struct{}

func (adapter) Title() string { return "3:2 Blackjack" }

func (adapter) Description() string {
	return "6-deck 3:2 blackjack — up to 3 hands, split, double, insurance. Fresh $1000 bankroll."
}

// New builds a fresh blackjack model over a newly-seeded game and pre-delivers
// the current terminal size so the first frame renders at the right width.
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
