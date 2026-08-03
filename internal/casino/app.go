// Package casino is the game-selector lobby. NewApp returns a Bubble Tea model
// with two states — Lobby and Playing — that lists the registered games, launches
// the selected one, and returns to the lobby on Esc. It embeds each game's model
// while Playing and delegates Update/View to it; the games themselves are unaware
// of the lobby.
package casino

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/game"
	"github.com/mykeychain/terminal-casino/internal/theme"
)

// state is the App's top-level mode.
type state int

const (
	stateLobby state = iota
	statePlaying
)

// App is the lobby Bubble Tea model.
type App struct {
	games  []game.Game
	cursor int
	state  state

	// active is the running game model while state == statePlaying; nil in lobby.
	active tea.Model

	width  int
	height int
}

// NewApp builds the lobby App over the given game registry. An empty registry is
// handled gracefully (the lobby renders an empty-state message and launches
// nothing).
func NewApp(games []game.Game) tea.Model {
	return App{games: games, state: stateLobby}
}

// Init implements tea.Model. The lobby needs no startup command.
func (a App) Init() tea.Cmd { return nil }

// Update implements tea.Model. Global quit keys are handled first; then Esc
// (while Playing) returns to the lobby; otherwise input is routed by state.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// While playing, the active model also needs the new size.
		if a.state == statePlaying && a.active != nil {
			var cmd tea.Cmd
			a.active, cmd = a.active.Update(msg)
			return a, cmd
		}
		return a, nil

	case tea.KeyMsg:
		key := msg.String()
		// Global quit from anywhere.
		if key == "q" || key == "ctrl+c" {
			return a, tea.Quit
		}
		if a.state == statePlaying {
			// Esc is intercepted at the App level: drop the game model and
			// return to the lobby. The game model never sees Esc.
			if key == "esc" {
				a.state = stateLobby
				a.active = nil
				return a, nil
			}
			return a.updatePlaying(msg)
		}
		return a.updateLobby(msg)
	}

	// Non-key, non-size messages (e.g. tick/async cmds) belong to the active game.
	if a.state == statePlaying && a.active != nil {
		var cmd tea.Cmd
		a.active, cmd = a.active.Update(msg)
		return a, cmd
	}
	return a, nil
}

// updateLobby handles cursor movement and launching in the Lobby state.
func (a App) updateLobby(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}
	case "down", "j":
		if a.cursor < len(a.games)-1 {
			a.cursor++
		}
	case "enter", " ":
		if len(a.games) == 0 {
			return a, nil
		}
		g := a.games[a.cursor]
		a.active = g.New(a.width, a.height)
		a.state = statePlaying
		return a, a.active.Init()
	}
	return a, nil
}

// updatePlaying delegates non-intercepted input to the active game model.
func (a App) updatePlaying(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.active == nil {
		return a, nil
	}
	var cmd tea.Cmd
	a.active, cmd = a.active.Update(msg)
	return a, cmd
}

// View implements tea.Model.
func (a App) View() string {
	if a.state == statePlaying && a.active != nil {
		return a.active.View()
	}
	return a.lobbyView()
}

// Lobby chrome styles, built from the shared theme palette.
var (
	lobbyTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.BrightGold)
	taglineStyle    = lipgloss.NewStyle().Foreground(theme.Dim)
	selectedStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Gold)
	unselectedStyle = lipgloss.NewStyle().Foreground(theme.SoftWhite)
	descStyle       = lipgloss.NewStyle().Foreground(theme.Dim)
	hintStyle       = lipgloss.NewStyle().Foreground(theme.Dim)
)

// lobbyView renders the branded title and the selectable game list, centered in
// the known terminal size.
func (a App) lobbyView() string {
	var b strings.Builder
	b.WriteString(lobbyTitleStyle.Render("♠ ♥  TERMINAL CASINO  ♦ ♣"))
	b.WriteString("\n")
	b.WriteString(taglineStyle.Render("Select a table"))
	b.WriteString("\n\n")

	if len(a.games) == 0 {
		b.WriteString(descStyle.Render("No games available."))
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("q/ctrl+c quit"))
		return a.frame(b.String())
	}

	for i, g := range a.games {
		cursor := "  "
		titleLine := unselectedStyle.Render(g.Title())
		if i == a.cursor {
			cursor = selectedStyle.Render("▸ ")
			titleLine = selectedStyle.Render(g.Title())
		}
		b.WriteString(cursor + titleLine)
		b.WriteString("\n")
		b.WriteString("  " + descStyle.Render(g.Description()))
		b.WriteString("\n\n")
	}

	b.WriteString(hintStyle.Render("↑/↓ (k/j) select · enter launch · q/ctrl+c quit"))
	return a.frame(b.String())
}

// frame centers the lobby content within the known terminal size when available.
func (a App) frame(content string) string {
	if a.width <= 0 || a.height <= 0 {
		return content
	}
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, content)
}
