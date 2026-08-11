package engine

import "math/rand/v2"

// Suit identifies a card's suit.
type Suit int

const (
	Spades Suit = iota
	Hearts
	Diamonds
	Clubs
)

// String returns the suit symbol.
func (s Suit) String() string {
	switch s {
	case Spades:
		return "♠"
	case Hearts:
		return "♥"
	case Diamonds:
		return "♦"
	case Clubs:
		return "♣"
	default:
		return "?"
	}
}

// Red reports whether the suit is drawn in red (hearts/diamonds).
func (s Suit) Red() bool { return s == Hearts || s == Diamonds }

// Rank identifies a card's rank. Numeric values are the card's blackjack base
// count where applicable (Two..Ten); face cards and Ace get distinct constants.
type Rank int

const (
	Two   Rank = 2
	Three Rank = 3
	Four  Rank = 4
	Five  Rank = 5
	Six   Rank = 6
	Seven Rank = 7
	Eight Rank = 8
	Nine  Rank = 9
	Ten   Rank = 10
	Jack  Rank = 11
	Queen Rank = 12
	King  Rank = 13
	Ace   Rank = 14
)

// ranks lists all thirteen ranks in ascending order, used to build decks.
var ranks = []Rank{Two, Three, Four, Five, Six, Seven, Eight, Nine, Ten, Jack, Queen, King, Ace}

// suits lists all four suits, used to build decks.
var suits = []Suit{Spades, Hearts, Diamonds, Clubs}

// String returns the short label for a rank (A, 2..10, J, Q, K).
func (r Rank) String() string {
	switch r {
	case Ace:
		return "A"
	case King:
		return "K"
	case Queen:
		return "Q"
	case Jack:
		return "J"
	case Ten:
		return "10"
	default:
		return string(rune('0' + int(r)))
	}
}

// Card is a single playing card.
type Card struct {
	Rank Rank
	Suit Suit
}

// Value returns the blackjack point value of the card. Ace returns 11 (its
// high value); the Hand type reduces aces to 1 as needed.
func (c Card) Value() int {
	switch c.Rank {
	case Jack, Queen, King:
		return 10
	case Ace:
		return 11
	default:
		return int(c.Rank)
	}
}

// isTenValue reports whether the card counts as ten (10, J, Q, K).
func (c Card) isTenValue() bool {
	return c.Rank == Ten || c.Rank == Jack || c.Rank == Queen || c.Rank == King
}

// String returns e.g. "A♠" or "10♥".
func (c Card) String() string { return c.Rank.String() + c.Suit.String() }

// Deck is an ordered set of 52 cards.
type Deck []Card

// NewDeck returns a fresh, ordered 52-card deck.
func NewDeck() Deck {
	d := make(Deck, 0, 52)
	for _, s := range suits {
		for _, r := range ranks {
			d = append(d, Card{Rank: r, Suit: s})
		}
	}
	return d
}

// CutCardPosition is the penetration depth at which the cut card sits. At
// ~75% of a 6-deck (312-card) shoe this is 234. Once the deal position reaches
// this index, the shoe is reshuffled at the start of the next hand.
const CutCardPosition = 234

// Shoe is a multi-deck shoe that deals cards from the front and tracks a cut
// card. It owns no global state: shuffling uses the injected *rand.Rand.
type Shoe struct {
	cards []Card
	pos   int
	decks int
	rng   *rand.Rand
}

// newShoe builds and shuffles a fresh shoe of the given number of decks using
// the supplied RNG.
func newShoe(decks int, rng *rand.Rand) *Shoe {
	s := &Shoe{decks: decks, rng: rng}
	s.reshuffle()
	return s
}

// newStackedShoe returns a shoe with an exact, pre-arranged card order and no
// RNG. Cards are dealt from index 0 upward. Intended for deterministic tests;
// it must not be allowed to reshuffle (keep deal position below CutCardPosition).
func newStackedShoe(cards []Card) *Shoe {
	c := make([]Card, len(cards))
	copy(c, cards)
	return &Shoe{cards: c, decks: NumDecks}
}

// reshuffle rebuilds the shoe with fresh decks and shuffles them in place using
// the owned RNG, resetting the deal position.
func (s *Shoe) reshuffle() {
	cards := make([]Card, 0, s.decks*52)
	for i := 0; i < s.decks; i++ {
		cards = append(cards, NewDeck()...)
	}
	s.cards = cards
	s.pos = 0
	if s.rng != nil {
		s.rng.Shuffle(len(s.cards), func(i, j int) {
			s.cards[i], s.cards[j] = s.cards[j], s.cards[i]
		})
	}
}

// draw removes and returns the next card from the front of the shoe.
func (s *Shoe) draw() Card {
	c := s.cards[s.pos]
	s.pos++
	return c
}

// Remaining reports how many cards are left before the shoe is exhausted.
func (s *Shoe) Remaining() int { return len(s.cards) - s.pos }

// CutCardReached reports whether the deal position has passed the cut card.
func (s *Shoe) CutCardReached() bool { return s.pos >= CutCardPosition }
