// Command casino runs the local casino TUI: a lobby / game-selector that launches
// the registered games (today: 3:2 Blackjack). Both this local binary and the SSH
// server land in the same lobby.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/mykeychain/terminal-casino/internal/blackjack32"
	"github.com/mykeychain/terminal-casino/internal/casino"
	"github.com/mykeychain/terminal-casino/internal/game"
)

func main() {
	// Pin the color profile once so styling is stable regardless of the local
	// terminal's advertised capabilities (locked decision 5: server-wide
	// TrueColor).
	lipgloss.SetColorProfile(termenv.TrueColor)

	games := []game.Game{
		blackjack32.Game(),
	}

	p := tea.NewProgram(casino.NewApp(games), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
