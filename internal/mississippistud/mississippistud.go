// Package mississippistud adapts the Mississippi Stud engine + UI to the neutral
// game.Game interface the casino lobby consumes. The engine stays clock-free; the
// time-based seed lives here at the UI/adapter layer (seeding is a caller
// concern), matching the poker3card and blackjack32 adapters.
package mississippistud

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/game"
	"github.com/mykeychain/terminal-casino/internal/mississippistud/engine"
	"github.com/mykeychain/terminal-casino/internal/mississippistud/ui"
)

// Game returns the mississippistud adapter as a game.Game for the lobby registry.
func Game() game.Game { return adapter{} }

// adapter implements game.Game for Mississippi Stud.
type adapter struct{}

func (adapter) Title() string { return "Mississippi Stud" }

func (adapter) Description() string {
	return "Mississippi Stud — 2 hole + 3 community cards; fold or raise 1×/2×/3× each street. Pays on your total wagered. Fresh $1000 bankroll."
}

// New builds a fresh Mississippi Stud model over a newly-seeded game and
// pre-delivers the current terminal size so the first frame renders at the right
// width.
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
