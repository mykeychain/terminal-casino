package casino

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/game"
)

// stubModel is a trivial tea.Model used as a launched game so the tests don't
// depend on any concrete game's internals.
type stubModel struct {
	initialized bool
}

func (m stubModel) Init() tea.Cmd { m.initialized = true; return nil }
func (m stubModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}
func (m stubModel) View() string { return "stub" }

// stubGame is a trivial game.Game whose New returns a stubModel.
type stubGame struct {
	title string
}

func (g stubGame) Title() string       { return g.title }
func (g stubGame) Description() string { return "desc for " + g.title }
func (g stubGame) New(w, h int) tea.Model {
	return stubModel{}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func newTestApp() App {
	m := NewApp([]game.Game{
		stubGame{title: "Alpha"},
		stubGame{title: "Beta"},
		stubGame{title: "Gamma"},
	})
	return m.(App)
}

func TestCursorMovesInLobby(t *testing.T) {
	a := newTestApp()
	if a.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", a.cursor)
	}

	m, _ := a.Update(key("down"))
	a = m.(App)
	if a.cursor != 1 {
		t.Fatalf("after down, cursor = %d, want 1", a.cursor)
	}

	// j also moves down.
	m, _ = a.Update(key("j"))
	a = m.(App)
	if a.cursor != 2 {
		t.Fatalf("after j, cursor = %d, want 2", a.cursor)
	}

	// Clamp at the bottom.
	m, _ = a.Update(key("down"))
	a = m.(App)
	if a.cursor != 2 {
		t.Fatalf("cursor should clamp at 2, got %d", a.cursor)
	}

	// up / k move back.
	m, _ = a.Update(key("up"))
	a = m.(App)
	m, _ = a.Update(key("k"))
	a = m.(App)
	if a.cursor != 0 {
		t.Fatalf("after up+k, cursor = %d, want 0", a.cursor)
	}

	// Clamp at the top.
	m, _ = a.Update(key("up"))
	a = m.(App)
	if a.cursor != 0 {
		t.Fatalf("cursor should clamp at 0, got %d", a.cursor)
	}
}

func TestEnterLaunchesGame(t *testing.T) {
	a := newTestApp()
	// Move to Beta, then launch.
	m, _ := a.Update(key("down"))
	a = m.(App)
	m, _ = a.Update(key("enter"))
	a = m.(App)

	if a.state != statePlaying {
		t.Fatalf("state = %v, want statePlaying", a.state)
	}
	if a.active == nil {
		t.Fatalf("active model is nil after launch")
	}
	if _, ok := a.active.(stubModel); !ok {
		t.Fatalf("active model type = %T, want stubModel", a.active)
	}
}

func TestSpaceLaunchesGame(t *testing.T) {
	a := newTestApp()
	m, _ := a.Update(key(" "))
	a = m.(App)
	if a.state != statePlaying || a.active == nil {
		t.Fatalf("space did not launch: state=%v active=%v", a.state, a.active)
	}
}

func TestEscReturnsToLobby(t *testing.T) {
	a := newTestApp()
	m, _ := a.Update(key("enter"))
	a = m.(App)
	if a.state != statePlaying {
		t.Fatalf("precondition failed: not playing")
	}

	m, _ = a.Update(key("esc"))
	a = m.(App)
	if a.state != stateLobby {
		t.Fatalf("state = %v, want stateLobby after esc", a.state)
	}
	if a.active != nil {
		t.Fatalf("active model should be dropped after esc, got %T", a.active)
	}
}

func TestQuitFromLobby(t *testing.T) {
	a := newTestApp()
	_, cmd := a.Update(key("q"))
	if cmd == nil {
		t.Fatalf("q in lobby returned nil cmd, want tea.Quit")
	}
	if msg := cmd(); msg == nil {
		t.Fatalf("quit cmd produced nil msg")
	} else if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("q cmd = %T, want tea.QuitMsg", msg)
	}
}

