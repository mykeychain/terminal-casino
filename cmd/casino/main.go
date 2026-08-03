// Command 32blackjack runs the local, single-player 3:2 Blackjack TUI.
package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/blackjack32/engine"
	"github.com/mykeychain/terminal-casino/internal/blackjack32/ui"
)

func main() {
	// The UI/main is the only place a time-based seed is allowed (locked
	// decision 5): the engine itself never reads the clock. This closure also
	// re-seeds a fresh game on restart after game over.
	newGame := func() *engine.Game {
		return engine.NewGame(time.Now().UnixNano())
	}

	model := ui.New(newGame(), newGame)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
