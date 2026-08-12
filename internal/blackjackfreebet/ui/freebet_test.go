package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/blackjackfreebet/engine"
	"github.com/mykeychain/terminal-casino/internal/theme"
)

// actionIndex returns how many "right" presses select the given action.
func actionIndex(t *testing.T, m Model, want engine.Action) int {
	t.Helper()
	for i, a := range m.playerActions() {
		if a == want {
			return i
		}
	}
	t.Fatalf("action %v is not currently legal", want)
	return 0
}

// TestFreeDoubleLabelShown: a two-card hard 11 offers a free double, and the
// action bar names it "Free Double" so the player sees the house is paying.
func TestFreeDoubleLabelShown(t *testing.T) {
	m := newModel(1000, engine.Five, engine.Seven, engine.Six, engine.Ten)
	m, _ = upd(t, m, keyEnter)
	m = advanceDeal(t, m)
	bar := stripANSI(m.renderActionBar())
	if !strings.Contains(bar, "Free Double") {
		t.Fatalf("action bar = %q, want it to advertise a free double", bar)
	}
}

// TestFreeActionRendersInGreenAccent: an unselected free action is tinted with
// the green "on the house" accent, distinguishing it from the plain actions.
func TestFreeActionRendersInGreenAccent(t *testing.T) {
	m := newModel(1000, engine.Five, engine.Seven, engine.Six, engine.Ten) // 5,6 = free double
	m, _ = upd(t, m, keyEnter)
	m = advanceDeal(t, m)
	// Cursor sits on Hit (index 0), so the free Double is unselected and must
	// carry the green accent rather than the plain soft-white style.
	bar := m.renderActionBar()
	probe := lipgloss.NewStyle().Foreground(theme.Green).Render("X")
	greenSeq := probe[:strings.Index(probe, "X")] // the opening SGR for the green accent
	if !strings.Contains(bar, greenSeq) {
		t.Fatalf("free action should render in the green accent; action bar lacks the green SGR")
	}
}

// TestFreeSplitLabelShownOwnMoneyDoublePlain: an 8,8 splits free (labeled), while
// the own-money double on the hard 16 stays a plain "Double".
func TestFreeSplitLabelShownOwnMoneyDoublePlain(t *testing.T) {
	m := newModel(1000, engine.Eight, engine.Seven, engine.Eight, engine.Ten)
	m, _ = upd(t, m, keyEnter)
	m = advanceDeal(t, m)
	bar := stripANSI(m.renderActionBar())
	if !strings.Contains(bar, "Free Split") {
		t.Fatalf("action bar = %q, want it to advertise a free split", bar)
	}
	if strings.Contains(bar, "Free Double") {
		t.Fatalf("action bar = %q, an own-money double on hard 16 must not be labeled free", bar)
	}
}

// TestFreeSplitFooterShowsFreeBet: after a free split the new hand carries a
// house-funded free bet, shown as "$N free" in its footer.
func TestFreeSplitFooterShowsFreeBet(t *testing.T) {
	m := newModel(1000, engine.Eight, engine.Seven, engine.Eight, engine.Ten, engine.Three, engine.Nine)
	m, _ = upd(t, m, keyEnter)
	m = advanceDeal(t, m)
	idx := actionIndex(t, m, engine.ActionSplit)
	for i := 0; i < idx; i++ {
		m, _ = upd(t, m, keyRight)
	}
	m, _ = upd(t, m, keyEnter) // free split
	view := stripANSI(m.View())
	if !strings.Contains(view, "$3 free") {
		t.Fatalf("view after a free split lacks the free-bet footer:\n%s", view)
	}
}

// TestPush22BannerAndDealerStatus: a dealer 22 pushes the player's 20. The result
// banner names the Push 22 rule and the dealer area labels 22 as a push, not a bust.
func TestPush22BannerAndDealerStatus(t *testing.T) {
	// Player 10,10 = 20 stands; dealer 10,6 = 16 hits 6 => 22.
	m := newModel(1000, engine.Ten, engine.Ten, engine.Ten, engine.Six, engine.Six)
	m, _ = upd(t, m, keyEnter)
	m = advanceDeal(t, m)
	for i, n := 0, standIndex(t, m); i < n; i++ {
		m, _ = upd(t, m, keyRight)
	}
	m, _ = upd(t, m, keyEnter) // stand
	m = advanceDealer(t, m)
	m = settleResult(t, m)

	if !m.game.DealerPush22() {
		t.Fatalf("expected a dealer Push 22, dealer value = %d", m.game.Dealer().Value)
	}
	if banner := stripANSI(m.renderBanner()); !strings.Contains(banner, "PUSH · DEALER 22") {
		t.Fatalf("banner = %q, want it to contain PUSH · DEALER 22", banner)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "22 · push") {
		t.Fatalf("dealer status should read '22 · push':\n%s", view)
	}
	if strings.Contains(view, "bust (22)") {
		t.Fatalf("a dealer 22 must not render as a bust:\n%s", view)
	}
}
