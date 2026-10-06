package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/iluvsuits/engine"
)

// ---- helpers ----

func card(r engine.Rank, s engine.Suit) engine.Card { return engine.Card{Rank: r, Suit: s} }

// scenarioBigFlush: the player holds a 6-card heart flush (plays up to 3×) and
// the dealer a qualifying 4-card club flush. Player cards come first.
func scenarioBigFlush() []engine.Card {
	return []engine.Card{
		card(engine.Ace, engine.Hearts), card(engine.Queen, engine.Hearts), card(engine.Nine, engine.Hearts),
		card(engine.Seven, engine.Hearts), card(engine.Four, engine.Hearts), card(engine.Two, engine.Hearts),
		card(engine.King, engine.Spades),

		card(engine.Jack, engine.Clubs), card(engine.Eight, engine.Clubs), card(engine.Six, engine.Clubs),
		card(engine.Three, engine.Clubs), card(engine.Ace, engine.Spades), card(engine.Ten, engine.Diamonds),
		card(engine.Five, engine.Diamonds),
	}
}

// newTestModel builds a Model over a stacked-deck game with the given wagers
// already posted.
func newTestModel(bankroll, ante, flushRush int, cards []engine.Card) Model {
	mk := func() *engine.Game {
		g := engine.NewGameWithDeck(bankroll, cards)
		_ = g.SetAnte(ante)
		_ = g.SetFlushRush(flushRush)
		return g
	}
	return New(mk(), mk).(Model)
}

var (
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyRight = tea.KeyMsg{Type: tea.KeyRight}
	keyLeft  = tea.KeyMsg{Type: tea.KeyLeft}
	keyTab   = tea.KeyMsg{Type: tea.KeyTab}
)

func keyRune(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func upd(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	mm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want ui.Model", next)
	}
	return mm, cmd
}

func advanceDeal(t *testing.T, m Model) Model {
	t.Helper()
	for i := 0; m.animState == animDealReveal; i++ {
		if i > 64 {
			t.Fatal("deal cascade did not finish")
		}
		m, _ = upd(t, m, dealTickMsg{})
	}
	return m
}

func advanceDealer(t *testing.T, m Model) Model {
	t.Helper()
	for i := 0; m.animState == animDealerReveal; i++ {
		if i > 64 {
			t.Fatal("dealer reveal did not finish")
		}
		m, _ = upd(t, m, dealerTickMsg{})
	}
	return m
}

