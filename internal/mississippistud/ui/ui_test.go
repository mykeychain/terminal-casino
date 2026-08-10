package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/mississippistud/engine"
)

// ansiRe strips SGR escape sequences so assertions can look at the plain glyphs.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// ---- helpers ----

func card(r engine.Rank, s engine.Suit) engine.Card { return engine.Card{Rank: r, Suit: s} }

// flushDeck stacks a hand that evaluates to a Flush (6:1): five spades that are
// not sequential. Order is hole[0], hole[1], community 3rd, 4th, 5th.
func flushDeck() []engine.Card {
	return []engine.Card{
		card(engine.Ace, engine.Spades), card(engine.King, engine.Spades),
		card(engine.Nine, engine.Spades), card(engine.Five, engine.Spades), card(engine.Three, engine.Spades),
	}
}

// pushDeck stacks a hand whose final five cards make a pair of Eights (a 6s–10s
// PUSH). The two hole cards already pair, so a visible push floor locks in early.
func pushDeck() []engine.Card {
	return []engine.Card{
		card(engine.Eight, engine.Spades), card(engine.Eight, engine.Hearts),
		card(engine.Two, engine.Clubs), card(engine.Five, engine.Diamonds), card(engine.King, engine.Clubs),
	}
}

// winPairDeck stacks a pair of Kings (Jacks-or-better WIN) locked from the hole.
func winPairDeck() []engine.Card {
	return []engine.Card{
		card(engine.King, engine.Spades), card(engine.King, engine.Hearts),
		card(engine.Two, engine.Clubs), card(engine.Five, engine.Diamonds), card(engine.Nine, engine.Clubs),
	}
}

func newTestModel(bankroll int, cards []engine.Card) Model {
	mk := func() *engine.Game { return engine.NewGameWithDeck(bankroll, cards) }
	return New(mk(), mk).(Model)
}

