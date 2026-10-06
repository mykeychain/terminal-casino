package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/djwild/engine"
)

// ---- helpers ----

func card(r engine.Rank, s engine.Suit) engine.Card { return engine.Card{Rank: r, Suit: s} }

// scenarioWildFullHouse: the player holds a wild full house (two pair plus the
// joker) and the dealer a pair of Queens. Player cards come first.
func scenarioWildFullHouse() []engine.Card {
	return []engine.Card{
		card(engine.King, engine.Hearts), card(engine.King, engine.Spades), engine.Joker,
		card(engine.Four, engine.Clubs), card(engine.Four, engine.Hearts),

		card(engine.Queen, engine.Hearts), card(engine.Queen, engine.Spades), card(engine.Nine, engine.Diamonds),
		card(engine.Six, engine.Clubs), card(engine.Three, engine.Clubs),
	}
}

// newTestModel builds a Model over a stacked-deck game with the given wagers
// already posted.
func newTestModel(bankroll, ante, trips int, cards []engine.Card) Model {
	mk := func() *engine.Game {
		g := engine.NewGameWithDeck(bankroll, cards)
		_ = g.SetAnte(ante)
		_ = g.SetTrips(trips)
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
	return newTestModel(1000, 10, 5, scenarioWildFullHouse())
}

func decisionState(t *testing.T) Model {
	t.Helper()
	m, _ := upd(t, bettingState(t), keyEnter)
	return advanceDeal(t, m)
}

func resultState(t *testing.T) Model {
	t.Helper()
	m, _ := upd(t, decisionState(t), keyRune('p'))
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
	m = advanceDeal(t, m)
	if m.animState != animIdle || m.game.Phase() != engine.PhaseDecision {
		t.Fatalf("after deal: animState=%v phase=%v", m.animState, m.game.Phase())
	}
	if f := m.frame(); f.playerCards != handSize || f.dealerCards != handSize || f.dealerFaceUp != 0 {
		t.Fatalf("decision frame = %+v, want all cards with dealer face down", f)
	}
	view := stripANSI(sized(t, m, 100, 40).View())
	for _, s := range []string{"Full House", "(with wilds)", "JK", "wild", "Play 2× $20", "Fold"} {
		if !strings.Contains(view, s) {
			t.Errorf("decision view missing %q", s)
		}
	}
}

func TestPlayWinPaysBlindAndTrips(t *testing.T) {
	m := resultState(t)
	s := m.game.Result()
	// Ante +10, Play +20, Blind Full House 3:1 +30, Trips wild Full House 6:1 on $5.
	want := 10 + 20 + 10*engine.BlindPayouts[engine.FullHouse] + 5*engine.TripsWildPayouts[engine.FullHouse]
	if s.Net != want {
		t.Fatalf("net = %d, want %d", s.Net, want)
	}
	if m.displayBankroll != 1000+want || m.game.Bankroll() != 1000+want {
		t.Fatalf("bankroll display=%d engine=%d, want %d", m.displayBankroll, m.game.Bankroll(), 1000+want)
	}
	view := stripANSI(sized(t, m, 100, 40).View())
	for _, s := range []string{"YOU WIN", "Blind", "Full House 3:1", "wild Full House 6:1", "Pair of Queens"} {
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
	want := -20 + 5*engine.TripsWildPayouts[engine.FullHouse]
	if s := m.game.Result(); !s.Folded || s.Net != want {
		t.Fatalf("settlement = %+v, want fold net %d", s, want)
	}
}

func TestInputGatedDuringDeal(t *testing.T) {
	m, _ := upd(t, bettingState(t), keyEnter)
	m, _ = upd(t, m, keyRune('p'))
	if m.game.Phase() != engine.PhaseDecision || m.game.PlayBet() != 0 {
		t.Fatal("a play key during the deal cascade must be ignored")
	}
}

func TestBettingAdjustAndSwitch(t *testing.T) {
	m := newTestModel(100, 3, 0, scenarioWildFullHouse())
	m, _ = upd(t, m, keyLeft) // the Ante floors at the minimum
	if m.game.Ante() != engine.MinBet {
		t.Fatalf("ante = %d, want floor %d", m.game.Ante(), engine.MinBet)
	}
	m, _ = upd(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m, _ = upd(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m, _ = upd(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m, _ = upd(t, m, tea.KeyMsg{Type: tea.KeyUp}) // 63 would need $126: capped at $48 (+ blind = $96)
	if m.game.Ante() != 48 || m.game.Blind() != 48 {
		t.Fatalf("ante/blind = %d/%d, want 48/48", m.game.Ante(), m.game.Blind())
	}
	m, _ = upd(t, m, keyTab)
	m, _ = upd(t, m, keyRight)
	if m.game.Trips() != 3 {
		t.Fatalf("trips = %d, want 3", m.game.Trips())
	}
	m, _ = upd(t, m, keyRight) // no room left
	if m.game.Trips() != 3 {
		t.Fatalf("trips = %d, want capped at 3", m.game.Trips())
	}
}

func TestPaytableOverlayToggles(t *testing.T) {
	m := sized(t, bettingState(t), 100, 40)
	m, _ = upd(t, m, keyRune('?'))
	view := stripANSI(m.View())
	for _, s := range []string{"Five Wilds", "1000:1", "2000:1", "Trips natural", "push"} {
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
// standard 80-plus columns the key hints fit in) every phase composes to exactly
// m.height rows with the title on the first line and the key hint on the last,
// and no row overflows the width.
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
				if !strings.HasPrefix(lines[0], "DJ Wild") {
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
	m, _ := upd(t, sized(t, bettingState(t), 90, 30), keyRune('?'))
	t.Logf("---- paytable ----\n%s", stripANSI(m.View()))
}
