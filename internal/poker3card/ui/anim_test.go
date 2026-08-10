package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/poker3card/engine"
)

// ---- helpers ----

func card(r engine.Rank, s engine.Suit) engine.Card { return engine.Card{Rank: r, Suit: s} }

// scenarioBigWin is the documented screenshot scenario: player straight flush,
// dealer 9-high (does not qualify), Ante $9 + Pair Plus $6, player Plays. Net +$294.
func scenarioBigWin() []engine.Card {
	return []engine.Card{
		card(engine.Ace, engine.Spades), card(engine.King, engine.Spades), card(engine.Queen, engine.Spades),
		card(engine.Five, engine.Diamonds), card(engine.Nine, engine.Clubs), card(engine.Two, engine.Hearts),
	}
}

// newTestModel builds a Model over a stacked-deck game with the given wagers
// already posted. A zero wager clears that bet (pass ante=0 for a Pair Plus-only
// hand).
func newTestModel(bankroll, ante, pairPlus int, cards []engine.Card) Model {
	mk := func() *engine.Game {
		g := engine.NewGameWithDeck(bankroll, cards)
		_ = g.SetAnte(ante)
		_ = g.SetPairPlus(pairPlus)
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
			t.Fatal("deal reveal did not terminate")
		}
		m, _ = upd(t, m, dealTickMsg{})
	}
	return m
}

