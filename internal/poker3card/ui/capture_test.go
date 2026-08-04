package ui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/mykeychain/terminal-casino/internal/poker3card/engine"
)

// TestCaptureFrames drives the model through key beats and writes each rendered
// View() (with ANSI256 color) to $CAPTURE_DIR. Skipped unless CAPTURE=1 so it
// never runs in the normal suite. It is a screenshot harness, not an assertion.
func TestCaptureFrames(t *testing.T) {
	if os.Getenv("CAPTURE") == "" {
		t.Skip("set CAPTURE=1 to write frame files")
	}
	lipgloss.SetColorProfile(termenv.ANSI256)
	dir := os.Getenv("CAPTURE_DIR")
	if dir == "" {
		dir = "."
	}
	size := tea.WindowSizeMsg{Width: 72, Height: 26}

	write := func(name, ansi string) {
		if err := os.WriteFile(dir+"/"+name+".ansi", []byte(ansi), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Betting stage — both wagers posted, before the deal.
	{
		m := newTestModel(1000, 9, 6, scenarioBigWin())
		m, _ = upd(t, m, size)
		write("betting", m.View())
	}

	// 2. Decision — deal complete, player face up / dealer face down, Play/Fold.
	{
		m := newTestModel(1000, 9, 6, scenarioBigWin())
		m, _ = upd(t, m, size)
		m, _ = upd(t, m, keyEnter) // deal
		m = advanceDeal(t, m)
		write("decision", m.View())
	}

	// 3. Showdown WIN — straight flush, dealer 9-high (doesn't qualify). Net +$294.
	{
		m := newTestModel(1000, 9, 6, scenarioBigWin())
		m, _ = upd(t, m, size)
		m, _ = upd(t, m, keyEnter)
		m = advanceDeal(t, m)
		for i, n := 0, playIndex(t, m); i < n; i++ {
			m, _ = upd(t, m, keyRight)
		}
		m, _ = upd(t, m, keyEnter) // Play
		m = advanceDealer(t, m)
		m = settleResult(t, m)
		write("win", m.View())
	}

	// 4. Showdown LOSS — player Ten-high, dealer pair of Kings (qualifies, wins).
	//    Ante $9 + Play $9 + Pair Plus $6 all lost. Net -$24.
	{
		lossDeck := []engine.Card{
			card(engine.Two, engine.Clubs), card(engine.Seven, engine.Diamonds), card(engine.Ten, engine.Spades),
			card(engine.King, engine.Hearts), card(engine.King, engine.Spades), card(engine.Four, engine.Diamonds),
		}
		m := newTestModel(1000, 9, 6, lossDeck)
		m, _ = upd(t, m, size)
		m, _ = upd(t, m, keyEnter)
		m = advanceDeal(t, m)
		for i, n := 0, playIndex(t, m); i < n; i++ {
			m, _ = upd(t, m, keyRight)
		}
		m, _ = upd(t, m, keyEnter) // Play
		m = advanceDealer(t, m)
		m = settleResult(t, m)
		write("loss", m.View())
	}
}
