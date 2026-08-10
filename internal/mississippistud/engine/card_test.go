package engine

import "testing"

// c builds a card from a rank and suit.
func c(r Rank, s Suit) Card { return Card{Rank: r, Suit: s} }

// h builds a five-card hand array.
func h(a, b, cc, d, e Card) [5]Card { return [5]Card{a, b, cc, d, e} }

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