func advanceDealer(t *testing.T, m Model) Model {
	t.Helper()
	for i := 0; m.animState == animDealerReveal; i++ {
		if i > 64 {
			t.Fatal("dealer reveal did not terminate")
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

// playIndex returns how many "right" presses select Play in the decision menu.
func playIndex(t *testing.T, m Model) int {
	t.Helper()
	for i, a := range m.decisionActions() {
		if a == actionPlay {
			return i
		}
	}
	t.Fatal("Play is not a legal decision")
	return 0
}

// ---- tests ----

// TestDealCascadeInterleaves checks the deal reveal advances one position per
// tick and the frame's player/dealer counts follow the interleaved schedule.
func TestDealCascadeInterleaves(t *testing.T) {
	m := newTestModel(1000, 9, 6, scenarioBigWin())
	m, cmd := upd(t, m, keyEnter) // deal
	if m.animState != animDealReveal {
		t.Fatalf("after deal: animState = %v, want animDealReveal", m.animState)
	}
	if cmd == nil {
		t.Fatal("deal should return a tick command")
	}
	// Escrow masked: header still shows the pre-deal bankroll.
	if m.displayBankroll != 1000 {
		t.Fatalf("displayBankroll = %d during deal, want 1000", m.displayBankroll)
	}

	// dealShown 1..6 maps to player=ceil(d/2), dealer=floor(d/2).
	wantP := []int{1, 1, 2, 2, 3, 3}
	wantD := []int{0, 1, 1, 2, 2, 3}
	for d := 1; d <= dealPositions(); d++ {
		f := m.frame()
		if f.playerCards != wantP[d-1] || f.dealerCards != wantD[d-1] {
			t.Fatalf("dealShown=%d: frame player=%d dealer=%d, want %d/%d",
				d, f.playerCards, f.dealerCards, wantP[d-1], wantD[d-1])
		}
		if f.dealerFaceUp != 0 {
			t.Fatalf("dealShown=%d: dealer face up %d, want 0 (backs)", d, f.dealerFaceUp)
		}
		if d < dealPositions() {
			m, _ = upd(t, m, dealTickMsg{})
		}
	}

	// One more tick lands the deal: Ante hand -> idle at the decision.
	m, _ = upd(t, m, dealTickMsg{})
	if m.animState != animIdle {
		t.Fatalf("after full deal: animState = %v, want animIdle", m.animState)
	}
	if m.game.Phase() != engine.PhaseDecision {
		t.Fatalf("phase = %v, want PhaseDecision", m.game.Phase())
	}
}

// TestPairPlusOnlySkipsDecision verifies that a Pair Plus-only hand (no Ante)
// settles at the deal and the UI routes straight from the deal cascade into the
// dealer reveal, never entering the decision phase.
func TestPairPlusOnlySkipsDecision(t *testing.T) {
	m := newTestModel(1000, 0, 6, scenarioBigWin())
	m, _ = upd(t, m, keyEnter) // deal
	m = advanceDeal(t, m)
	// Deal finished into a settled round -> straight to the dealer reveal.
	if m.animState != animDealerReveal {
		t.Fatalf("after deal: animState = %v, want animDealerReveal", m.animState)
	}
	if m.game.Phase() != engine.PhaseRoundOver {
		t.Fatalf("phase = %v, want PhaseRoundOver (no decision)", m.game.Phase())
	}
	m = advanceDealer(t, m)
	if m.animState != animResult {
		t.Fatalf("after dealer reveal: animState = %v, want animResult", m.animState)
	}
	m = settleResult(t, m)
	if m.animState != animIdle {
		t.Fatalf("final animState = %v, want animIdle", m.animState)
	}
	// Pair Plus straight flush 40:1 on $6 = +$240.
	if got := m.game.Bankroll(); got != 1240 {
		t.Fatalf("bankroll = %d, want 1240 (+$240)", got)
	}
}

// TestPlayWinConverges walks the documented big-win scenario through Play, the
// dealer reveal, and the result count-up, asserting the bankroll converges to the
// settled value and the settlement breakdown names every component.
func TestPlayWinConverges(t *testing.T) {
	m := newTestModel(1000, 9, 6, scenarioBigWin())
	m, _ = upd(t, m, keyEnter)
	m = advanceDeal(t, m)
	if m.game.Phase() != engine.PhaseDecision {
		t.Fatalf("phase = %v, want PhaseDecision", m.game.Phase())
	}

	// Select Play and confirm.
	for i, n := 0, playIndex(t, m); i < n; i++ {
		m, _ = upd(t, m, keyRight)
	}
	m, cmd := upd(t, m, keyEnter)
	if m.animState != animDealerReveal {
		t.Fatalf("after play: animState = %v, want animDealerReveal", m.animState)
	}
	if cmd == nil {
		t.Fatal("play-into-round-over should return a tick command")
	}

	m = advanceDealer(t, m)
	if m.animState != animResult {
		t.Fatalf("after dealer reveal: animState = %v, want animResult", m.animState)
	}
	target := m.game.Bankroll()
	if target != 1294 {
		t.Fatalf("settled bankroll = %d, want 1294 (+$294)", target)
	}
	if m.bankFrom != 1000 {
		t.Fatalf("bankFrom = %d, want 1000", m.bankFrom)
	}

	last := m.displayBankroll
	for m.animState == animResult && m.bankStep < bankrollSteps {
		m, _ = upd(t, m, bankrollTickMsg{})
		if m.displayBankroll < last || m.displayBankroll > target {
			t.Fatalf("displayBankroll out of range: %d (last %d, target %d)", m.displayBankroll, last, target)
		}
		last = m.displayBankroll
	}
	if m.displayBankroll != target {
		t.Fatalf("displayBankroll = %d after count-up, want %d", m.displayBankroll, target)
	}

	m, _ = upd(t, m, resultDoneMsg{})
	if m.animState != animIdle {
		t.Fatalf("after result hold: animState = %v, want animIdle", m.animState)
	}

	// Settled frame: dealer fully revealed, banner + breakdown present.
	f := m.frame()
	if f.dealerFaceUp != handSize || !f.showOutcomes || !f.showBanner || !f.showControls {
		t.Fatalf("idle round-over frame = %+v", f)
	}
	if got := stripANSI(m.renderBanner()); !strings.Contains(got, "YOU WIN +$294") {
		t.Fatalf("banner = %q, want it to contain YOU WIN +$294", got)
	}
	breakdown := stripANSI(m.renderSettlement())
	for _, want := range []string{"Dealer doesn't qualify", "Ante", "Play", "Ante Bonus (Straight Flush)", "Pair Plus", "Net +$294"} {
		if !strings.Contains(breakdown, want) {
			t.Fatalf("settlement breakdown %q missing %q", breakdown, want)
		}
	}
}

// TestFoldForfeitsAnte checks the Fold path settles the round (Ante lost) and the
// breakdown reports the fold.
func TestFoldForfeitsAnte(t *testing.T) {
	m := newTestModel(1000, 9, 0, scenarioBigWin())
	m, _ = upd(t, m, keyEnter)
	m = advanceDeal(t, m)
	m, _ = upd(t, m, keyRune('f')) // fold hotkey
	if m.game.Phase() != engine.PhaseRoundOver {
		t.Fatalf("after fold: phase = %v, want PhaseRoundOver", m.game.Phase())
	}
	m = advanceDealer(t, m)
	m = settleResult(t, m)
	if got := m.game.Bankroll(); got != 991 {
		t.Fatalf("bankroll = %d, want 991 (Ante $9 forfeited)", got)
	}
	if got := stripANSI(m.renderSettlement()); !strings.Contains(got, "Ante forfeited") {
		t.Fatalf("settlement = %q, want it to mention Ante forfeited", got)
	}
}

// TestInputGatedDuringAnimation verifies that, while an animation runs, keys are
// ignored and the animation does not advance.
func TestInputGatedDuringAnimation(t *testing.T) {
	m := newTestModel(1000, 9, 6, scenarioBigWin())
	m, _ = upd(t, m, keyEnter) // deal -> animDealReveal
	if m.animState != animDealReveal {
		t.Fatalf("animState = %v, want animDealReveal", m.animState)
	}
	before := m
	m, _ = upd(t, m, keyEnter) // a normal key must be swallowed
	if m.animState != before.animState || m.dealShown != before.dealShown {
		t.Fatalf("gated key advanced animation")
	}
}
