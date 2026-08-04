// Command casino runs the local casino TUI: a lobby / game-selector that launches
// the registered games (3:2 Blackjack and 3-Card Poker). Both this local binary
// and the SSH server land in the same lobby.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/blackjack32"
	"github.com/mykeychain/terminal-casino/internal/casino"
	"github.com/mykeychain/terminal-casino/internal/game"
	"github.com/mykeychain/terminal-casino/internal/poker3card"
	"github.com/mykeychain/terminal-casino/internal/theme"
)

func main() {
	// Pin the shared color profile — identical to the SSH server, so local and
	// remote render the same (see theme.Profile).
	theme.Apply()

	games := []game.Game{
		blackjack32.Game(),
		poker3card.Game(),
	}

	p := tea.NewProgram(casino.NewApp(games), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
