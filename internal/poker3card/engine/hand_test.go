package engine

import "testing"

// c builds a card from a rank and suit.
func c(r Rank, s Suit) Card { return Card{Rank: r, Suit: s} }

// h builds a three-card hand array.
func h(a, b, cc Card) [3]Card { return [3]Card{a, b, cc} }

func TestNewDeckIsFull(t *testing.T) {
	d := NewDeck(nil)
	if d.Remaining() != 52 {
		t.Fatalf("deck size = %d, want 52", d.Remaining())
	}
	seen := map[Card]bool{}
	for d.Remaining() > 0 {
		card := d.draw()
		if seen[card] {
			t.Fatalf("duplicate card %v", card)
		}
		seen[card] = true
	}
}

func TestCardString(t *testing.T) {
	if got := c(Ace, Spades).String(); got != "A♠" {
		t.Errorf("A♠ string = %q", got)
	}
	if got := c(Ten, Hearts).String(); got != "10♥" {
		t.Errorf("10♥ string = %q", got)
	}
}

// ---- Category recognition ----

func TestEvaluateCategories(t *testing.T) {
	tests := []struct {
		name string
		hand [3]Card
		want HandCategory
	}{
		{"straight flush", h(c(Nine, Spades), c(Eight, Spades), c(Seven, Spades)), StraightFlush},
		{"three of a kind", h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs)), ThreeOfAKind},
		{"straight mixed", h(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs)), Straight},
		{"flush", h(c(King, Diamonds), c(Nine, Diamonds), c(Two, Diamonds)), Flush},
		{"pair", h(c(King, Spades), c(King, Hearts), c(Three, Clubs)), Pair},
		{"high card", h(c(King, Spades), c(Nine, Hearts), c(Two, Clubs)), HighCard},
	}
	for _, tc := range tests {
		if got := Evaluate(tc.hand).Category; got != tc.want {
			t.Errorf("%s: category = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// ---- Ranking ORDER (three-card specific) ----

func TestRankingOrder(t *testing.T) {
	straightFlush := Evaluate(h(c(Nine, Spades), c(Eight, Spades), c(Seven, Spades)))
	trips := Evaluate(h(c(Five, Spades), c(Five, Hearts), c(Five, Clubs)))
	straight := Evaluate(h(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs)))
	flush := Evaluate(h(c(King, Diamonds), c(Nine, Diamonds), c(Two, Diamonds)))
	pair := Evaluate(h(c(King, Spades), c(King, Hearts), c(Three, Clubs)))
	high := Evaluate(h(c(King, Spades), c(Nine, Hearts), c(Two, Clubs)))

	// Ordered strongest -> weakest.
	order := []HandValue{straightFlush, trips, straight, flush, pair, high}
	for i := 0; i+1 < len(order); i++ {
		if Compare(order[i], order[i+1]) <= 0 {
			t.Errorf("rank %d should beat rank %d", i, i+1)
		}
	}

	// Explicit checks from the spec.
	if Compare(straight, flush) <= 0 {
		t.Error("straight must beat flush (three-card ordering)")
	}
	if Compare(trips, straight) <= 0 {
		t.Error("three of a kind must beat straight")
	}
	if Compare(straightFlush, trips) <= 0 {
		t.Error("straight flush must be the top category")
	}
	if Compare(pair, high) <= 0 {
		t.Error("pair must beat high card")
	}
}

// ---- Straight edge cases ----

func TestStraightAceLowAndHigh(t *testing.T) {
	// A-2-3 is a straight (lowest), Ace plays low, high card = 3.
	wheel := Evaluate(h(c(Ace, Spades), c(Two, Hearts), c(Three, Clubs)))
	if wheel.Category != Straight {
		t.Fatalf("A-2-3 category = %v, want Straight", wheel.Category)
	}
	if wheel.Tiebreak[0] != 3 {
		t.Errorf("A-2-3 straight high = %d, want 3", wheel.Tiebreak[0])
	}

	// Q-K-A is a straight (highest), high card = Ace(14).
	broadway := Evaluate(h(c(Queen, Spades), c(King, Hearts), c(Ace, Clubs)))
	if broadway.Category != Straight {
		t.Fatalf("Q-K-A category = %v, want Straight", broadway.Category)
	}
	if broadway.Tiebreak[0] != int(Ace) {
		t.Errorf("Q-K-A straight high = %d, want 14", broadway.Tiebreak[0])
	}

	// The highest straight beats the lowest.
	if Compare(broadway, wheel) <= 0 {
		t.Error("Q-K-A must beat A-2-3")
	}

	// K-A-2 is NOT a straight (wraps around); it is a plain high card.
	notStraight := Evaluate(h(c(King, Spades), c(Ace, Hearts), c(Two, Clubs)))
	if notStraight.Category != HighCard {
		t.Errorf("K-A-2 category = %v, want HighCard (not a straight)", notStraight.Category)
	}
}

// ---- Tie handling and tiebreak ----

func TestTiePush(t *testing.T) {
	// Identical ranks, different suits => exact tie regardless of suit.
	a := Evaluate(h(c(King, Spades), c(Nine, Hearts), c(Four, Clubs)))
	b := Evaluate(h(c(King, Diamonds), c(Nine, Clubs), c(Four, Hearts)))
	if Compare(a, b) != 0 {
		t.Error("equal ranks must tie (suits never break ties)")
	}
}

func TestHighCardTiebreak(t *testing.T) {
	// Same top card, differ on the second card.
	strong := Evaluate(h(c(King, Spades), c(Ten, Hearts), c(Two, Clubs)))
	weak := Evaluate(h(c(King, Diamonds), c(Nine, Hearts), c(Eight, Clubs)))
	if Compare(strong, weak) <= 0 {
		t.Error("K-10-2 must beat K-9-8 on the second card")
	}

	// Same top two cards, differ on the third.
	s2 := Evaluate(h(c(King, Spades), c(Nine, Hearts), c(Five, Clubs)))
	w2 := Evaluate(h(c(King, Diamonds), c(Nine, Clubs), c(Three, Hearts)))
	if Compare(s2, w2) <= 0 {
		t.Error("K-9-5 must beat K-9-3 on the third card")
	}
}

func TestPairTiebreak(t *testing.T) {
	// Higher pair wins.
	pKings := Evaluate(h(c(King, Spades), c(King, Hearts), c(Two, Clubs)))
	pQueens := Evaluate(h(c(Queen, Spades), c(Queen, Hearts), c(Ace, Clubs)))
	if Compare(pKings, pQueens) <= 0 {
		t.Error("pair of kings must beat pair of queens regardless of kicker")
	}
	// Equal pair, higher kicker wins.
	kAce := Evaluate(h(c(King, Spades), c(King, Hearts), c(Ace, Clubs)))
	kTwo := Evaluate(h(c(King, Diamonds), c(King, Clubs), c(Two, Hearts)))
	if Compare(kAce, kTwo) <= 0 {
		t.Error("pair of kings with ace kicker must beat with two kicker")
	}
}

// ---- Hand names ----

func TestHandNames(t *testing.T) {
	tests := []struct {
		hand [3]Card
		want string
	}{
		{h(c(Nine, Spades), c(Eight, Spades), c(Seven, Spades)), "Straight Flush"},
		{h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs)), "Three of a Kind"},
		{h(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs)), "Straight"},
		{h(c(King, Diamonds), c(Nine, Diamonds), c(Two, Diamonds)), "Flush"},
		{h(c(King, Spades), c(King, Hearts), c(Three, Clubs)), "Pair of Kings"},
		{h(c(Six, Spades), c(Six, Hearts), c(Three, Clubs)), "Pair of Sixes"},
		{h(c(Queen, Spades), c(Nine, Hearts), c(Two, Clubs)), "Queen-high"},
		{h(c(Ace, Spades), c(Nine, Hearts), c(Four, Clubs)), "Ace-high"},
	}
	for _, tc := range tests {
		if got := Evaluate(tc.hand).Name(); got != tc.want {
			t.Errorf("name = %q, want %q", got, tc.want)
		}
	}
}
