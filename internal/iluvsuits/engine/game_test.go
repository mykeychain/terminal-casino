package engine

import "testing"

// stack joins the player's seven cards and the dealer's seven into a deck order.
func stack(player, dealer []Card) []Card {
	return append(append([]Card(nil), player...), dealer...)
}

// Fixture hands.
var (
	// playerFive: a 5-card heart flush, King-high (straight-flush run of 1).
	playerFive = []Card{
		c(King, Hearts), c(Ten, Hearts), c(Eight, Hearts), c(Five, Hearts), c(Three, Hearts),
		c(Two, Spades), c(Four, Clubs),
	}
	// playerFour: a 4-card spade flush, Queen-high.
	playerFour = []Card{
		c(Queen, Spades), c(Nine, Spades), c(Six, Spades), c(Two, Spades),
		c(Ace, Hearts), c(King, Diamonds), c(Four, Clubs),
	}
	// playerThree: a 3-card spade flush with no straight-flush run — it loses
	// both side bets.
	playerThree = []Card{
		c(Queen, Spades), c(Nine, Spades), c(Six, Spades),
		c(Ace, Hearts), c(King, Diamonds), c(Four, Clubs), c(Two, Hearts),
	}
	// dealerFour: a qualifying 4-card club flush, Jack-high.
	dealerFour = []Card{
		c(Jack, Clubs), c(Seven, Clubs), c(Six, Clubs), c(Three, Clubs),
		c(Ace, Spades), c(Seven, Diamonds), c(Nine, Hearts),
	}
	// dealerNoQualify: at best a 3-card flush Eight-high (or 2-card flushes).
	dealerNoQualify = []Card{
		c(Eight, Diamonds), c(Five, Diamonds), c(Two, Diamonds),
		c(Ace, Clubs), c(King, Clubs), c(Queen, Spades), c(Jack, Hearts),
	}
	// dealerFive: a qualifying 5-card diamond flush, Ace-high.
	dealerFive = []Card{
		c(Ace, Diamonds), c(Queen, Diamonds), c(Ten, Diamonds), c(Six, Diamonds), c(Four, Diamonds),
		c(Three, Spades), c(Five, Clubs),
	}
)

// play deals with the given wagers and plays mult (0 folds), returning the game.
func play(t *testing.T, bankroll, ante, fr, sfr int, player, dealer []Card, mult int) *Game {
	t.Helper()
	g := NewGameWithDeck(bankroll, stack(player, dealer))
	if err := g.SetAnte(ante); err != nil {
		t.Fatalf("SetAnte: %v", err)
	}
	if err := g.SetFlushRush(fr); err != nil {
		t.Fatalf("SetFlushRush: %v", err)
	}
	if err := g.SetSuperFlushRush(sfr); err != nil {
		t.Fatalf("SetSuperFlushRush: %v", err)
	}
	if err := g.Deal(); err != nil {
		t.Fatalf("Deal: %v", err)
	}
	var err error
	if mult == 0 {
		err = g.Fold()
	} else {
		err = g.Play(mult)
	}
	if err != nil {
		t.Fatalf("decision: %v", err)
	}
	if got := g.Bankroll(); got != bankroll+g.Result().Net {
		t.Fatalf("money invariant: bankroll %d != start %d + net %d", got, bankroll, g.Result().Net)
	}
	return g
}

func TestPlayWinPaysAnteAndPlay(t *testing.T) {
	g := play(t, 1000, 10, 0, 0, playerFive, dealerFour, 2)
	s := g.Result()
	if !s.DealerQualified || s.Ante.Outcome != OutcomeWin || s.Play.Outcome != OutcomeWin {
		t.Fatalf("settlement = %+v, want qualified dealer and wins", s)
	}
	if s.Ante.Net != 10 || s.Play.Net != 20 || s.Net != 30 {
		t.Errorf("nets ante=%d play=%d total=%d, want 10/20/30", s.Ante.Net, s.Play.Net, s.Net)
	}
}

func TestDealerNotQualifyingPaysAntePushesPlay(t *testing.T) {
	g := play(t, 1000, 10, 0, 0, playerFour, dealerNoQualify, 1)
	s := g.Result()
	if s.DealerQualified {
		t.Fatal("dealer with an 8-high 3-card flush must not qualify")
	}
	if s.Ante.Outcome != OutcomeWin || s.Play.Outcome != OutcomePush || s.Net != 10 {
		t.Errorf("settlement = %+v, want ante win, play push, net +10", s)
	}
}

func TestDealerBeatsPlayer(t *testing.T) {
	g := play(t, 1000, 10, 0, 0, playerFive, dealerFive, 2)
	s := g.Result()
	if s.Comparison >= 0 || s.Net != -30 {
		t.Errorf("settlement = %+v, want dealer win and net -30", s)
	}
}

func TestTiePushes(t *testing.T) {
	mirror := []Card{ // the same ranks as playerFour's spade flush, in clubs
		c(Queen, Clubs), c(Nine, Clubs), c(Six, Clubs), c(Two, Clubs),
		c(Ace, Diamonds), c(King, Hearts), c(Four, Hearts),
	}
	g := play(t, 1000, 10, 0, 0, playerFour, mirror, 1)
	s := g.Result()
	if s.Comparison != 0 || s.Ante.Outcome != OutcomePush || s.Play.Outcome != OutcomePush || s.Net != 0 {
		t.Errorf("settlement = %+v, want a full push", s)
	}
}