// TestQReturnsToLobbyWhilePlaying: in a game, q leaves the table for the lobby
// (it does not quit the program) — the same as Esc.
func TestQReturnsToLobbyWhilePlaying(t *testing.T) {
	a := newTestApp()
	m, _ := a.Update(key("enter"))
	a = m.(App)
	if a.state != statePlaying {
		t.Fatalf("precondition failed: not playing")
	}

	m, cmd := a.Update(key("q"))
	a = m.(App)
	if a.state != stateLobby {
		t.Fatalf("q while playing: state = %v, want stateLobby", a.state)
	}
	if a.active != nil {
		t.Fatalf("q while playing should drop the game model, got %T", a.active)
	}
	if cmd != nil {
		t.Fatalf("q while playing should not quit the program")
	}
}

// TestQuitEntirelyWhilePlaying: Q quits the whole casino from inside a game.
func TestQuitEntirelyWhilePlaying(t *testing.T) {
	a := newTestApp()
	m, _ := a.Update(key("enter"))
	a = m.(App)
	_, cmd := a.Update(key("Q"))
	if cmd == nil {
		t.Fatalf("Q while playing returned nil cmd, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("Q while playing did not produce QuitMsg")
	}
}

// TestQuitEntirelyFromLobby: Q also quits from the lobby.
func TestQuitEntirelyFromLobby(t *testing.T) {
	a := newTestApp()
	_, cmd := a.Update(key("Q"))
	if cmd == nil || func() bool { _, ok := cmd().(tea.QuitMsg); return !ok }() {
		t.Fatalf("Q in lobby did not produce QuitMsg")
	}
}

func TestRegistryLengthRespected(t *testing.T) {
	a := newTestApp()
	if len(a.games) != 3 {
		t.Fatalf("registry length = %d, want 3", len(a.games))
	}

	// Empty registry: Enter must not launch or panic.
	empty := NewApp(nil).(App)
	m, cmd := empty.Update(key("enter"))
	empty = m.(App)
	if empty.state != stateLobby {
		t.Fatalf("empty registry launched a game")
	}
	if cmd != nil {
		t.Fatalf("empty registry enter returned a non-nil cmd")
	}
}

func TestWindowSizeStored(t *testing.T) {
	a := newTestApp()
	m, _ := a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = m.(App)
	if a.width != 120 || a.height != 40 {
		t.Fatalf("size not stored: got %dx%d", a.width, a.height)
	}
}

func TestLobbyViewRenders(t *testing.T) {
	m := NewApp([]game.Game{
		stubGame{title: "3:2 Blackjack"},
		stubGame{title: "Mississippi Stud"},
	})
	a := m.(App)
	sm, _ := a.Update(tea.WindowSizeMsg{Width: 100, Height: 44})
	a = sm.(App)
	view := a.View()

	// The masthead cards, the wordmark, both game titles, the selection marker,
	// and the hint are all present.
	for _, want := range []string{"╭─────╮", "TERMINAL CASINO", "3:2 Blackjack", "Mississippi Stud", "▸", "play"} {
		if !strings.Contains(view, want) {
			t.Fatalf("lobby view missing %q", want)
		}
	}
	// Only the selected game's tagline is shown.
	if !strings.Contains(view, "desc for 3:2 Blackjack") {
		t.Fatalf("lobby view missing the selected game's tagline")
	}
	if strings.Contains(view, "desc for Mississippi Stud") {
		t.Fatalf("lobby view shows an unselected game's tagline")
	}

	// Moving the cursor swaps which tagline is shown.
	dm, _ := a.Update(tea.KeyMsg{Type: tea.KeyDown})
	view = dm.(App).View()
	if !strings.Contains(view, "desc for Mississippi Stud") || strings.Contains(view, "desc for 3:2 Blackjack") {
		t.Fatalf("tagline did not follow the cursor")
	}
}

func TestLobbyViewCompactNoPanic(t *testing.T) {
	a := newTestApp()
	// A short terminal drops the masthead; the game tiles must still render.
	m, _ := a.Update(tea.WindowSizeMsg{Width: 52, Height: 15})
	a = m.(App)
	if view := a.View(); !strings.Contains(view, "Alpha") {
		t.Fatalf("compact lobby missing a game title:\n%s", view)
	}
}
