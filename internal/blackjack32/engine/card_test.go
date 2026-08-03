package engine

import "testing"

func TestNewDeck(t *testing.T) {
	d := NewDeck()
	if len(d) != 52 {
		t.Fatalf("deck size = %d, want 52", len(d))
	}
	seen := map[Card]bool{}
	for _, c := range d {
		if seen[c] {
			t.Fatalf("duplicate card %v", c)
		}
		seen[c] = true
	}
}

func TestCardValue(t *testing.T) {
	tests := []struct {
		rank Rank
		want int
	}{
		{Two, 2}, {Nine, 9}, {Ten, 10}, {Jack, 10}, {Queen, 10}, {King, 10}, {Ace, 11},
	}
	for _, tc := range tests {
		if got := (Card{Rank: tc.rank}).Value(); got != tc.want {
			t.Errorf("%v value = %d, want %d", tc.rank, got, tc.want)
		}
	}
}

func TestShoeShuffleIsDeterministicPerSeed(t *testing.T) {
	// Same seed => identical shoe order; the RNG is owned and injected.
	a := NewGame(42)
	b := NewGame(42)
	for i := 0; i < 100; i++ {
		if a.shoe.draw() != b.shoe.draw() {
			t.Fatalf("same-seed shoes diverged at %d", i)
		}
	}
	// Different seed => (almost certainly) different order.
	c := NewGame(43)
	diff := false
	d := NewGame(42)
	for i := 0; i < 100; i++ {
		if c.shoe.draw() != d.shoe.draw() {
			diff = true
			break
		}
	}
	if !diff {
		t.Fatalf("different seeds produced identical order")
	}
}

func TestShoeSizeAndCutCard(t *testing.T) {
	g := NewGame(1)
	if got := g.CardsRemaining(); got != NumDecks*52 {
		t.Fatalf("shoe size = %d, want %d", got, NumDecks*52)
	}
	if CutCardPosition != 234 {
		t.Fatalf("CutCardPosition = %d, want 234", CutCardPosition)
	}
	if g.CutCardReached() {
		t.Fatalf("fresh shoe should not have reached cut card")
	}
}
