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
	"github.com/mykeychain/terminal-casino/internal/tui"
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
		// Q or Ctrl+C quits the whole casino from anywhere — lobby or table.
		if key == "Q" || key == "ctrl+c" {
			return a, tea.Quit
		}
		if a.state == statePlaying {
			// In a game, q or Esc leaves the table and returns to the lobby.
			// Both are intercepted here, so the game model never sees them.
			if key == "q" || key == "esc" {
				a.state = stateLobby
				a.active = nil
				return a, nil
			}
			return a.updatePlaying(msg)
		}
		// In the lobby there is no table to leave, so q quits the casino too.
		if key == "q" {
			return a, tea.Quit
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

// Below these terminal heights the lobby trades vertical space for fit: compact
// (3-row) masthead cards under shortHeight, and no masthead at all under
// minMastheadHeight (the game tiles take priority).
const (
	shortHeight       = 28
	minMastheadHeight = 20
)

// tileWidth is the target inner width of a game tile; descriptions wrap to it so
// every tile lines up. The divider matches the tile's outer width (inner + 4).
const tileWidth = 46

// Lobby chrome styles, built from the shared theme palette.
var (
	wordmarkStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.BrightGold)
	taglineStyle  = lipgloss.NewStyle().Foreground(theme.Dim)
	hintStyle     = lipgloss.NewStyle().Foreground(theme.Dim)
	dividerStyle  = lipgloss.NewStyle().Foreground(theme.Dim)

	// Suit tints for the wordmark: red ♥ ♦, soft-white ♠ ♣ — matching the cards.
	suitRedStyle  = lipgloss.NewStyle().Foreground(theme.Red)
	suitDarkStyle = lipgloss.NewStyle().Foreground(theme.SoftWhite)

	// Game-tile chrome: the selected tile gets a gold border + bright title; the
	// rest use the default soft-white border with a dim title.
	tileSelBorderStyle = lipgloss.NewStyle().Foreground(theme.Gold)
	tileSelTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.BrightGold)
	tileBorderStyle    = lipgloss.NewStyle().Foreground(theme.SoftWhite)
	tileTitleStyle     = lipgloss.NewStyle().Foreground(theme.Dim)
)

// mastheadFaces spell SSH across three cards — a nod to how you reach the
// casino. The suits are ♠ ♠ ♥ (Spade Spade Heart) and the rank glyph is the
// letter itself, so the letters and the suits both say SSH.
var mastheadFaces = []tui.Face{
	{Rank: "S", Suit: "♠"},
	{Rank: "S", Suit: "♠"},
	{Rank: "H", Suit: "♥", Red: true},
}

// wordmark renders the branded title with tinted suits framing the name.
func wordmark() string {
	return suitDarkStyle.Render("♠") + " " + suitRedStyle.Render("♥") + "  " +
		wordmarkStyle.Render("TERMINAL CASINO") + "  " +
		suitRedStyle.Render("♦") + " " + suitDarkStyle.Render("♣")
}

// masthead lays the three SSH cards side by side, at the chosen vertical density.
func masthead(short bool) string {
	cards := make([]string, len(mastheadFaces))
	for i, f := range mastheadFaces {
		cards[i] = tui.RenderCard(f, short)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

// tileInner returns the game-tile inner width, shrunk to fit a narrow terminal.
func (a App) tileInner() int {
	w := tileWidth
	if a.width > 0 && a.width-8 < w {
		w = a.width - 8
	}
	if w < 20 {
		w = 20
	}
	return w
}

// gameTile frames one game as a titled panel: the title in the border, the
// description wrapped to the tile width beneath it. The selected tile is
// gold-bordered with a bright title and soft-white description; the rest are
// soft-white and dim.
func (a App) gameTile(g game.Game, selected bool) string {
	descColor := theme.Dim
	border, title := tileBorderStyle, tileTitleStyle
	if selected {
		descColor = theme.SoftWhite
		border, title = tileSelBorderStyle, tileSelTitleStyle
	}
	desc := lipgloss.NewStyle().Width(a.tileInner()).Foreground(descColor).Render(g.Description())
	return tui.TitledBoxWith(g.Title(), desc, border, title)
}

// lobbyView renders the branded masthead, wordmark, and the framed game tiles,
// centered in the known terminal size.
func (a App) lobbyView() string {
	if len(a.games) == 0 {
		empty := lipgloss.JoinVertical(lipgloss.Center,
			wordmark(),
			taglineStyle.Render("No tables open"),
			"",
			hintStyle.Render("q quit"),
		)
		return a.frame(empty)
	}

	short := a.height > 0 && a.height < shortHeight
	showMasthead := !(a.height > 0 && a.height < minMastheadHeight)

	var parts []string
	if showMasthead {
		parts = append(parts, masthead(short), "")
	}
	parts = append(parts,
		wordmark(),
		taglineStyle.Render("Select a table"),
		"",
		dividerStyle.Render(strings.Repeat("─", a.tileInner()+4)),
		"",
	)
	for i, g := range a.games {
		parts = append(parts, a.gameTile(g, i == a.cursor))
	}
	parts = append(parts,
		"",
		hintStyle.Render("↑/↓ (k/j) select · enter sit down · q quit"),
	)

	return a.frame(lipgloss.JoinVertical(lipgloss.Center, parts...))
}

// frame centers the lobby content within the known terminal size when available.
func (a App) frame(content string) string {
	if a.width <= 0 || a.height <= 0 {
		return content
	}
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, content)
}
