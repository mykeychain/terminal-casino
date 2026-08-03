package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/blackjack32/engine"
)

// animState is the UI's presentation sub-state, independent of the engine Phase.
// The engine resolves a round instantly; animState controls how much of that
// already-computed result has become visible on screen. While it is anything
// other than animIdle an animation is playing and player input (except q /
// ctrl+c) is ignored.
type animState int

const (
	// animIdle: no animation. The View renders straight from engine state, as
	// the UI did before the feel tier existed.
	animIdle animState = iota
	// animDealReveal: the initial deal is cascading in, one card per dealBeat.
	animDealReveal
	// animDealerReveal: the hole card flips and the dealer's drawn cards are
	// revealed one at a time (suspense).
	animDealerReveal
	// animResult: the outcome banner is up and the bankroll is ticking toward
	// its settled value.
	animResult
)

// Timing constants for the feel tier. All are tunable here in one place.
const (
	dealBeat     = 300 * time.Millisecond  // per card during the initial deal (2x original)
	dealerBeat   = 1050 * time.Millisecond // per card as the dealer draws (suspense)
	flipPause    = 900 * time.Millisecond  // hole-card reveal beat before dealer draws
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
	dealerTickMsg   struct{} // flip the hole card, then reveal a dealer draw
	bankrollTickMsg struct{} // advance the bankroll count-up
	resultDoneMsg   struct{} // resultHold elapsed; unlock controls
)

// tick schedules msg to be delivered after d.
func tick(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}

// ---- Transitions between effects ----

// startDealReveal snapshots the pre-deal bankroll (so the header does not drop
// when the stake is escrowed) and begins the deal cascade with the first card
// already visible.
func (m Model) startDealReveal(preDealBankroll int) (Model, tea.Cmd) {
	m.displayBankroll = preDealBankroll
	m.animState = animDealReveal
	m.dealShown = 1 // first card (spot 0's first card) is shown immediately
	m.dealerShown = 0
	m.holeFlipped = false
	return m, tick(dealBeat, dealTickMsg{})
}

