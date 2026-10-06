package engine

import (
	"testing"
)

// c builds a card from a rank and suit.
func c(r Rank, s Suit) Card { return Card{Rank: r, Suit: s} }

// h builds a five-card hand array.
func h(a, b, cc, d, e Card) [5]Card { return [5]Card{a, b, cc, d, e} }

func TestNewDeckHasJoker(t *testing.T) {
	d := NewDeck(nil)
	if d.Remaining() != 53 {
		t.Fatalf("deck size = %d, want 53", d.Remaining())
	}
	seen := map[Card]bool{}
	wilds := 0
	for d.Remaining() > 0 {
		card := d.draw()
		if seen[card] {
			t.Fatalf("duplicate card %v", card)
		}
		seen[card] = true
		if card.IsWild() {
			wilds++
		}
	}
	if !seen[Joker] || wilds != 5 {
		t.Fatalf("joker present = %v, wilds = %d; want true, 5", seen[Joker], wilds)
	}
}

func TestEvaluateCategories(t *testing.T) {
	cases := []struct {
		name     string
		hand     [5]Card
		want     HandCategory
		natural  bool
		wantName string
	}{
		{"five wilds", h(Joker, c(Two, Spades), c(Two, Hearts), c(Two, Diamonds), c(Two, Clubs)), FiveWilds, false, "Five Wilds"},
		{"natural royal", h(c(Ten, Hearts), c(Jack, Hearts), c(Queen, Hearts), c(King, Hearts), c(Ace, Hearts)), RoyalFlush, true, "Royal Flush"},
		{"wild royal", h(c(Ten, Hearts), Joker, c(Queen, Hearts), c(King, Hearts), c(Ace, Hearts)), RoyalFlush, false, "Royal Flush"},
		{"four wilds and a king make a royal", h(c(King, Spades), Joker, c(Two, Hearts), c(Two, Diamonds), c(Two, Clubs)), RoyalFlush, false, "Royal Flush"},
		{"five of a kind", h(c(Nine, Hearts), c(Nine, Spades), c(Nine, Clubs), c(Two, Hearts), Joker), FiveOfAKind, false, "Five Nines"},
		{"wild straight flush", h(c(Five, Clubs), c(Six, Clubs), c(Eight, Clubs), c(Nine, Clubs), c(Two, Hearts)), StraightFlush, false, "Straight Flush"},
		{"natural straight flush with a deuce", h(c(Two, Spades), c(Three, Spades), c(Four, Spades), c(Five, Spades), c(Six, Spades)), StraightFlush, true, "Straight Flush"},
		{"wild quads", h(c(King, Hearts), c(King, Spades), c(King, Clubs), Joker, c(Four, Diamonds)), FourOfAKind, false, "Four of a Kind"},
		{"natural full house", h(c(King, Hearts), c(King, Spades), c(King, Clubs), c(Four, Hearts), c(Four, Diamonds)), FullHouse, true, "Full House"},
		{"two pair plus wild is a full house", h(c(King, Hearts), c(King, Spades), c(Four, Clubs), c(Four, Hearts), Joker), FullHouse, false, "Full House"},
		{"wild flush", h(c(King, Hearts), c(Nine, Hearts), c(Seven, Hearts), c(Four, Hearts), c(Two, Spades)), Flush, false, "Flush"},
		{"wild straight", h(c(Six, Hearts), c(Seven, Spades), c(Nine, Clubs), c(Ten, Hearts), Joker), Straight, false, "Straight"},
		{"one wild is trips", h(c(Ace, Hearts), c(Ace, Spades), c(Nine, Clubs), c(Six, Hearts), c(Two, Clubs)), ThreeOfAKind, false, "Three of a Kind"},
		{"one wild pairs the top card", h(c(Ace, Hearts), c(Jack, Spades), c(Nine, Clubs), c(Six, Hearts), Joker), Pair, false, "Pair of Aces"},
		{"natural two pair", h(c(Ace, Hearts), c(Ace, Spades), c(Nine, Clubs), c(Nine, Hearts), c(Four, Clubs)), TwoPair, true, "Two Pair"},
		{"high card", h(c(Ace, Hearts), c(Jack, Spades), c(Nine, Clubs), c(Six, Hearts), c(Four, Clubs)), HighCard, true, "Ace-high"},
	}
	for _, tc := range cases {
		v := Evaluate(tc.hand)
		if v.Category != tc.want || v.Natural != tc.natural || v.Name() != tc.wantName {
			t.Errorf("%s: got %v natural=%v %q, want %v natural=%v %q",
				tc.name, v.Category, v.Natural, v.Name(), tc.want, tc.natural, tc.wantName)
		}
	}
}

