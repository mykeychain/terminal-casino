package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// animState is the UI's presentation sub-state, independent of the engine Phase.
// Mississippi Stud's play is otherwise animation-free — the reveal is
// player-driven, so a settled frame is a pure function of engine state — and the
// only timed effect is the board clearing at the end of a hand.
type animState int

const (
	// animIdle: no animation. The View renders straight from engine state.
	animIdle animState = iota
	// animClearing: the finished hand's cards are being swept off the felt one at a
	// time before the next hand's betting screen appears. While this runs, player
	// input (except q / ctrl+c, and enter to skip) is ignored.
	animClearing
)

// clearBeat is the per-card pacing of the board-clear sweep. It matches the deal
// pacing of the other tables (dealBeat) so cards leave the felt at the same
// rhythm cards arrive on the others.
const clearBeat = 300 * time.Millisecond

// numBoard is the number of card positions on the felt during a hand: the two
// hole cards plus the three community slots.
const numBoard = 2 + numCommunity

// clearTickMsg advances the board-clear sweep by one card.
type clearTickMsg struct{}

// tick schedules msg to be delivered after d.
func tick(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}

// startClear begins sweeping the settled hand off the felt. The board starts
// fully populated and each tick removes one card; when the felt is clear the
// engine advances to the next hand's betting screen (see finishClear).
func (m Model) startClear() (Model, tea.Cmd) {
	m.animState = animClearing
	m.boardShown = numBoard
	return m, tick(clearBeat, clearTickMsg{})
}

// finishClear advances the engine to the next hand and returns to idle. It is the
// single place the sweep hands back to normal play, whether it ran to completion
// or the player skipped it.
func (m Model) finishClear() (Model, tea.Cmd) {
	m.animState = animIdle
	m.boardShown = 0
	if err := m.game.NextHand(); err != nil {
		m.msg = err.Error()
	} else {
		m.msg = ""
		m.cursor = 0
	}
	return m, nil
}

// updateAnim handles animation ticks. handled is false when msg is not an
// animation tick, so Update can fall through to its other cases.
func (m Model) updateAnim(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg.(type) {
	case clearTickMsg:
		mm, cmd := m.onClearTick()
		return mm, cmd, true
	}
	return m, nil, false
}

// onClearTick removes one more card from the felt, rescheduling until the board
// is clear, then advances to the next hand.
func (m Model) onClearTick() (Model, tea.Cmd) {
	if m.animState != animClearing {
		return m, nil
	}
	m.boardShown--
	if m.boardShown > 0 {
		return m, tick(clearBeat, clearTickMsg{})
	}
	return m.finishClear()
}

// boardCounts reports how many hole cards and community slots the View should
// draw right now. Normally it is the full board; during the clear sweep it is the
// shrinking remainder, with cards leaving in reverse deal order — the community
// slots first (right to left), then the two hole cards.
func (m Model) boardCounts() (holeShown, communityShown int) {
	if m.animState != animClearing {
		return len(m.game.HoleCards()), numCommunity
	}
	holeShown = clampCount(m.boardShown, 2)
	communityShown = clampCount(m.boardShown-2, numCommunity)
	return holeShown, communityShown
}

// clampCount clamps n into [0, max].
func clampCount(n, max int) int {
	if n < 0 {
		return 0
	}
	if n > max {
		return max
	}
	return n
}
