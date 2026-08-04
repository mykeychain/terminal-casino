package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/poker3card/engine"
)

// handSize is the fixed number of cards in a Three-Card Poker hand. Both the
// player and the dealer always hold exactly three.
const handSize = 3

// animState is the UI's presentation sub-state, independent of the engine Phase.
// The engine resolves a round instantly; animState controls how much of that
// already-computed result has become visible on screen. While it is anything
// other than animIdle an animation is playing and player input (except q /
// ctrl+c) is ignored.
type animState int

const (
	// animIdle: no animation. The View renders straight from engine state, so a
	// settled frame is a pure function of the engine (deterministic screenshots).
	animIdle animState = iota
	// animDealReveal: the initial deal is cascading in, one card per dealBeat —
	// the player's cards face up, the dealer's as face-down backs.
	animDealReveal
	// animDealerReveal: the dealer's three cards flip face up one at a time at
	// showdown (suspense).
	animDealerReveal
	// animResult: the outcome banner is up and the bankroll is ticking toward its
	// settled value.
	animResult
)

// Timing constants for the feel tier. All are tunable here in one place; they
// mirror the blackjack table's pacing.
const (
	dealBeat     = 300 * time.Millisecond  // per card during the initial deal
	dealerBeat   = 1050 * time.Millisecond // per card as the dealer's hand flips (suspense)
	flipPause    = 900 * time.Millisecond  // beat before the first dealer card flips
	resultHold   = 1800 * time.Millisecond // banner hold before Next hand unlocks
	bankrollTick = 1350 * time.Millisecond // total duration of the bankroll count-up

	// bankrollSteps is the number of frames the bankroll count animation is
	// divided into; each frame is bankrollTick/bankrollSteps apart.
	bankrollSteps = 9
)

// Typed tick messages that drive each effect. They carry no payload; the Model's
// counters and animState decide what each tick does.
type (
	dealTickMsg     struct{} // advance the initial deal cascade
	dealerTickMsg   struct{} // flip the next dealer card
	bankrollTickMsg struct{} // advance the bankroll count-up
	resultDoneMsg   struct{} // resultHold elapsed; unlock controls
)

// tick schedules msg to be delivered after d.
func tick(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}

// dealPositions is the number of reveal steps in the initial deal: three player
// cards and three dealer backs, dealt interleaved.
func dealPositions() int { return 2 * handSize }

// ---- Transitions between effects ----

// startDealReveal snapshots the pre-deal bankroll (so the header does not drop
// when the stake is escrowed) and begins the deal cascade with the first card
// already visible.
func (m Model) startDealReveal(preDealBankroll int) (Model, tea.Cmd) {
	m.displayBankroll = preDealBankroll
	m.animState = animDealReveal
	m.dealShown = 1 // the player's first card is on the table immediately
	m.dealerShown = 0
	return m, tick(dealBeat, dealTickMsg{})
}

// startDealerReveal begins the showdown: the dealer's three face-down cards flip
// up one at a time. The first tick (after flipPause) flips the first card.
func (m Model) startDealerReveal() (Model, tea.Cmd) {
	m.animState = animDealerReveal
	m.dealerShown = 0
	return m, tick(flipPause, dealerTickMsg{})
}

// startResult raises the outcome banner and begins ticking displayBankroll from
// its held pre-settlement value toward the engine's settled bankroll. Because
// displayBankroll was frozen at the pre-deal value for the whole round, the
// count-up's delta equals the round's net — the same figure the banner shows.
func (m Model) startResult() (Model, tea.Cmd) {
	m.animState = animResult
	m.bankFrom = m.displayBankroll
	m.bankStep = 0
	return m, tea.Batch(
		tick(bankrollTick/bankrollSteps, bankrollTickMsg{}),
		tick(resultHold, resultDoneMsg{}),
	)
}

// afterDecision is called after Play or Fold. Both settle the round, so the
// dealer reveal begins immediately.
func (m Model) afterDecision() (Model, tea.Cmd) {
	if m.game.Phase() == engine.PhaseRoundOver {
		return m.startDealerReveal()
	}
	return m, nil
}

// ---- Tick handlers ----

