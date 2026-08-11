package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/blackjackfreebet/engine"
)

// ---- helpers ----

func stack(rs ...engine.Rank) []engine.Card {
	out := make([]engine.Card, len(rs))
	for i, r := range rs {
		out[i] = engine.Card{Rank: r}
	}
	return out
}

// newModel builds a Model over a stacked-shoe game for deterministic play. The
// deck is drawn in slice order; deal order is spot-0 card, dealer up, spot-0
// card, dealer hole, then dealer draws.
func newModel(bankroll int, rs ...engine.Rank) Model {
	mk := func() *engine.Game { return engine.NewGameWithShoe(bankroll, stack(rs...)) }
	return New(mk(), mk)
}

var (
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyRight = tea.KeyMsg{Type: tea.KeyRight}
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

// settleResult finishes the bankroll count-up and then lets the hold elapse.
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

// standIndex returns how many "right" presses select Stand in the player menu.
func standIndex(t *testing.T, m Model) int {
	t.Helper()
	for i, a := range m.playerActions() {
		if a == engine.ActionStand {
			return i
		}
	}
	t.Fatal("Stand is not a legal action")
	return 0
}

// ---- tests ----

// TestInputGatedDuringAnimation verifies that, while an animation runs, keys are
// ignored: the animation state does not advance and the engine phase is unchanged.
func TestInputGatedDuringAnimation(t *testing.T) {
	m := newModel(1000, engine.Ten, engine.Six, engine.Nine, engine.Ten, engine.Ten)
	m, cmd := upd(t, m, keyEnter) // deal
	if m.animState != animDealReveal {
		t.Fatalf("after deal: animState = %v, want animDealReveal", m.animState)
	}
	if cmd == nil {
		t.Fatal("deal should return a tick command")
	}

	before := m
	// A normal key must be swallowed: no counter movement, no phase change.
	m, _ = upd(t, m, keyEnter)
	if m.animState != before.animState || m.dealShown != before.dealShown {
		t.Fatalf("key advanced animation: state %v->%v, dealShown %d->%d",
			before.animState, m.animState, before.dealShown, m.dealShown)
	}
	if m.game.Phase() != before.game.Phase() {
		t.Fatal("gated key changed engine phase")
	}
}

// TestDealRevealAdvancesToIdle checks the deal cascade advances one position per
// tick and lands idle at the player's turn with all cards shown.
func TestDealRevealAdvancesToIdle(t *testing.T) {
	m := newModel(1000, engine.Ten, engine.Six, engine.Nine, engine.Ten, engine.Ten)
	m, _ = upd(t, m, keyEnter) // deal

	// Escrow is masked: header value stays at the pre-deal bankroll.
	if m.displayBankroll != 1000 {
		t.Fatalf("displayBankroll = %d during deal, want 1000 (pre-deal)", m.displayBankroll)
	}

	n := len(m.game.Player())
	total := dealPositions(n)
	if m.dealShown != 1 {
		t.Fatalf("initial dealShown = %d, want 1", m.dealShown)
	}
	// Feed ticks; each should advance dealShown by exactly one until complete.
	prev := m.dealShown
	for m.animState == animDealReveal {
		m, _ = upd(t, m, dealTickMsg{})
		if m.animState == animDealReveal && m.dealShown != prev+1 {
			t.Fatalf("dealShown jumped %d -> %d", prev, m.dealShown)
		}
		prev = m.dealShown
	}
	if m.dealShown < total {
		t.Fatalf("deal ended at dealShown=%d, want >= %d", m.dealShown, total)
	}
	if m.animState != animIdle {
		t.Fatalf("after deal reveal: animState = %v, want animIdle", m.animState)
	}
	if m.game.Phase() != engine.PhasePlayerTurn {
		t.Fatalf("phase = %v, want PhasePlayerTurn", m.game.Phase())
	}
	// Full final deal is on the table: both player cards + dealer up-card + hole.
	f := m.frame()
	if f.playerCards[0] != 2 {
		t.Fatalf("playerCards[0] = %d, want 2", f.playerCards[0])
	}
	if f.dealerCards != 2 || f.dealerFaceUp != 1 {
		t.Fatalf("dealer frame = (cards %d, faceUp %d), want (2,1)", f.dealerCards, f.dealerFaceUp)
	}
}

// TestSettleWinAnimatesAndConverges walks a full stand-and-win round through the
// dealer reveal and result effects, and asserts displayBankroll converges to the
// engine's settled bankroll and the machine returns to idle showing everything.
func TestSettleWinAnimatesAndConverges(t *testing.T) {
	// Player 10+9=19 stands; dealer 6+10=16 draws 10 -> 26 bust; player wins.
	m := newModel(1000, engine.Ten, engine.Six, engine.Nine, engine.Ten, engine.Ten)
	m, _ = upd(t, m, keyEnter)
	m = advanceDeal(t, m)

	// Select Stand and confirm.
	for i, n := 0, standIndex(t, m); i < n; i++ {
		m, _ = upd(t, m, keyRight)
	}
	m, cmd := upd(t, m, keyEnter)
	if m.animState != animDealerReveal {
		t.Fatalf("after stand: animState = %v, want animDealerReveal", m.animState)
	}
	if cmd == nil {
		t.Fatal("stand-into-round-over should return a tick command")
	}
	if m.game.Phase() != engine.PhaseRoundOver {
		t.Fatalf("phase = %v, want PhaseRoundOver", m.game.Phase())
	}

	dealerCards := len(m.game.Dealer().Cards)
	if dealerCards != 3 {
		t.Fatalf("dealer has %d cards, want 3 (up, hole, one draw)", dealerCards)
	}
	m = advanceDealer(t, m)
	if m.animState != animResult {
		t.Fatalf("after dealer reveal: animState = %v, want animResult", m.animState)
	}

	target := m.game.Bankroll()
	if target != 1003 {
		t.Fatalf("settled bankroll = %d, want 1003 (won $3)", target)
	}
	// Bankroll count-up is monotonic and converges to the target.
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
	if m.displayBankroll != target {
		t.Fatalf("final displayBankroll = %d, want %d", m.displayBankroll, target)
	}
	// Final state fully shown: dealer entirely face up, outcomes + banner + controls.
	f := m.frame()
	if f.dealerFaceUp != f.dealerCards || f.dealerCards != dealerCards {
		t.Fatalf("dealer not fully revealed: faceUp %d of %d (want %d)", f.dealerFaceUp, f.dealerCards, dealerCards)
	}
	if !f.showOutcomes || !f.showBanner || !f.showControls {
		t.Fatalf("idle round-over frame flags = outcomes:%v banner:%v controls:%v", f.showOutcomes, f.showBanner, f.showControls)
	}
	if got := m.renderBanner(); !strings.Contains(got, "WIN +$3") {
		t.Fatalf("banner = %q, want it to contain WIN +$3", got)
	}
	// The header now renders the settled bankroll.
	if h := m.renderHeader(); !strings.Contains(h, "$1003") {
		t.Fatalf("header = %q, want it to contain $1003", h)
	}
}

// TestDealerBlackjackSettlesAtDeal covers the deal-settles route: a ten up-card
// with an ace hole gives the dealer a natural, so the round is over the moment
// the deal finishes; the UI still flips the hole (no draws) then settles.
func TestDealerBlackjackSettlesAtDeal(t *testing.T) {
	// Player 10+9=19; dealer 10 up, Ace hole = blackjack. Player loses $3.
	m := newModel(1000, engine.Ten, engine.Ten, engine.Nine, engine.Ace)
	m, _ = upd(t, m, keyEnter)
	m = advanceDeal(t, m)

	// Deal finished into a settled round -> straight to the dealer reveal.
	if m.animState != animDealerReveal {
		t.Fatalf("after deal: animState = %v, want animDealerReveal", m.animState)
	}
	if len(m.game.Dealer().Cards) != 2 {
		t.Fatalf("dealer has %d cards, want 2 (no draws on blackjack)", len(m.game.Dealer().Cards))
	}
	m = advanceDealer(t, m)
	if m.animState != animResult {
		t.Fatalf("after flip: animState = %v, want animResult", m.animState)
	}

	target := m.game.Bankroll()
	if target != 997 {
		t.Fatalf("settled bankroll = %d, want 997 (lost $3)", target)
	}
	m = settleResult(t, m)
	if m.animState != animIdle {
		t.Fatalf("final animState = %v, want animIdle", m.animState)
	}
	if m.displayBankroll != target {
		t.Fatalf("final displayBankroll = %d, want %d", m.displayBankroll, target)
	}
	if got := m.renderBanner(); !strings.Contains(got, "DEALER WINS") {
		t.Fatalf("banner = %q, want DEALER WINS", got)
	}
}
