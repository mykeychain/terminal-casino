package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/poker3card/engine"
)

// ansiRe strips SGR escape sequences so assertions can look at the plain glyphs.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// compactCardRe matches a rank immediately followed by a suit glyph — the
// signature of a compact (3-row) card body, "10♠", which never occurs at full
// density where the suit sits alone on its own centered row.
var compactCardRe = regexp.MustCompile(`(10|[2-9AJQK])[♠♥♦♣]`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func sized(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	mm, _ := upd(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	return mm
}

func lastLines(s string, n int) string {
	lines := strings.Split(stripANSI(s), "\n")
	if n > len(lines) {
		n = len(lines)
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// ---- phase fixtures ----

func bettingState(t *testing.T) Model {
	t.Helper()
	return newTestModel(1000, 9, 6, scenarioBigWin())
}

func decisionState(t *testing.T) Model {
	t.Helper()
	m := bettingState(t)
	m, _ = upd(t, m, keyEnter)
	return advanceDeal(t, m)
}

func resultState(t *testing.T) Model {
	t.Helper()
	m := decisionState(t)
	for i, n := 0, playIndex(t, m); i < n; i++ {
		m, _ = upd(t, m, keyRight)
	}
	m, _ = upd(t, m, keyEnter)
	m = advanceDealer(t, m)
	return settleResult(t, m)
}

// TestHeightSweepAnchoredFrame is the responsive-layout guarantee: across a sweep
// of terminal heights every phase composes to exactly m.height rows, the title is
// always the first line, and the control panel is always pinned to the last rows.
func TestHeightSweepAnchoredFrame(t *testing.T) {
	const width = 90

	cases := []struct {
		name       string
		make       func(*testing.T) Model
		control    string
		panelTitle string
		hasCards   bool
	}{
		{"betting", bettingState, "enter deal", "Place your wagers", false},
		{"decision", decisionState, "enter confirm", "Your move", true},
		{"result", resultState, "enter continue", "Round over", true},
	}

	for _, tc := range cases {
		base := tc.make(t)
		for _, h := range []int{20, 24, 30, 40} {
			view := sized(t, base, width, h).View()

			if got := lipgloss.Height(view); got != h {
				t.Fatalf("%s h=%d: view height = %d, want exactly %d", tc.name, h, got, h)
			}
			first := stripANSI(strings.SplitN(view, "\n", 2)[0])
			if !strings.Contains(first, "Three-Card Poker") {
				t.Fatalf("%s h=%d: first line %q lacks the title", tc.name, h, first)
			}
			tail := lastLines(view, 9)
			if !strings.Contains(tail, tc.control) {
				t.Fatalf("%s h=%d: control hint %q not in bottom rows:\n%s", tc.name, h, tc.control, tail)
			}
			if !strings.Contains(tail, tc.panelTitle) {
				t.Fatalf("%s h=%d: panel title %q not in bottom rows:\n%s", tc.name, h, tc.panelTitle, tail)
			}
		}
	}
}

// TestDensitySwitchesWithHeight checks the table drops to compact (3-row) cards
// when the middle region is tight and uses full 5-row cards when there is room.
func TestDensitySwitchesWithHeight(t *testing.T) {
	base := decisionState(t)
	compact := compactCardRe.MatchString(stripANSI(sized(t, base, 90, 16).View()))
	if !compact {
		t.Fatal("h=16: expected compact cards")
	}
	full := compactCardRe.MatchString(stripANSI(sized(t, base, 90, 40).View()))
	if full {
		t.Fatal("h=40: expected full-density cards")
	}
}

// TestViewFallbackBeforeSize verifies the pre-size fallback renders the title and
// control panel via simple stacking rather than panicking.
func TestViewFallbackBeforeSize(t *testing.T) {
	m := bettingState(t)
	view := stripANSI(m.View())
	if !strings.Contains(view, "Three-Card Poker") {
		t.Fatalf("fallback view lacks title:\n%s", view)
	}
	if !strings.Contains(view, "enter deal") {
		t.Fatalf("fallback view lacks control panel:\n%s", view)
	}
}

// TestBettingAdjustAndSwitch exercises the two-wager betting input: switching
// focus with tab, stepping on the MinBet grid, clearing a wager to $0, and the
// bankroll cap.
func TestBettingAdjustAndSwitch(t *testing.T) {
	// Start from a clean default game (Ante = MinBet, no Pair Plus).
	mk := func() *engine.Game { return engine.NewGameWithDeck(1000, scenarioBigWin()) }
	m := New(mk(), mk).(Model)

	if m.game.Ante() != engine.MinBet {
		t.Fatalf("initial ante = %d, want %d", m.game.Ante(), engine.MinBet)
	}
	// Right steps the Ante up by MinBet.
	m, _ = upd(t, m, keyRight)
	if m.game.Ante() != 2*engine.MinBet {
		t.Fatalf("ante after right = %d, want %d", m.game.Ante(), 2*engine.MinBet)
	}
	// Left twice clears it to 0 (crosses below MinBet -> cleared).
	m, _ = upd(t, m, keyLeft)
	m, _ = upd(t, m, keyLeft)
	if m.game.Ante() != 0 {
		t.Fatalf("ante after two lefts = %d, want 0 (cleared)", m.game.Ante())
	}
	// Switch focus to Pair Plus and raise it.
	m, _ = upd(t, m, keyTab)
	if m.focusedSpot != spotPairPlus {
		t.Fatalf("focusedSpot = %d, want %d after tab", m.focusedSpot, spotPairPlus)
	}
	m, _ = upd(t, m, keyRight)
	if m.game.PairPlus() != engine.MinBet {
		t.Fatalf("pair plus = %d, want %d", m.game.PairPlus(), engine.MinBet)
	}
	// Pair Plus-only deal is legal (one wager suffices).
	m, _ = upd(t, m, keyEnter)
	if m.animState != animDealReveal {
		t.Fatalf("deal with Pair Plus only should start the reveal, animState = %v", m.animState)
	}
}

// TestDealRejectedWithNoWager checks that trying to deal with both wagers cleared
// surfaces a message instead of crashing or dealing.
func TestDealRejectedWithNoWager(t *testing.T) {
	mk := func() *engine.Game {
		g := engine.NewGameWithDeck(1000, scenarioBigWin())
		_ = g.SetAnte(0)
		return g
	}
	m := New(mk(), mk).(Model)
	m, _ = upd(t, m, keyEnter) // no wager -> rejected
	if m.animState != animIdle {
		t.Fatalf("animState = %v, want animIdle (deal rejected)", m.animState)
	}
	if m.game.Phase() != engine.PhaseBetting {
		t.Fatalf("phase = %v, want PhaseBetting", m.game.Phase())
	}
	if !strings.Contains(m.msg, "Ante or Pair Plus") {
		t.Fatalf("msg = %q, want a place-a-wager hint", m.msg)
	}
}