// updateAnim handles the typed animation messages. It returns handled=false when
// msg is not an animation tick so Update can fall through to its other cases.
func (m Model) updateAnim(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg.(type) {
	case dealTickMsg:
		mm, cmd := m.onDealTick()
		return mm, cmd, true
	case dealerTickMsg:
		mm, cmd := m.onDealerTick()
		return mm, cmd, true
	case bankrollTickMsg:
		mm, cmd := m.onBankrollTick()
		return mm, cmd, true
	case resultDoneMsg:
		mm, cmd := m.onResultDone()
		return mm, cmd, true
	}
	return m, nil, false
}

func (m Model) onDealTick() (Model, tea.Cmd) {
	if m.animState != animDealReveal {
		return m, nil
	}
	m.dealShown++
	if m.dealShown < dealPositions() {
		return m, tick(dealBeat, dealTickMsg{})
	}
	// The full deal is on the table. Route by the engine's phase: a Pair
	// Plus-only hand already settled and goes straight to the showdown; an
	// Ante hand waits for the player's Play/Fold decision.
	switch m.game.Phase() {
	case engine.PhaseRoundOver:
		return m.startDealerReveal()
	default:
		m.animState = animIdle
		return m, nil
	}
}

func (m Model) onDealerTick() (Model, tea.Cmd) {
	if m.animState != animDealerReveal {
		return m, nil
	}
	m.dealerShown++
	if m.dealerShown < handSize {
		return m, tick(dealerBeat, dealerTickMsg{})
	}
	return m.startResult()
}

func (m Model) onBankrollTick() (Model, tea.Cmd) {
	if m.animState != animResult {
		return m, nil
	}
	m.bankStep++
	m.displayBankroll = lerpMoney(m.bankFrom, m.game.Bankroll(), m.bankStep, bankrollSteps)
	if m.bankStep < bankrollSteps {
		return m, tick(bankrollTick/bankrollSteps, bankrollTickMsg{})
	}
	m.displayBankroll = m.game.Bankroll()
	return m, nil
}

func (m Model) onResultDone() (Model, tea.Cmd) {
	if m.animState != animResult {
		return m, nil
	}
	m.animState = animIdle
	m.displayBankroll = m.game.Bankroll()
	return m, nil
}

// lerpMoney interpolates from -> to at step/steps, landing exactly on to at the
// final step.
func lerpMoney(from, to, step, steps int) int {
	if step >= steps {
		return to
	}
	return from + (to-from)*step/steps
}

// ---- Frame: what the View should draw right now ----

// revealFrame is the derived description of the current frame: how much of the
// (already-known) engine state is visible. Every render function reads from it
// so the same code renders both the animated and the settled (idle) views.
type revealFrame struct {
	playerCards  int  // number of player cards to draw (face up)
	dealerCards  int  // number of dealer card slots to draw
	dealerFaceUp int  // slots [0,dealerFaceUp) are face up; the rest are backs
	showOutcomes bool // settlement breakdown + hand names
	showBanner   bool // centered result headline
	showControls bool // action bar / prompt
}

// frame derives the current revealFrame from animState, the reveal counters and
// engine state. In animIdle it is a pure function of the engine, so a settled
// frame renders deterministically.
func (m Model) frame() revealFrame {
	switch m.animState {
	case animDealReveal:
		// Interleaved deal: player card, dealer back, repeated. After d shown
		// positions the player holds ceil(d/2) and the dealer holds d/2 backs.
		d := m.dealShown
		return revealFrame{
			playerCards:  (d + 1) / 2,
			dealerCards:  d / 2,
			dealerFaceUp: 0,
		}

	case animDealerReveal:
		return revealFrame{
			playerCards:  handSize,
			dealerCards:  handSize,
			dealerFaceUp: m.dealerShown,
		}

	case animResult:
		return revealFrame{
			playerCards:  handSize,
			dealerCards:  handSize,
			dealerFaceUp: handSize,
			showOutcomes: true,
			showBanner:   true,
			showControls: true,
		}

	default: // animIdle: render straight from engine state
		if !m.dealt() {
			// Betting / game over: no cards on the felt.
			return revealFrame{showControls: true}
		}
		f := revealFrame{
			playerCards:  handSize,
			dealerCards:  handSize,
			showControls: true,
		}
		if m.game.DealerRevealed() {
			f.dealerFaceUp = handSize
		}
		roundOver := m.game.Phase() == engine.PhaseRoundOver
		f.showOutcomes = roundOver
		f.showBanner = roundOver
		return f
	}
}

// dealt reports whether a hand has been dealt (the player holds cards).
func (m Model) dealt() bool { return len(m.game.Player().Cards) > 0 }
