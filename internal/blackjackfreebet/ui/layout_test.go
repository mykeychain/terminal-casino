package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/blackjackfreebet/engine"
)

// ansiRe strips SGR escape sequences so assertions can look at the plain glyphs.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// compactCardRe matches a rank immediately followed by a suit glyph — the
// signature of a compact (3-row) card body, "10♠", which never occurs at full
// density where the suit sits alone on its own centered row.
var compactCardRe = regexp.MustCompile(`(10|[2-9AJQK])[♠♥♦♣]`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// sized delivers a WindowSizeMsg so the View composes to exactly w×h.
func sized(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	return upSize(t, m, tea.WindowSizeMsg{Width: w, Height: h})
}

func upSize(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	mm, _ := upd(t, m, msg)
	return mm
}

// lastLines returns the final n lines of s, ANSI stripped and joined, for
// asserting that the control panel is pinned to the bottom.
func lastLines(s string, n int) string {
	lines := strings.Split(stripANSI(s), "\n")
	if n > len(lines) {
		n = len(lines)
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// bettingState is a fresh single-spot betting model.
func bettingState(t *testing.T) Model {
	t.Helper()
	return newModel(1000, engine.Ten, engine.Six, engine.Nine, engine.Ten, engine.Ten)
}

// playerTurnState deals a single hand and advances to the player's turn.
func playerTurnState(t *testing.T) Model {
	t.Helper()
	m := bettingState(t)
	m, _ = upd(t, m, keyEnter)
	return advanceDeal(t, m)
}

// resultState plays a stand-and-win round through to the settled round-over
// screen (banner + "Round over" panel, controls unlocked).
func resultState(t *testing.T) Model {
	t.Helper()
	m := playerTurnState(t)
	for i, n := 0, standIndex(t, m); i < n; i++ {
		m, _ = upd(t, m, keyRight)
	}
	m, _ = upd(t, m, keyEnter)
	m = advanceDealer(t, m)
	return settleResult(t, m)
}

// twoHandState opens a second spot and deals, landing on a two-hand player turn.
func twoHandState(t *testing.T) Model {
	t.Helper()
	m := newModel(1000, engine.Nine, engine.Six, engine.Seven, engine.Ten,
		engine.Five, engine.Eight, engine.Four, engine.Ten)
	m, _ = upd(t, m, keyRune('a')) // add a second spot
	m, _ = upd(t, m, keyEnter)     // deal
	return advanceDeal(t, m)
}

// TestHeightSweepAnchoredFrame is the responsive-layout guarantee: across a sweep
// of terminal heights, every phase composes to *exactly* m.height rows, the title
// is always the first line, the control panel is always pinned to the last rows,
// and the table switches to compact cards at small heights and full cards at
// large ones. Uses a stacked-shoe game (ui.New) so play is deterministic.
func TestHeightSweepAnchoredFrame(t *testing.T) {
	const width = 80

	cases := []struct {
		name string
		make func(*testing.T) Model
		// control text pinned to the bottom for this phase (the hint line).
		control string
		// panelTitle appears in the bottom control box.
		panelTitle string
		// hasCards is true for phases that render a table (so density applies).
		hasCards bool
	}{
		{"betting", bettingState, "enter deal", "Place your bet", false},
		{"player-turn", playerTurnState, "enter confirm", "Your move", true},
		{"result", resultState, "enter continue", "Round over", true},
		{"two-hand", twoHandState, "enter confirm", "Your move", true},
	}

	for _, tc := range cases {
		base := tc.make(t)
		for _, h := range []int{18, 20, 24, 30, 40} {
			view := sized(t, base, width, h).View()

			// Exact fit: the frame is precisely h rows tall.
			if got := lipgloss.Height(view); got != h {
				t.Fatalf("%s h=%d: view height = %d, want exactly %d", tc.name, h, got, h)
			}

			// Title pinned to row 0.
			first := stripANSI(strings.SplitN(view, "\n", 2)[0])
			if !strings.Contains(first, "Free Bet Blackjack") {
				t.Fatalf("%s h=%d: first line %q lacks the title", tc.name, h, first)
			}

			// Control panel pinned to the bottom rows.
			tail := lastLines(view, 6)
			if !strings.Contains(tail, tc.control) {
				t.Fatalf("%s h=%d: control hint %q not in bottom rows:\n%s", tc.name, h, tc.control, tail)
			}
			if !strings.Contains(tail, tc.panelTitle) {
				t.Fatalf("%s h=%d: panel title %q not in bottom rows:\n%s", tc.name, h, tc.panelTitle, tail)
			}

			// Height-aware density: compact cards at the small heights, full at the
			// large ones. Only meaningful for phases that draw a table.
			if tc.hasCards {
				compact := compactCardRe.MatchString(stripANSI(view))
				wantCompact := h <= 20
				if compact != wantCompact {
					t.Fatalf("%s h=%d: compact-card density = %v, want %v", tc.name, h, compact, wantCompact)
				}
			}
		}
	}
}

// TestViewFallbackBeforeSize verifies the pre-size fallback: with no WindowSizeMsg
// yet (width/height == 0) the View still renders the title and control panel via
// simple stacking rather than panicking or forcing a zero-height frame.
func TestViewFallbackBeforeSize(t *testing.T) {
	m := bettingState(t)
	view := m.View()
	if !strings.Contains(stripANSI(view), "Free Bet Blackjack") {
		t.Fatalf("fallback view lacks title:\n%s", stripANSI(view))
	}
	if !strings.Contains(stripANSI(view), "enter deal") {
		t.Fatalf("fallback view lacks control panel:\n%s", stripANSI(view))
	}
}