func settleResult(t *testing.T, m Model) Model {
	t.Helper()
	for i := 0; m.animState == animResult && m.bankStep < bankrollSteps; i++ {
		if i > 64 {
			t.Fatal("bankroll tick did not converge")
		}
		m, _ = upd(t, m, bankrollTickMsg{})
	}
	m, _ = upd(t, m, resultDoneMsg{})
	return m
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func sized(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	mm, _ := upd(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	return mm
}

// ---- phase fixtures ----

func bettingState(t *testing.T) Model {
	t.Helper()
	return newTestModel(1000, 10, 5, scenarioBigFlush())
}

func decisionState(t *testing.T) Model {
	t.Helper()
	m, _ := upd(t, bettingState(t), keyEnter)
	return advanceDeal(t, m)
}

func resultState(t *testing.T) Model {
	t.Helper()
	m, _ := upd(t, decisionState(t), keyRune('3'))
	return settleResult(t, advanceDealer(t, m))
}

// ---- tests ----

func TestDealCascadeThenDecision(t *testing.T) {
	m, cmd := upd(t, bettingState(t), keyEnter)
	if m.animState != animDealReveal || cmd == nil {
		t.Fatalf("after deal: animState = %v, cmd nil = %v", m.animState, cmd == nil)
	}
	if m.displayBankroll != 1000 {
		t.Fatalf("displayBankroll = %d during deal, want 1000 (escrow masked)", m.displayBankroll)
	}
	f := m.frame()
	if f.playerCards != 1 || f.dealerCards != 0 {
		t.Fatalf("first frame player=%d dealer=%d, want 1/0", f.playerCards, f.dealerCards)
	}
	m = advanceDeal(t, m)
	if m.animState != animIdle || m.game.Phase() != engine.PhaseDecision {
		t.Fatalf("after deal: animState=%v phase=%v", m.animState, m.game.Phase())
	}
	if f := m.frame(); f.playerCards != handSize || f.dealerCards != handSize || f.dealerFaceUp != 0 {
		t.Fatalf("decision frame = %+v, want all cards with dealer face down", f)
	}
}

func TestDecisionMenuOffersFlushCappedPlays(t *testing.T) {
	m := decisionState(t)
	got := m.decisionActions()
	want := []int{1, 2, 3, foldAction}
	if len(got) != len(want) {
		t.Fatalf("actions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("actions = %v, want %v", got, want)
		}
	}
	view := stripANSI(sized(t, m, 100, 40).View())
	for _, s := range []string{"Play 3× $30", "Fold", "6-card ♥ flush, Ace-high", "(play up to 3×)"} {
		if !strings.Contains(view, s) {
			t.Errorf("decision view missing %q", s)
		}
	}
}

func TestPlayThreeTimesWins(t *testing.T) {
	m := resultState(t)
	s := m.game.Result()
	// Ante $10 + Play $30 win 1:1, Flush Rush $5 on a 6-card flush pays 20:1.
	if s.Net != 10+30+100 {
		t.Fatalf("net = %d, want %d", s.Net, 140)
	}
	if m.displayBankroll != 1140 || m.game.Bankroll() != 1140 {
		t.Fatalf("bankroll display=%d engine=%d, want 1140", m.displayBankroll, m.game.Bankroll())
	}
	view := stripANSI(sized(t, m, 100, 40).View())
	for _, s := range []string{"YOU WIN +$140", "Play 3×", "Flush Rush", "6-card flush 20:1", "(qualifies)"} {
		if !strings.Contains(view, s) {
			t.Errorf("result view missing %q", s)
		}
	}
}

func TestFoldSettlesAndRevealsDealer(t *testing.T) {
	m, cmd := upd(t, decisionState(t), keyRune('f'))
	if m.animState != animDealerReveal || cmd == nil {
		t.Fatalf("after fold: animState = %v", m.animState)
	}
	m = settleResult(t, advanceDealer(t, m))
	if s := m.game.Result(); !s.Folded || s.Net != -10+100 {
		t.Fatalf("settlement = %+v, want fold with flush rush still paid", s)
	}
}

func TestInputGatedDuringDeal(t *testing.T) {
	m, _ := upd(t, bettingState(t), keyEnter)
	m, _ = upd(t, m, keyRune('3'))
	if m.game.Phase() != engine.PhaseDecision || m.game.PlayBet() != 0 {
		t.Fatal("a play key during the deal cascade must be ignored")
	}
}

func TestBettingAdjustAndSwitch(t *testing.T) {
	m := newTestModel(1000, 3, 0, scenarioBigFlush())
	m, _ = upd(t, m, keyLeft) // the Ante floors at the minimum
	if m.game.Ante() != engine.MinBet {
		t.Fatalf("ante = %d, want floor %d", m.game.Ante(), engine.MinBet)
	}
	m, _ = upd(t, m, keyRight)
	if m.game.Ante() != 2*engine.MinBet {
		t.Fatalf("ante = %d, want %d", m.game.Ante(), 2*engine.MinBet)
	}
	m, _ = upd(t, m, keyTab)
	m, _ = upd(t, m, keyRight)
	if m.game.FlushRush() != engine.MinBet {
		t.Fatalf("flush rush = %d, want %d", m.game.FlushRush(), engine.MinBet)
	}
	m, _ = upd(t, m, keyLeft) // a side bet clears to $0
	m, _ = upd(t, m, keyTab)
	m, _ = upd(t, m, keyRight)
	if m.game.FlushRush() != 0 || m.game.SuperFlushRush() != engine.MinBet {
		t.Fatalf("side bets = %d/%d, want 0/%d", m.game.FlushRush(), m.game.SuperFlushRush(), engine.MinBet)
	}
}

func TestPaytableOverlayToggles(t *testing.T) {
	m := sized(t, bettingState(t), 100, 40)
	m, _ = upd(t, m, keyRune('?'))
	view := stripANSI(m.View())
	for _, s := range []string{"Flush Rush", "Super Flush Rush", "7-card flush", "100:1", "up to 3× Ante"} {
		if !strings.Contains(view, s) {
			t.Errorf("paytable missing %q", s)
		}
	}
	m, _ = upd(t, m, keyEnter)
	if m.showPaytable || m.game.Phase() != engine.PhaseBetting {
		t.Fatal("enter must only dismiss the paytable")
	}
}

// TestHeightSweepAnchoredFrame: across a sweep of terminal sizes (from the
// standard 80-plus columns the key hints fit in) every phase
// composes to exactly m.height rows with the title on the first line and the key
// hint on the last, and no row overflows the width.
func TestHeightSweepAnchoredFrame(t *testing.T) {
	phases := []struct {
		name string
		make func(*testing.T) Model
		hint string
	}{
		{"betting", bettingState, "enter deal"},
		{"decision", decisionState, "enter confirm"},
		{"result", resultState, "enter continue"},
	}
	for _, p := range phases {
		for _, w := range []int{82, 90, 120} {
			for h := 24; h <= 50; h += 2 {
				m := sized(t, p.make(t), w, h)
				view := m.View()
				lines := strings.Split(stripANSI(view), "\n")
				if len(lines) != h {
					t.Fatalf("%s %dx%d: %d rows, want %d", p.name, w, h, len(lines), h)
				}
				if !strings.HasPrefix(lines[0], "I Luv Suits") {
					t.Fatalf("%s %dx%d: first line %q", p.name, w, h, lines[0])
				}
				if !strings.Contains(lines[h-1], p.hint) {
					t.Fatalf("%s %dx%d: last line %q, want hint %q", p.name, w, h, lines[h-1], p.hint)
				}
				for i, l := range strings.Split(view, "\n") {
					if lipgloss.Width(l) > w {
						t.Fatalf("%s %dx%d: row %d is %d wide", p.name, w, h, i, lipgloss.Width(l))
					}
				}
			}
		}
	}
}

// TestRenderedFrames logs a few full frames for eyeballing with `go test -v`.
func TestRenderedFrames(t *testing.T) {
	for _, p := range []struct {
		name string
		m    Model
	}{
		{"betting", bettingState(t)},
		{"decision", decisionState(t)},
		{"result", resultState(t)},
	} {
		t.Logf("---- %s (90x30) ----\n%s", p.name, stripANSI(sized(t, p.m, 90, 30).View()))
	}
}
