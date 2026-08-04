package engine

import "testing"

// ---- Category recognition ----

func TestEvaluateCategories(t *testing.T) {
	tests := []struct {
		name string
		hand [5]Card
		want HandCategory
	}{
		{"royal flush", h(c(Ten, Spades), c(Jack, Spades), c(Queen, Spades), c(King, Spades), c(Ace, Spades)), RoyalFlush},
		{"straight flush", h(c(Nine, Spades), c(Eight, Spades), c(Seven, Spades), c(Six, Spades), c(Five, Spades)), StraightFlush},
		{"four of a kind", h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), c(Seven, Diamonds), c(Two, Spades)), FourOfAKind},
		{"full house", h(c(King, Spades), c(King, Hearts), c(King, Clubs), c(Two, Spades), c(Two, Hearts)), FullHouse},
		{"flush", h(c(King, Diamonds), c(Nine, Diamonds), c(Seven, Diamonds), c(Four, Diamonds), c(Two, Diamonds)), Flush},
		{"straight", h(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs), c(Six, Diamonds), c(Five, Spades)), Straight},
		{"three of a kind", h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), c(King, Diamonds), c(Two, Spades)), ThreeOfAKind},
		{"two pair", h(c(King, Spades), c(King, Hearts), c(Two, Clubs), c(Two, Diamonds), c(Nine, Spades)), TwoPair},
		{"pair", h(c(King, Spades), c(King, Hearts), c(Nine, Clubs), c(Four, Diamonds), c(Two, Spades)), Pair},
		{"high card", h(c(King, Spades), c(Nine, Hearts), c(Seven, Clubs), c(Four, Diamonds), c(Two, Spades)), HighCard},
	}
	for _, tc := range tests {
		if got := Evaluate(tc.hand).Category; got != tc.want {
			t.Errorf("%s: category = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// ---- Ranking ORDER (standard five-card ordering) ----

func TestRankingOrder(t *testing.T) {
	royal := Evaluate(h(c(Ten, Spades), c(Jack, Spades), c(Queen, Spades), c(King, Spades), c(Ace, Spades)))
	straightFlush := Evaluate(h(c(Nine, Spades), c(Eight, Spades), c(Seven, Spades), c(Six, Spades), c(Five, Spades)))
	quads := Evaluate(h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), c(Seven, Diamonds), c(Two, Spades)))
	fullHouse := Evaluate(h(c(King, Spades), c(King, Hearts), c(King, Clubs), c(Two, Spades), c(Two, Hearts)))
	flush := Evaluate(h(c(King, Diamonds), c(Nine, Diamonds), c(Seven, Diamonds), c(Four, Diamonds), c(Two, Diamonds)))
	straight := Evaluate(h(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs), c(Six, Diamonds), c(Five, Spades)))
	trips := Evaluate(h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), c(King, Diamonds), c(Two, Spades)))
	twoPair := Evaluate(h(c(King, Spades), c(King, Hearts), c(Two, Clubs), c(Two, Diamonds), c(Nine, Spades)))
	pair := Evaluate(h(c(King, Spades), c(King, Hearts), c(Nine, Clubs), c(Four, Diamonds), c(Two, Spades)))
	high := Evaluate(h(c(King, Spades), c(Nine, Hearts), c(Seven, Clubs), c(Four, Diamonds), c(Two, Spades)))

	// Ordered strongest -> weakest.
	order := []HandValue{royal, straightFlush, quads, fullHouse, flush, straight, trips, twoPair, pair, high}
	for i := 0; i+1 < len(order); i++ {
		if Compare(order[i], order[i+1]) <= 0 {
			t.Errorf("rank %d (%v) should beat rank %d (%v)", i, order[i].Category, i+1, order[i+1].Category)
		}
	}

	// Explicit five-card checks: flush beats straight (opposite of the 3-card game).
	if Compare(flush, straight) <= 0 {
		t.Error("flush must beat straight (five-card ordering)")
	}
	if Compare(royal, straightFlush) <= 0 {
		t.Error("royal flush must beat a lower straight flush")
	}
}

// ---- Straight edge cases ----

func TestStraightEdgeCases(t *testing.T) {
	// Wheel A-2-3-4-5 is a straight, high card 5 (Ace plays low).
	wheel := Evaluate(h(c(Ace, Spades), c(Two, Hearts), c(Three, Clubs), c(Four, Diamonds), c(Five, Spades)))
	if wheel.Category != Straight {
		t.Fatalf("A-2-3-4-5 category = %v, want Straight", wheel.Category)
	}
	if wheel.Tiebreak[0] != 5 {
		t.Errorf("wheel high = %d, want 5", wheel.Tiebreak[0])
	}

	// Broadway 10-J-Q-K-A is a straight, high card Ace.
	broadway := Evaluate(h(c(Ten, Spades), c(Jack, Hearts), c(Queen, Clubs), c(King, Diamonds), c(Ace, Spades)))
	if broadway.Category != Straight {
		t.Fatalf("10-J-Q-K-A category = %v, want Straight", broadway.Category)
	}
	if broadway.Tiebreak[0] != int(Ace) {
		t.Errorf("broadway high = %d, want 14", broadway.Tiebreak[0])
	}
	if Compare(broadway, wheel) <= 0 {
		t.Error("broadway must beat the wheel")
	}

	// Q-K-A-2-3 is NOT a straight (no wrap-around).
	qka23 := Evaluate(h(c(Queen, Spades), c(King, Hearts), c(Ace, Clubs), c(Two, Diamonds), c(Three, Spades)))
	if qka23.Category == Straight {
		t.Error("Q-K-A-2-3 must NOT be a straight")
	}
	// K-A-2-3-4 is NOT a straight (no wrap-around).
	ka234 := Evaluate(h(c(King, Spades), c(Ace, Hearts), c(Two, Clubs), c(Three, Diamonds), c(Four, Spades)))
	if ka234.Category == Straight {
		t.Error("K-A-2-3-4 must NOT be a straight")
	}
}