func TestFoldLosesAnteButSideBetsResolve(t *testing.T) {
	g := play(t, 1000, 10, 5, 5, playerFive, dealerFour, 0)
	s := g.Result()
	if !s.Folded || s.Ante.Net != -10 {
		t.Fatalf("settlement = %+v, want fold forfeiting ante", s)
	}
	if s.FlushRush.Outcome != OutcomeWin || s.FlushRush.Length != 5 || s.FlushRush.Net != 50 {
		t.Errorf("flush rush = %+v, want 5-card win at 10:1 (+50)", s.FlushRush)
	}
	if s.SuperFlushRush.Outcome != OutcomeLoss || s.SuperFlushRush.Net != -5 {
		t.Errorf("super flush rush = %+v, want a loss", s.SuperFlushRush)
	}
	if s.Net != -10+50-5 {
		t.Errorf("net = %d, want %d", s.Net, -10+50-5)
	}
}

func TestSuperFlushRushPays(t *testing.T) {
	player := []Card{
		c(Five, Spades), c(Six, Spades), c(Seven, Spades), c(Eight, Spades),
		c(King, Hearts), c(Two, Diamonds), c(Ten, Clubs),
	}
	g := play(t, 1000, 10, 0, 3, player, dealerNoQualify, 1)
	s := g.Result()
	if s.SuperFlushRush.Length != 4 || s.SuperFlushRush.Net != 3*SuperFlushRushPayouts[4] {
		t.Errorf("super flush rush = %+v, want 4-card run paying %d", s.SuperFlushRush, 3*SuperFlushRushPayouts[4])
	}
}

func TestPlayCappedByFlushLength(t *testing.T) {
	g := NewGameWithDeck(1000, stack(playerFour, dealerFour))
	if err := g.Deal(); err != nil {
		t.Fatal(err)
	}
	if g.MaxPlay() != 1 || g.CanPlay(2) || g.CanPlay(3) {
		t.Fatalf("4-card flush: MaxPlay=%d, want only 1×", g.MaxPlay())
	}
	if err := g.Play(2); err != ErrIllegalAction {
		t.Errorf("Play(2) err = %v, want ErrIllegalAction", err)
	}

	g = NewGameWithDeck(1000, stack(playerFive, dealerFour))
	_ = g.Deal()
	if got := g.LegalPlays(); len(got) != 2 {
		t.Errorf("5-card flush legal plays = %v, want [1 2]", got)
	}
}

func TestPlayCappedByBankroll(t *testing.T) {
	g := NewGameWithDeck(25, stack(playerFive, dealerFour))
	_ = g.SetAnte(10)
	_ = g.Deal() // bankroll 15 after the ante
	if got := g.LegalPlays(); len(got) != 1 || got[0] != 1 {
		t.Errorf("legal plays = %v, want [1] (2× needs $20)", got)
	}
}

func TestBetValidation(t *testing.T) {
	g := NewGameWithDeck(20, stack(playerFive, dealerFour))
	if g.SetAnte(2) != ErrInvalidBet {
		t.Error("ante below the minimum must be rejected")
	}
	if g.SetFlushRush(2) != ErrInvalidBet {
		t.Error("side bet below the minimum must be rejected")
	}
	_ = g.SetAnte(10)
	if g.SetFlushRush(11) != ErrInvalidBet {
		t.Error("wagers beyond the bankroll must be rejected")
	}
	if g.SetFlushRush(10) != nil || g.SetSuperFlushRush(3) != ErrInvalidBet {
		t.Error("side bets share the bankroll with the ante")
	}
}

func TestNextHandEndsGameWhenBroke(t *testing.T) {
	g := play(t, 40, 10, 10, 10, playerThree, dealerFive, 1) // loses 10+10+10+10
	if g.Bankroll() != 0 {
		t.Fatalf("bankroll = %d, want 0", g.Bankroll())
	}
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseGameOver {
		t.Errorf("phase = %v, want game over", g.Phase())
	}

}

func TestNextHandClampsSideBetsFirst(t *testing.T) {
	g := play(t, 100, 20, 20, 20, playerThree, dealerFive, 1) // net -80 → bankroll 20
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Ante() != 20 || g.FlushRush() != 0 || g.SuperFlushRush() != 0 {
		t.Errorf("wagers = %d/%d/%d, want 20/0/0", g.Ante(), g.FlushRush(), g.SuperFlushRush())
	}
	if len(g.Player().Cards) != 0 {
		t.Error("the board must be clear on the next betting screen")
	}
}

func TestRandomPlayKeepsMoneyInvariant(t *testing.T) {
	for seed := int64(1); seed <= 300; seed++ {
		g := NewGame(seed)
		_ = g.SetFlushRush(3)
		_ = g.SetSuperFlushRush(3)
		before := g.Bankroll()
		if err := g.Deal(); err != nil {
			t.Fatal(err)
		}
		if g.Dealer().Revealed {
			t.Fatal("dealer must stay hidden during the decision")
		}
		plays := g.LegalPlays()
		if seed%4 == 0 || len(plays) == 0 {
			_ = g.Fold()
		} else {
			_ = g.Play(plays[len(plays)-1])
		}
		if g.Bankroll() != before+g.Result().Net {
			t.Fatalf("seed %d: bankroll %d != %d + %d", seed, g.Bankroll(), before, g.Result().Net)
		}
		seen := map[Card]bool{}
		for _, cd := range append(g.Player().Cards, g.Dealer().Cards...) {
			if seen[cd] {
				t.Fatalf("seed %d: duplicate card %v", seed, cd)
			}
			seen[cd] = true
		}
	}
}