// startDealerReveal begins the dealer's reveal: the up-card is already face up
// and the hole card sits face down. The first tick (after flipPause) flips it.
func (m Model) startDealerReveal() (Model, tea.Cmd) {
	m.animState = animDealerReveal
	m.dealerShown = 1 // only the up-card is face up; the hole is still a back
	m.holeFlipped = false
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

// afterAction is called after any engine action that might settle the round
// (Insurance, Hit, Stand, Double, Split). If the round is now over, the dealer
// reveal begins; otherwise play stays instant and control returns to the player.
func (m Model) afterAction() (Model, tea.Cmd) {
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
	n := len(m.game.Player())
	total := dealPositions(n)
	if m.dealShown < total {
		return m, tick(dealBeat, dealTickMsg{})
	}
	// The full deal is on the table. Route by the engine's phase.
	switch m.game.Phase() {
	case engine.PhaseRoundOver:
		// Dealer blackjack or a player natural already settled the round.
		return m.startDealerReveal()
	default:
		// PhaseInsurance (prompt) or PhasePlayerTurn (player acts).
		m.animState = animIdle
		return m, nil
	}
}

func (m Model) onDealerTick() (Model, tea.Cmd) {
	if m.animState != animDealerReveal {
		return m, nil
	}
	m.dealerShown++ // first tick flips the hole card; later ticks reveal draws
	m.holeFlipped = true
	total := len(m.game.Dealer().Cards)
	if m.dealerShown < total {
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

// dealPositions is the number of reveal steps in the initial deal for n spots:
// each spot's first card, the dealer up-card, each spot's second card, and the
// dealer hole card (revealed as a face-down back) — 2n + 2 in all.
func dealPositions(n int) int { return 2*n + 2 }

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
	dealerCards  int   // number of dealer card slots to draw
	dealerFaceUp int   // slots [0,dealerFaceUp) are face up; the rest are backs
	playerCards  []int // per player-hand, how many cards to draw
	showOutcomes bool  // per-hand outcome text + round-net line
	showBanner   bool  // centered result headline
	showControls bool  // action bar / prompt
}

// frame derives the current revealFrame from animState, the reveal counters and
// engine state.
func (m Model) frame() revealFrame {
	dv := m.game.Dealer()
	hands := m.game.Player()
	n := len(hands)

	switch m.animState {
	case animDealReveal:
		f := revealFrame{playerCards: make([]int, n)}
		for i := range hands {
			// Deal order positions: first cards 0..n-1, dealer up = n,
			// second cards n+1..2n, dealer hole = 2n+1.
			if m.dealShown > i {
				f.playerCards[i] = 1
			}
			if m.dealShown > n+1+i {
				f.playerCards[i] = 2
			}
		}
		if m.dealShown > n { // dealer up-card is on the table
			f.dealerCards = 1
			f.dealerFaceUp = 1
		}
		if m.dealShown > 2*n+1 { // dealer hole placed (as a back)
			f.dealerCards = 2
		}
		return f

	case animDealerReveal:
		// Show only the up-card + hole (2 slots) plus each draw as it is revealed,
		// so drawn cards appear one at a time instead of sitting as face-down backs
		// that telegraph how many the dealer will draw.
		return revealFrame{
			dealerCards:  max(m.dealerShown, 2),
			dealerFaceUp: m.dealerShown, // 1 (up-card), then 2 after the flip, then draws
			playerCards:  fullCounts(hands),
		}

	case animResult:
		return revealFrame{
			dealerCards:  len(dv.Cards),
			dealerFaceUp: len(dv.Cards),
			playerCards:  fullCounts(hands),
			showOutcomes: true,
			showBanner:   true,
		}

	default: // animIdle: render straight from engine state (pre-feel-tier behavior)
		f := revealFrame{
			dealerCards:  len(dv.Cards),
			playerCards:  fullCounts(hands),
			showControls: true,
		}
		if dv.Revealed {
			f.dealerFaceUp = len(dv.Cards)
		} else if len(dv.Cards) > 0 {
			f.dealerFaceUp = 1
		}
		roundOver := m.game.Phase() == engine.PhaseRoundOver
		f.showOutcomes = roundOver
		f.showBanner = roundOver
		return f
	}
}

// fullCounts returns each hand's full card count (everything visible).
func fullCounts(hands []engine.HandView) []int {
	out := make([]int, len(hands))
	for i, h := range hands {
		out[i] = len(h.Cards)
	}
	return out
}

// renderBanner builds the centered result headline for the settled round, styled
// with the existing win/lose/push/blackjack colors. Single-hand rounds get a
// per-outcome headline; multi-hand rounds get a net-oriented one.
func (m Model) renderBanner() string {
	hands := m.game.Player()
	if len(hands) == 0 {
		return ""
	}

	var text string
	var style lipgloss.Style
	var win bool
	if len(hands) == 1 {
		h := hands[0]
		switch h.Outcome {
		case engine.OutcomeBlackjack:
			text, style, win = fmt.Sprintf("BLACKJACK! +$%d", h.Net), blackjackStyle, true
		case engine.OutcomeWin:
			text, style, win = fmt.Sprintf("WIN +$%d", h.Net), winStyle, true
		case engine.OutcomePush:
			text, style = "PUSH", pushStyle
		case engine.OutcomeLose:
			if h.Bust {
				text, style = "BUST", loseStyle
			} else {
				text, style = "DEALER WINS", loseStyle
			}
		default:
			return ""
		}
	} else {
		net := 0
		for _, h := range hands {
			net += h.Net
		}
		switch {
		case net > 0:
			text, style, win = fmt.Sprintf("YOU WIN +$%d", net), winStyle, true
		case net < 0:
			text, style = fmt.Sprintf("YOU LOSE -$%d", -net), loseStyle
		default:
			text, style = "PUSH", pushStyle
		}
	}

	// base keeps the outcome color (no blink) for the bankroll line below.
	base := style
	// Blink the headline on a win.
	if win {
		style = style.Blink(true)
	}
	// Two indented lines (aligned with the hand rows): the outcome, then the
	// player's bankroll — value only, no label — which ticks during animResult.
	outcome := handIndent + style.Render(text)
	bankroll := handIndent + base.Render(fmt.Sprintf("$%d", m.displayBankroll))
	return outcome + "\n" + bankroll
}
