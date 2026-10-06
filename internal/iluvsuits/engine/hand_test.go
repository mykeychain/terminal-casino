package engine

import "testing"

// c builds a card from a rank and suit.
func c(r Rank, s Suit) Card { return Card{Rank: r, Suit: s} }

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

func TestEvaluatePicksLongestFlush(t *testing.T) {
	hand := []Card{
		c(Ace, Spades), c(King, Spades), // 2-card spade flush, Ace-high
		c(Four, Hearts), c(Six, Hearts), c(Nine, Hearts), // 3-card heart flush
		c(Two, Clubs), c(Three, Diamonds),
	}
	f := Evaluate(hand)
	if f.Suit != Hearts || f.Len() != 3 || f.High() != Nine {
		t.Fatalf("got %v %d-card %v-high, want 3-card hearts nine-high", f.Suit, f.Len(), f.High())
	}
	if got, want := f.Name(), "3-card ♥ flush, Nine-high"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
}

func TestEvaluateBreaksEqualLengthByRank(t *testing.T) {
	hand := []Card{
		c(King, Spades), c(Nine, Spades), c(Two, Spades),
		c(King, Hearts), c(Ten, Hearts), c(Three, Hearts),
		c(Four, Clubs),
	}
	f := Evaluate(hand)
	if f.Suit != Hearts {
		t.Fatalf("suit = %v, want hearts (K-10-3 beats K-9-2)", f.Suit)
	}
}

func TestCompare(t *testing.T) {
	five := FlushHand{Ranks: []Rank{Seven, Six, Five, Four, Two}}
	fourAce := FlushHand{Ranks: []Rank{Ace, King, Queen, Jack}}
	if Compare(five, fourAce) <= 0 {
		t.Error("a 5-card flush must beat any 4-card flush")
	}
	a := FlushHand{Suit: Spades, Ranks: []Rank{Ace, Nine, Four}}
	b := FlushHand{Suit: Hearts, Ranks: []Rank{Ace, Nine, Three}}
	if Compare(a, b) <= 0 || Compare(b, a) >= 0 {
		t.Error("equal-length flushes compare rank by rank")
	}
	tie := FlushHand{Suit: Clubs, Ranks: []Rank{Ace, Nine, Four}}
	if Compare(a, tie) != 0 {
		t.Error("identical ranks in different suits must tie")
	}
}

func TestQualifies(t *testing.T) {
	cases := []struct {
		ranks []Rank
		want  bool
	}{
		{[]Rank{Ace, King}, false},
		{[]Rank{Eight, Seven, Two}, false},
		{[]Rank{Nine, Three, Two}, true},
		{[]Rank{Six, Five, Four, Two}, true},
	}
	for _, tc := range cases {
		if got := Qualifies(FlushHand{Ranks: tc.ranks}); got != tc.want {
			t.Errorf("Qualifies(%v) = %v, want %v", tc.ranks, got, tc.want)
		}
	}
}

func TestStraightFlushLen(t *testing.T) {
	cases := []struct {
		name string
		hand []Card
		want int
	}{
		{"no run", []Card{c(Two, Spades), c(Four, Spades), c(Six, Hearts), c(Eight, Hearts), c(Ten, Clubs), c(Queen, Clubs), c(Ace, Diamonds)}, 1},
		{"run split by suit", []Card{c(Five, Spades), c(Six, Hearts), c(Seven, Spades), c(Two, Clubs), c(Jack, Clubs), c(King, Diamonds), c(Nine, Diamonds)}, 1},
		{"three run", []Card{c(Five, Spades), c(Six, Spades), c(Seven, Spades), c(Two, Clubs), c(Jack, Clubs), c(King, Diamonds), c(Nine, Diamonds)}, 3},
		{"ace low", []Card{c(Ace, Hearts), c(Two, Hearts), c(Three, Hearts), c(Four, Hearts), c(Jack, Clubs), c(King, Diamonds), c(Nine, Diamonds)}, 4},
		{"ace high", []Card{c(Queen, Clubs), c(King, Clubs), c(Ace, Clubs), c(Two, Spades), c(Three, Diamonds), c(Five, Hearts), c(Nine, Diamonds)}, 3},
		{"no wrap", []Card{c(King, Clubs), c(Ace, Clubs), c(Two, Clubs), c(Five, Spades), c(Eight, Diamonds), c(Ten, Hearts), c(Four, Diamonds)}, 2},
		{"seven", []Card{c(Two, Spades), c(Three, Spades), c(Four, Spades), c(Five, Spades), c(Six, Spades), c(Seven, Spades), c(Eight, Spades)}, 7},
	}
	for _, tc := range cases {
		if got := StraightFlushLen(tc.hand); got != tc.want {
			t.Errorf("%s: StraightFlushLen = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestSortForDisplayPutsFlushFirst(t *testing.T) {
	hand := []Card{
		c(Two, Clubs), c(Ace, Spades), c(Four, Hearts), c(Nine, Hearts),
		c(Three, Diamonds), c(Six, Hearts), c(King, Spades),
	}
	got := SortForDisplay(hand)
	want := []Card{
		c(Nine, Hearts), c(Six, Hearts), c(Four, Hearts),
		c(Ace, Spades), c(King, Spades),
		c(Three, Diamonds), c(Two, Clubs), // equal-length suits fall back to suit order
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortForDisplay = %v, want %v", got, want)
		}
	}
}

func TestMaxPlayMultiple(t *testing.T) {
	for n, want := range map[int]int{2: 1, 3: 1, 4: 1, 5: 2, 6: 3, 7: 3} {
		if got := MaxPlayMultiple(n); got != want {
			t.Errorf("MaxPlayMultiple(%d) = %d, want %d", n, got, want)
		}
	}
}
