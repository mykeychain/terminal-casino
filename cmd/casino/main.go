// Command casino runs the local casino TUI: a lobby / game-selector that launches
// the registered games (today: 3:2 Blackjack). Both this local binary and the SSH
// server land in the same lobby.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/blackjack32"
	"github.com/mykeychain/terminal-casino/internal/casino"
	"github.com/mykeychain/terminal-casino/internal/game"
)

func main() {
	// Locally, let Lip Gloss auto-detect the terminal's color capability from its
	// stdout. Forcing TrueColor breaks terminals that only do 256 colors (e.g.
	// macOS Terminal.app renders 24-bit sequences as garbage); auto-detect
	// downsamples the palette correctly for whatever terminal is attached.

	games := []game.Game{
		blackjack32.Game(),
	}

	p := tea.NewProgram(casino.NewApp(games), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
