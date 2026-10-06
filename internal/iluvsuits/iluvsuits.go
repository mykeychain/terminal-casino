// Package iluvsuits adapts the I Luv Suits engine + UI to the neutral game.Game
// interface the casino lobby consumes. The engine stays clock-free; the
// time-based seed lives here at the UI/adapter layer (seeding is a caller
// concern), matching the other game adapters.
package iluvsuits

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/game"
	"github.com/mykeychain/terminal-casino/internal/iluvsuits/engine"
	"github.com/mykeychain/terminal-casino/internal/iluvsuits/ui"
)

// Game returns the iluvsuits adapter as a game.Game for the lobby registry.
func Game() game.Game { return adapter{} }

// adapter implements game.Game for I Luv Suits.
type adapter struct{}

func (adapter) Title() string { return "I Luv Suits" }

func (adapter) Description() string {
	return "I Luv Suits — 7 cards each, the longest flush wins; Play up to 3× on a big flush. Fresh $1000 bankroll."
}

// New builds a fresh I Luv Suits model over a newly-seeded game and pre-delivers
// the current terminal size so the first frame renders at the right width.
func (adapter) New(width, height int) tea.Model {
	newGame := func() *engine.Game {
		return engine.NewGame(time.Now().UnixNano())
	}
	m := ui.New(newGame(), newGame)

	// Pre-seed the size so the model renders correctly on its first View, before
	// the program forwards its own WindowSizeMsg.
	seeded, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return seeded
}