func TestRoyalDistinctFromStraightFlush(t *testing.T) {
	royal := Evaluate(h(c(Ten, Hearts), c(Jack, Hearts), c(Queen, Hearts), c(King, Hearts), c(Ace, Hearts)))
	if royal.Category != RoyalFlush {
		t.Fatalf("10-J-Q-K-A suited category = %v, want RoyalFlush", royal.Category)
	}
	sf := Evaluate(h(c(Nine, Hearts), c(Ten, Hearts), c(Jack, Hearts), c(Queen, Hearts), c(King, Hearts)))
	if sf.Category != StraightFlush {
		t.Fatalf("9-K suited category = %v, want StraightFlush", sf.Category)
	}
	if HandPayout(royal.Category) != 500 || HandPayout(sf.Category) != 100 {
		t.Errorf("royal/straight-flush payouts = %d/%d, want 500/100",
			HandPayout(royal.Category), HandPayout(sf.Category))
	}
}

// ---- Pair qualification tiers ----

func TestPairQualificationTiers(t *testing.T) {
	tests := []struct {
		pairRank Rank
		wantMult int
		wantOut  Outcome
	}{
		{Ace, PairWinPayout, OutcomeWin},
		{King, PairWinPayout, OutcomeWin},
		{Queen, PairWinPayout, OutcomeWin},
		{Jack, PairWinPayout, OutcomeWin},
		{Ten, 0, OutcomePush},
		{Nine, 0, OutcomePush},
		{Eight, 0, OutcomePush},
		{Seven, 0, OutcomePush},
		{Six, 0, OutcomePush},
		{Five, 0, OutcomeLoss},
		{Four, 0, OutcomeLoss},
		{Three, 0, OutcomeLoss},
		{Two, 0, OutcomeLoss},
	}
	for _, tc := range tests {
		// Build a hand with exactly one pair of the given rank and three low,
		// distinct kickers that never form a better hand.
		v := Evaluate(h(
			c(tc.pairRank, Spades), c(tc.pairRank, Hearts),
			kicker(tc.pairRank, 0), kicker(tc.pairRank, 1), kicker(tc.pairRank, 2),
		))
		if v.Category != Pair {
			t.Fatalf("pair of %v: category = %v, want Pair", tc.pairRank, v.Category)
		}
		mult, out := payout(v)
		if mult != tc.wantMult || out != tc.wantOut {
			t.Errorf("pair of %v: got mult %d outcome %v, want %d %v",
				tc.pairRank, mult, out, tc.wantMult, tc.wantOut)
		}
	}
}

// kicker returns a distinct low card that differs from the pair rank, used to
// pad a one-pair test hand without accidentally making two pair or a straight.
func kicker(pair Rank, i int) Card {
	choices := []Rank{Two, Three, Four, Six, Eight, Ten, Queen}
	suitsForKick := []Suit{Clubs, Diamonds, Spades}
	// pick ranks that are not the pair rank and are non-consecutive spread
	picked := make([]Rank, 0, 3)
	for _, r := range choices {
		if r == pair {
			continue
		}
		picked = append(picked, r)
		if len(picked) == 3 {
			break
		}
	}
	return Card{Rank: picked[i], Suit: suitsForKick[i]}
}

// ---- Hand names ----

func TestHandNames(t *testing.T) {
	tests := []struct {
		hand [5]Card
		want string
	}{
		{h(c(Ten, Spades), c(Jack, Spades), c(Queen, Spades), c(King, Spades), c(Ace, Spades)), "Royal Flush"},
		{h(c(Nine, Spades), c(Eight, Spades), c(Seven, Spades), c(Six, Spades), c(Five, Spades)), "Straight Flush"},
		{h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), c(Seven, Diamonds), c(Two, Spades)), "Four of a Kind"},
		{h(c(King, Spades), c(King, Hearts), c(King, Clubs), c(Two, Spades), c(Two, Hearts)), "Full House"},
		{h(c(King, Diamonds), c(Nine, Diamonds), c(Seven, Diamonds), c(Four, Diamonds), c(Two, Diamonds)), "Flush"},
		{h(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs), c(Six, Diamonds), c(Five, Spades)), "Straight"},
		{h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), c(King, Diamonds), c(Two, Spades)), "Three of a Kind"},
		{h(c(King, Spades), c(King, Hearts), c(Two, Clubs), c(Two, Diamonds), c(Nine, Spades)), "Two Pair"},
		{h(c(King, Spades), c(King, Hearts), c(Nine, Clubs), c(Four, Diamonds), c(Two, Spades)), "Pair of Kings"},
		{h(c(Six, Spades), c(Six, Hearts), c(Nine, Clubs), c(Four, Diamonds), c(Two, Spades)), "Pair of Sixes"},
		{h(c(Queen, Spades), c(Nine, Hearts), c(Seven, Clubs), c(Four, Diamonds), c(Two, Spades)), "Queen-high"},
		{h(c(Ace, Spades), c(Nine, Hearts), c(Seven, Clubs), c(Four, Diamonds), c(Two, Spades)), "Ace-high"},
	}
	for _, tc := range tests {
		if got := Evaluate(tc.hand).Name(); got != tc.want {
			t.Errorf("name = %q, want %q", got, tc.want)
		}
	}
}