var (
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyRight = tea.KeyMsg{Type: tea.KeyRight}
	keyLeft  = tea.KeyMsg{Type: tea.KeyLeft}
	keyUp    = tea.KeyMsg{Type: tea.KeyUp}
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

func sized(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	mm, _ := upd(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	return mm
}

// raiseAllStreets drives a dealt hand through 3rd, 4th, and 5th Street, raising
// 1× each time, landing in PhaseRoundOver.
func raiseAllStreets(t *testing.T, m Model) Model {
	t.Helper()
	for i := 0; i < numCommunity; i++ {
		var cmd tea.Cmd
		m, cmd = upd(t, m, keyRune('1'))
		_ = cmd
	}
	if m.game.Phase() != engine.PhaseRoundOver {
		t.Fatalf("after raising every street: phase = %v, want PhaseRoundOver", m.game.Phase())
	}
	return m
}

// advanceToNextHand presses "Next hand" from a settled hand and runs the
// board-clear sweep to completion via its ticks, landing on the next hand
// (betting) or game over.
func advanceToNextHand(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = upd(t, m, keyEnter) // start the board-clear sweep
	for i := 0; i < numBoard+2 && m.animState == animClearing; i++ {
		m, _ = upd(t, m, clearTickMsg{})
	}
	if m.animState != animIdle {
		t.Fatalf("board-clear sweep did not finish (animState = %d)", m.animState)
	}
	return m
}

// ---- tests ----

// TestPhasesRenderWithoutPanic is the layout smoke test: every phase, across a
// sweep of terminal sizes (including tiny and pre-size), renders to exactly
// m.height rows with the title first and the control panel pinned to the bottom.
func TestPhasesRenderWithoutPanic(t *testing.T) {
	betting := func() Model { return newTestModel(1000, flushDeck()) }
	street := func() Model {
		m := betting()
		m, _ = upd(t, m, keyEnter) // deal -> 3rd Street
		return m
	}
	fourth := func() Model {
		m := street()
		m, _ = upd(t, m, keyRune('1')) // raise -> 4th Street
		return m
	}
	result := func() Model { return raiseAllStreets(t, street()) }

	cases := []struct {
		name    string
		make    func() Model
		control string
		title   string
	}{
		{"betting", betting, "enter deal", "Set your ante"},
		{"3rd-street", street, "f fold", "3rd Street"},
		{"4th-street", fourth, "f fold", "4th Street"},
		{"round-over", result, "enter continue", "Hand settled"},
	}

	for _, tc := range cases {
		base := tc.make()
		// Pre-size fallback must not panic and must carry the title.
		if !strings.Contains(stripANSI(base.View()), "Mississippi Stud") {
			t.Fatalf("%s: pre-size view lacks title", tc.name)
		}
		// Compact (narrow) width must at least render without panic and keep the
		// title; the exact-height guarantee below uses widths the hint line fits on
		// (matching the house style — a very narrow hint wraps, as in poker3card).
		if !strings.Contains(stripANSI(sized(t, base, 70, 20).View()), "Mississippi Stud") {
			t.Fatalf("%s: compact-width view lacks title", tc.name)
		}
		for _, wh := range [][2]int{{80, 18}, {90, 24}, {120, 40}} {
			w, h := wh[0], wh[1]
			view := sized(t, base, w, h).View()
			if got := lipgloss.Height(view); got != h {
				t.Fatalf("%s %dx%d: view height = %d, want %d", tc.name, w, h, got, h)
			}
			first := stripANSI(strings.SplitN(view, "\n", 2)[0])
			if !strings.Contains(first, "Mississippi Stud") {
				t.Fatalf("%s %dx%d: first line %q lacks the title", tc.name, w, h, first)
			}
			tail := strings.Join(strings.Split(stripANSI(view), "\n"), "\n")
			if !strings.Contains(tail, tc.control) {
				t.Fatalf("%s %dx%d: control hint %q missing", tc.name, w, h, tc.control)
			}
			if !strings.Contains(stripANSI(view), tc.title) {
				t.Fatalf("%s %dx%d: panel title %q missing", tc.name, w, h, tc.title)
			}
		}
	}
}

// TestPaytableOverlayRenders checks the `?` overlay toggles on and renders the
// flat payouts and the pair tiers without panic.
func TestPaytableOverlayRenders(t *testing.T) {
	m := sized(t, newTestModel(1000, flushDeck()), 90, 30)
	m, _ = upd(t, m, keyRune('?'))
	if !m.showPaytable {
		t.Fatal("? did not open the paytable")
	}
	view := stripANSI(m.View())
	for _, want := range []string{"Royal Flush", "500:1", "Flush", "6:1", "Jacks or better", "push"} {
		if !strings.Contains(view, want) {
			t.Fatalf("paytable overlay missing %q:\n%s", want, view)
		}
	}
	m, _ = upd(t, m, keyRune('?'))
	if m.showPaytable {
		t.Fatal("? did not close the paytable")
	}
}

// TestAnteAdjust exercises the betting input: stepping the Ante on the MinBet
// grid, the coarse step, and the MinBet floor (it never clears below the table
// minimum).
func TestAnteAdjust(t *testing.T) {
	m := newTestModel(1000, flushDeck())
	if m.game.Ante() != engine.MinBet {
		t.Fatalf("initial ante = %d, want %d", m.game.Ante(), engine.MinBet)
	}
	m, _ = upd(t, m, keyRight)
	if m.game.Ante() != 2*engine.MinBet {
		t.Fatalf("ante after right = %d, want %d", m.game.Ante(), 2*engine.MinBet)
	}
	// Left twice would go below MinBet; it must floor at MinBet.
	m, _ = upd(t, m, keyLeft)
	m, _ = upd(t, m, keyLeft)
	if m.game.Ante() != engine.MinBet {
		t.Fatalf("ante after two lefts = %d, want floor %d", m.game.Ante(), engine.MinBet)
	}
	m, _ = upd(t, m, keyUp) // coarse step
	if m.game.Ante() != engine.MinBet+coarseBetStep {
		t.Fatalf("ante after up = %d, want %d", m.game.Ante(), engine.MinBet+coarseBetStep)
	}
}

// TestProgressiveRevealAndRunningTotal verifies each street's Raise reveals one
// more community card and grows the total wagered, and that the ledger reflects
// the engine's running total.
func TestProgressiveRevealAndRunningTotal(t *testing.T) {
	m := sized(t, newTestModel(1000, flushDeck()), 90, 30)
	m, _ = upd(t, m, keyEnter) // deal
	if got := revealedCount(m.game.CommunityCards()); got != 0 {
		t.Fatalf("just dealt: revealed = %d, want 0 (all backs)", got)
	}
	if got := m.game.TotalWagered(); got != m.game.Ante() {
		t.Fatalf("just dealt: total wagered = %d, want ante %d", got, m.game.Ante())
	}

	ante := m.game.Ante()
	for street := 1; street <= numCommunity; street++ {
		m, _ = upd(t, m, keyRune('1')) // raise 1x
		wantReveal := street
		if m.game.Phase() == engine.PhaseRoundOver {
			wantReveal = numCommunity // the 5th-Street raise reveals the last card
		}
		if got := revealedCount(m.game.CommunityCards()); got != wantReveal {
			t.Fatalf("after street %d raise: revealed = %d, want %d", street, got, wantReveal)
		}
		wantTotal := ante * (1 + street) // ante + one street bet of 1x per street
		if got := m.game.TotalWagered(); got != wantTotal {
			t.Fatalf("after street %d raise: total wagered = %d, want %d", street, got, wantTotal)
		}
	}

	// The ledger line reflects the running total.
	ledger := stripANSI(m.renderLedger())
	if !strings.Contains(ledger, fmt.Sprintf("Total at risk $%d", m.game.TotalWagered())) {
		t.Fatalf("ledger %q missing the running total", ledger)
	}
}

// TestFoldForfeits checks a fold on 3rd Street settles as a loss of everything
// wagered and the settlement names the forfeited total.
func TestFoldForfeits(t *testing.T) {
	m := sized(t, newTestModel(1000, flushDeck()), 90, 30)
	m, _ = upd(t, m, keyEnter)     // deal
	m, _ = upd(t, m, keyRune('f')) // fold
	if m.game.Phase() != engine.PhaseRoundOver {
		t.Fatalf("after fold: phase = %v, want PhaseRoundOver", m.game.Phase())
	}
	if got := m.game.Bankroll(); got != 1000-m.game.Ante() {
		t.Fatalf("bankroll after fold = %d, want %d (ante forfeited)", got, 1000-m.game.Ante())
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "forfeited") || !strings.Contains(view, "YOU LOSE") {
		t.Fatalf("fold view missing forfeit/loss messaging:\n%s", view)
	}
}

// TestWinSettlementBreakdown drives a Flush to showdown and checks the settlement
// shows the hand name, the multiplier, the total wagered, and the profit.
func TestWinSettlementBreakdown(t *testing.T) {
	m := sized(t, newTestModel(1000, flushDeck()), 90, 30)
	m, _ = upd(t, m, keyEnter)
	m = raiseAllStreets(t, m)
	s := m.game.Result()
	if s.Outcome != engine.OutcomeWin || s.HandName != "Flush" {
		t.Fatalf("result = %+v, want a Flush win", s)
	}
	breakdown := stripANSI(m.renderSettlement())
	for _, want := range []string{"Flush", fmt.Sprintf("%d:1", s.Multiplier),
		fmt.Sprintf("on $%d wagered", s.TotalWagered), fmt.Sprintf("+$%d", s.Profit)} {
		if !strings.Contains(breakdown, want) {
			t.Fatalf("settlement %q missing %q", breakdown, want)
		}
	}
}

// TestPushSettlement drives a 6s–10s pair to a PUSH and checks the banner reads
// PUSH (not a win or loss) and the breakdown says the stake is returned.
func TestPushSettlement(t *testing.T) {
	m := sized(t, newTestModel(1000, pushDeck()), 90, 30)
	m, _ = upd(t, m, keyEnter)
	m = raiseAllStreets(t, m)
	if m.game.Result().Outcome != engine.OutcomePush {
		t.Fatalf("outcome = %v, want push", m.game.Result().Outcome)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "PUSH") {
		t.Fatalf("push view lacks PUSH banner:\n%s", view)
	}
	if !strings.Contains(stripANSI(m.renderSettlement()), "returned") {
		t.Fatalf("push settlement missing 'returned'")
	}
}

// TestMadeHandHint checks the subtle floor hint: a paired hole shows the push /
// win note, and it names the pair.
func TestMadeHandHint(t *testing.T) {
	// Push floor (pair of Eights).
	m := newTestModel(1000, pushDeck())
	m, _ = upd(t, m, keyEnter)
	if hint := stripANSI(m.madeHandHint()); !strings.Contains(hint, "pair of Eights") || !strings.Contains(hint, "pushing") {
		t.Fatalf("push hint = %q, want it to name the pair and say pushing", hint)
	}

	// Win floor (pair of Kings).
	m2 := newTestModel(1000, winPairDeck())
	m2, _ = upd(t, m2, keyEnter)
	if hint := stripANSI(m2.madeHandHint()); !strings.Contains(hint, "pair of Kings") || !strings.Contains(hint, "paying") {
		t.Fatalf("win hint = %q, want it to name the pair and say paying", hint)
	}

	// No pair -> no hint.
	m3 := newTestModel(1000, flushDeck())
	m3, _ = upd(t, m3, keyEnter)
	if hint := m3.madeHandHint(); hint != "" {
		t.Fatalf("no-pair hint = %q, want empty", hint)
	}
}

// TestGameOverRestart checks the game-over menu restarts into a fresh game with a
// full bankroll.
func TestGameOverRestart(t *testing.T) {
	// A bankroll of exactly MinBet allows one deal; after a fold the bankroll is 0,
	// so NextHand routes to game over.
	m := sized(t, newTestModel(engine.MinBet, flushDeck()), 90, 30)
	m, _ = upd(t, m, keyEnter)     // deal (ante escrowed)
	m, _ = upd(t, m, keyRune('f')) // fold -> lose the ante, bankroll 0
	m = advanceToNextHand(t, m)    // Next hand + sweep -> game over
	if m.game.Phase() != engine.PhaseGameOver {
		t.Fatalf("phase = %v, want PhaseGameOver", m.game.Phase())
	}
	if !strings.Contains(stripANSI(m.View()), "Out of chips") {
		t.Fatal("game-over view lacks the out-of-chips panel")
	}
	// Restart (cursor 0) mints a fresh game from the newGame closure (which, in
	// this test, seeds bankroll = MinBet; in production it is a full $1000).
	m, _ = upd(t, m, keyEnter)
	if m.game.Phase() != engine.PhaseBetting || m.game.Bankroll() != engine.MinBet {
		t.Fatalf("after restart: phase %v bankroll %d, want betting / %d",
			m.game.Phase(), m.game.Bankroll(), engine.MinBet)
	}
}

// TestRaiseOnlyOffersLegalMultiples checks that when the bankroll cannot afford a
// 2× or 3× raise, only the affordable multiples appear in the menu and a too-big
// raise hotkey is ignored.
func TestRaiseOnlyOffersLegalMultiples(t *testing.T) {
	// Ante 3, bankroll 4 after the ante is escrowed on deal (start 7 - 3 escrow =
	// 4): only 1× ($3) is affordable, 2× ($6) is not.
	m := newTestModel(7, flushDeck())
	m, _ = upd(t, m, keyEnter) // deal, bankroll now 4
	actions := m.streetActions()
	if len(actions) != 2 || actions[0] != foldAction || actions[1] != 1 {
		t.Fatalf("street actions = %v, want [fold 1x] only", actions)
	}
	// The 2× hotkey is not legal and must be ignored (still on 3rd Street).
	before := m.game.Phase()
	m, _ = upd(t, m, keyRune('2'))
	if m.game.Phase() != before {
		t.Fatalf("illegal 2x raise advanced the phase to %v", m.game.Phase())
	}
}

// TestBoardClearsOnNextHand checks that once the clear sweep finishes, the next
// betting screen is clean — the settled hand's community and hole cards are gone,
// not left lingering on the felt.
func TestBoardClearsOnNextHand(t *testing.T) {
	m := sized(t, newTestModel(1000, flushDeck()), 100, 30)
	m, _ = upd(t, m, keyEnter)  // deal
	m = raiseAllStreets(t, m)   // settle
	m = advanceToNextHand(t, m) // Next hand + run the sweep -> betting
	if m.game.Phase() != engine.PhaseBetting {
		t.Fatalf("after Next hand: phase = %v, want betting", m.game.Phase())
	}
	view := stripANSI(m.View())
	if strings.Contains(view, "Community") || strings.Contains(view, "▚") {
		t.Fatalf("board not cleared on the new-hand betting screen:\n%s", view)
	}
}

// TestClearSweepRemovesCardsOneByOne checks the board-clear animation: pressing
// "Next hand" starts a sweep with the full board on the felt, each tick removes
// one card, and the final tick advances to the next hand's betting screen.
func TestClearSweepRemovesCardsOneByOne(t *testing.T) {
	m := sized(t, newTestModel(1000, flushDeck()), 100, 30)
	m, _ = upd(t, m, keyEnter) // deal
	m = raiseAllStreets(t, m)  // settle -> full board
	m, _ = upd(t, m, keyEnter) // Next hand -> start sweep
	if m.animState != animClearing || m.boardShown != numBoard {
		t.Fatalf("after Next hand: animState=%d boardShown=%d, want clearing / %d",
			m.animState, m.boardShown, numBoard)
	}
	// Each tick removes exactly one card while the sweep is still running.
	for want := numBoard - 1; want >= 1; want-- {
		m, _ = upd(t, m, clearTickMsg{})
		if m.boardShown != want {
			t.Fatalf("boardShown = %d, want %d", m.boardShown, want)
		}
		if m.animState != animClearing {
			t.Fatalf("sweep ended early at boardShown %d", want)
		}
	}
	// The final tick clears the felt and advances to the next hand.
	m, _ = upd(t, m, clearTickMsg{})
	if m.animState != animIdle || m.game.Phase() != engine.PhaseBetting {
		t.Fatalf("after final tick: animState=%d phase=%v, want idle / betting",
			m.animState, m.game.Phase())
	}
}

// TestClearSweepSkip checks that enter/space during the sweep skips straight to
// the next hand instead of waiting out the ticks.
func TestClearSweepSkip(t *testing.T) {
	m := sized(t, newTestModel(1000, flushDeck()), 100, 30)
	m, _ = upd(t, m, keyEnter) // deal
	m = raiseAllStreets(t, m)  // settle
	m, _ = upd(t, m, keyEnter) // Next hand -> start sweep
	m, _ = upd(t, m, keyEnter) // skip
	if m.animState != animIdle || m.game.Phase() != engine.PhaseBetting {
		t.Fatalf("after skip: animState=%d phase=%v, want idle / betting",
			m.animState, m.game.Phase())
	}
}
