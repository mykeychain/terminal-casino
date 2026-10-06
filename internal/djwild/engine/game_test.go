package engine

import "testing"

// stack joins the player's five cards and the dealer's five into a deck order.
func stack(player, dealer [5]Card) []Card {
	return append(append([]Card(nil), player[:]...), dealer[:]...)
}

// Fixture hands.
var (
	fullHouse  = h(c(King, Hearts), c(King, Spades), c(King, Clubs), c(Four, Hearts), c(Four, Diamonds))
	trips      = h(c(Ace, Hearts), c(Ace, Spades), c(Nine, Clubs), c(Six, Hearts), Joker) // wild trips
	pairQueens = h(c(Queen, Hearts), c(Queen, Spades), c(Nine, Diamonds), c(Six, Clubs), c(Three, Clubs))
	pairJacks  = h(c(Jack, Hearts), c(Jack, Spades), c(Nine, Clubs), c(Six, Diamonds), c(Three, Hearts))
	highCard   = h(c(Ace, Diamonds), c(Jack, Clubs), c(Nine, Hearts), c(Six, Spades), c(Four, Clubs))
	// pairQueens2 ties pairQueens exactly (same ranks, other suits).
	pairQueens2 = h(c(Queen, Diamonds), c(Queen, Clubs), c(Nine, Spades), c(Six, Hearts), c(Three, Diamonds))
)

// play deals with the given wagers and plays (or folds), checking the money
// invariant.
func play(t *testing.T, bankroll, ante, tripsBet int, player, dealer [5]Card, doPlay bool) *Game {
	t.Helper()
	g := NewGameWithDeck(bankroll, stack(player, dealer))
	if err := g.SetAnte(ante); err != nil {
		t.Fatalf("SetAnte: %v", err)
	}
	if err := g.SetTrips(tripsBet); err != nil {
		t.Fatalf("SetTrips: %v", err)
	}
	if err := g.Deal(); err != nil {
		t.Fatalf("Deal: %v", err)
	}
	if g.Dealer().Revealed {
		t.Fatal("dealer must stay hidden during the decision")
	}
	var err error
	if doPlay {
		err = g.Play()
	} else {
		err = g.Fold()
	}
	if err != nil {
		t.Fatalf("decision: %v", err)
	}
	if got := g.Bankroll(); got != bankroll+g.Result().Net {
		t.Fatalf("money invariant: bankroll %d != start %d + net %d", got, bankroll, g.Result().Net)
	}
	return g
}

func TestWinWithBlindBonus(t *testing.T) {
	g := play(t, 1000, 10, 0, fullHouse, pairQueens, true)
	s := g.Result()
	// Ante +10, Play (2× = $20) +20, Blind Full House 3:1 = +30.
	if s.Ante.Net != 10 || s.Play.Net != 20 || s.Blind.Net != 30 || s.Blind.Multiplier != 3 {
		t.Fatalf("settlement = %+v", s)
	}
	if s.Net != 60 {
		t.Errorf("net = %d, want 60", s.Net)
	}
}

func TestWinBelowStraightPushesBlind(t *testing.T) {
	g := play(t, 1000, 10, 0, pairQueens, pairJacks, true)
	s := g.Result()
	if s.Ante.Outcome != OutcomeWin || s.Play.Outcome != OutcomeWin || s.Blind.Outcome != OutcomePush {
		t.Fatalf("settlement = %+v, want ante/play win and blind push", s)
	}
	if s.Net != 30 {
		t.Errorf("net = %d, want 30", s.Net)
	}
}

func TestDealerAlwaysPlays(t *testing.T) {
	// A pair of Jacks loses to a dealer's pair of Queens; there is no qualifier.
	g := play(t, 1000, 10, 0, pairJacks, pairQueens, true)
	if s := g.Result(); s.Comparison >= 0 || s.Net != -40 {
		t.Fatalf("settlement = %+v, want a loss of ante+blind+play (-40)", s)
	}
	// Even a dealer's bare high card plays: the pair of Jacks wins.
	g = play(t, 1000, 10, 0, pairJacks, highCard, true)
	if s := g.Result(); s.Comparison <= 0 || s.Net != 30 {
		t.Fatalf("settlement = %+v, want a win against high card (+30)", s)
	}
}

