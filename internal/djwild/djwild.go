// Package djwild adapts the DJ Wild engine + UI to the neutral game.Game
// interface the casino lobby consumes. The engine stays clock-free; the
// time-based seed lives here at the UI/adapter layer (seeding is a caller
// concern), matching the other game adapters.
package djwild

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/djwild/engine"
	"github.com/mykeychain/terminal-casino/internal/djwild/ui"
	"github.com/mykeychain/terminal-casino/internal/game"
)

// Game returns the djwild adapter as a game.Game for the lobby registry.
func Game() game.Game { return adapter{} }

// adapter implements game.Game for DJ Wild.
type adapter struct{}

func (adapter) Title() string { return "DJ Wild" }

func (adapter) Description() string {
	return "DJ Wild — 5-card stud with deuces & a joker wild. Ante + Blind, then fold or Play 2×. Fresh $1000 bankroll."
}

// New builds a fresh DJ Wild model over a newly-seeded game and pre-delivers the
// current terminal size so the first frame renders at the right width.
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