func TestWildsPlayHighest(t *testing.T) {
	// A wild with a 10-high straight draw plays as the top end.
	v := Evaluate(h(c(Seven, Hearts), c(Eight, Spades), c(Nine, Clubs), c(Ten, Hearts), Joker))
	if v.Category != Straight || v.Tiebreak[0] != int(Jack) {
		t.Errorf("open-ended straight with a wild = %v high %v, want Jack-high straight", v.Category, v.Tiebreak)
	}
	// A wild in a flush plays as the Ace.
	v = Evaluate(h(c(King, Hearts), c(Nine, Hearts), c(Seven, Hearts), c(Four, Hearts), Joker))
	if v.Category != Flush || v.Tiebreak[0] != int(Ace) || v.Tiebreak[1] != int(King) {
		t.Errorf("wild flush tiebreak = %v, want Ace, King, ...", v.Tiebreak)
	}
}

func TestNaturalAndWildTie(t *testing.T) {
	natural := Evaluate(h(c(Five, Clubs), c(Six, Clubs), c(Seven, Clubs), c(Eight, Clubs), c(Nine, Clubs)))
	wild := Evaluate(h(c(Five, Hearts), c(Six, Hearts), c(Seven, Hearts), c(Eight, Hearts), Joker))
	if natural.Category != StraightFlush || wild.Category != StraightFlush {
		t.Fatalf("got %v / %v, want straight flushes", natural.Category, wild.Category)
	}
	if Compare(natural, wild) != 0 {
		t.Errorf("a natural and a wild 9-high straight flush must tie")
	}
}

func TestCategoryOrder(t *testing.T) {
	order := []HandCategory{HighCard, Pair, TwoPair, ThreeOfAKind, Straight, Flush, FullHouse,
		FourOfAKind, StraightFlush, FiveOfAKind, RoyalFlush, FiveWilds}
	for i := 1; i < len(order); i++ {
		if order[i] <= order[i-1] {
			t.Fatalf("%v must outrank %v", order[i], order[i-1])
		}
	}
}

// TestTripsReturn enumerates every five-card hand from the 53-card deck and
// checks the Trips pay table keeps a modest house edge. It also logs the
// category distribution behind the pay tables (run with -v).
func TestTripsReturn(t *testing.T) {
	if testing.Short() {
		t.Skip("exhaustive enumeration")
	}
	deck := NewDeck(nil).cards
	var total, tripsReturn float64
	counts := map[HandCategory][2]int{} // [natural, wild]
	var idx [5]int
	for idx[0] = 0; idx[0] < 53; idx[0]++ {
		for idx[1] = idx[0] + 1; idx[1] < 53; idx[1]++ {
			for idx[2] = idx[1] + 1; idx[2] < 53; idx[2]++ {
				for idx[3] = idx[2] + 1; idx[3] < 53; idx[3]++ {
					for idx[4] = idx[3] + 1; idx[4] < 53; idx[4]++ {
						v := Evaluate(h(deck[idx[0]], deck[idx[1]], deck[idx[2]], deck[idx[3]], deck[idx[4]]))
						total++
						n := counts[v.Category]
						if v.Natural {
							n[0]++
						} else {
							n[1]++
						}
						counts[v.Category] = n
						if m, ok := TripsPayout(v); ok {
							tripsReturn += float64(m + 1)
						}
					}
				}
			}
		}
	}
	if total != 2869685 {
		t.Fatalf("enumerated %v hands, want 2869685", total)
	}
	for cat := FiveWilds; cat >= HighCard; cat-- {
		n := counts[cat]
		t.Logf("%-16s natural %8d  wild %8d", cat, n[0], n[1])
	}
	rtp := tripsReturn / total
	t.Logf("Trips return %.4f%% (house edge %.2f%%)", rtp*100, (1-rtp)*100)
	if rtp >= 1 || rtp < 0.9 {
		t.Errorf("Trips return %.4f outside [0.90, 1)", rtp)
	}
}