func TestTiePushesEverything(t *testing.T) {
	g := play(t, 1000, 10, 0, pairQueens, pairQueens2, true)
	s := g.Result()
	if s.Comparison != 0 || s.Net != 0 || s.Blind.Outcome != OutcomePush {
		t.Fatalf("settlement = %+v, want full push", s)
	}
}

func TestFoldForfeitsAnteAndBlindTripsStillPays(t *testing.T) {
	g := play(t, 1000, 10, 5, trips, fullHouse, false)
	s := g.Result()
	if !s.Folded || s.Ante.Net != -10 || s.Blind.Net != -10 || s.Play.Bet != 0 {
		t.Fatalf("settlement = %+v, want ante and blind forfeited", s)
	}
	if s.Trips.Outcome != OutcomeWin || s.Trips.Multiplier != TripsWildPayouts[ThreeOfAKind] {
		t.Fatalf("trips = %+v, want a wild three of a kind win", s.Trips)
	}
	if s.Net != -20+5*TripsWildPayouts[ThreeOfAKind] {
		t.Errorf("net = %d", s.Net)
	}
}

func TestNaturalTripsPaysMore(t *testing.T) {
	natural := h(c(Ace, Hearts), c(Ace, Spades), c(Ace, Clubs), c(Six, Hearts), c(Four, Clubs))
	g := play(t, 1000, 10, 5, natural, fullHouse, false)
	if m := g.Result().Trips.Multiplier; m != TripsNaturalPayouts[ThreeOfAKind] {
		t.Errorf("natural trips multiplier = %d, want %d", m, TripsNaturalPayouts[ThreeOfAKind])
	}
}

func TestBetValidation(t *testing.T) {
	g := NewGameWithDeck(30, stack(fullHouse, pairQueens))
	if g.SetAnte(2) != ErrInvalidBet {
		t.Error("ante below the minimum must be rejected")
	}
	if g.SetAnte(16) != ErrInvalidBet {
		t.Error("ante + blind beyond the bankroll must be rejected")
	}
	if err := g.SetAnte(15); err != nil {
		t.Fatalf("ante 15 + blind 15 = 30 fits: %v", err)
	}
	if g.SetTrips(3) != ErrInvalidBet {
		t.Error("trips beyond the bankroll must be rejected")
	}
	if g.SetTrips(1) != ErrInvalidBet {
		t.Error("trips below the minimum must be rejected")
	}
}

func TestPlayNeedsTwiceTheAnte(t *testing.T) {
	g := NewGameWithDeck(30, stack(fullHouse, pairQueens))
	_ = g.SetAnte(10)
	_ = g.Deal() // bankroll 10 left; Play needs 20
	if g.CanPlay() {
		t.Fatal("Play must be illegal when the bankroll can't cover 2× the Ante")
	}
	if g.Play() != ErrIllegalAction {
		t.Error("Play must return ErrIllegalAction")
	}
}

func TestNextHandClampsAndEnds(t *testing.T) {
	g := play(t, 100, 20, 10, pairJacks, pairQueens, true) // loses 20+20+40+10 → 10 left
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Ante() != 5 || g.Trips() != 0 {
		t.Errorf("wagers = ante %d trips %d, want 5/0", g.Ante(), g.Trips())
	}
	if len(g.Player().Cards) != 0 {
		t.Error("the board must be clear on the next betting screen")
	}

	g = play(t, 46, 10, 6, pairJacks, pairQueens, true) // loses 46 → 0 left
	_ = g.NextHand()
	if g.Phase() != PhaseGameOver {
		t.Errorf("phase = %v, want game over", g.Phase())
	}
}

func TestRandomPlayKeepsMoneyInvariant(t *testing.T) {
	for seed := int64(1); seed <= 300; seed++ {
		g := NewGame(seed)
		_ = g.SetTrips(3)
		before := g.Bankroll()
		if err := g.Deal(); err != nil {
			t.Fatal(err)
		}
		if g.Player().Value.Category >= Pair {
			_ = g.Play()
		} else {
			_ = g.Fold()
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
