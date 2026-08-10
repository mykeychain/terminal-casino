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

// Rank identifies a card's rank. Numeric values are the card's poker rank, with
// Ace high (14). Straight logic treats the Ace as high, except for the A-2-3-4-5
// wheel where it plays low.
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

// Name returns the full singular rank name (e.g. "King", "Ace", "Two"), used by
// the hand evaluator to build human-readable descriptions.
func (r Rank) Name() string {
	switch r {
	case Two:
		return "Two"
	case Three:
		return "Three"
	case Four:
		return "Four"
	case Five:
		return "Five"
	case Six:
		return "Six"
	case Seven:
		return "Seven"
	case Eight:
		return "Eight"
	case Nine:
		return "Nine"
	case Ten:
		return "Ten"
	case Jack:
		return "Jack"
	case Queen:
		return "Queen"
	case King:
		return "King"
	case Ace:
		return "Ace"
	default:
		return "?"
	}
}

// Card is a single playing card.
type Card struct {
	Rank Rank
	Suit Suit
}

// String returns e.g. "A♠" or "10♥".
func (c Card) String() string { return c.Rank.String() + c.Suit.String() }

// Deck is an ordered set of 52 cards. Mississippi Stud uses one freshly shuffled
// 52-card deck per hand (no shoe, no cut card).
type Deck struct {
	cards []Card
	pos   int
	rng   *rand.Rand
}

// NewDeck returns a fresh, ordered 52-card deck bound to the supplied RNG. Call
// Shuffle before dealing. A nil RNG is allowed for a stacked deck (see
// NewStackedDeck) that is never shuffled.
func NewDeck(rng *rand.Rand) *Deck {
	cards := make([]Card, 0, 52)
	for _, s := range suits {
		for _, r := range ranks {
			cards = append(cards, Card{Rank: r, Suit: s})
		}
	}
	return &Deck{cards: cards, rng: rng}
}

// NewStackedDeck returns a deck with an exact, pre-arranged card order and no
// RNG. Cards are dealt from index 0 upward. Intended for deterministic tests; it
// must not be shuffled.
func NewStackedDeck(cards []Card) *Deck {
	c := make([]Card, len(cards))
	copy(c, cards)
	return &Deck{cards: c}
}

// Shuffle randomizes the deck in place using the owned RNG and resets the deal
// position. It is a no-op when the deck owns no RNG (a stacked deck).
func (d *Deck) Shuffle() {
	d.pos = 0
	if d.rng != nil {
		d.rng.Shuffle(len(d.cards), func(i, j int) {
			d.cards[i], d.cards[j] = d.cards[j], d.cards[i]
		})
	}
}

// draw removes and returns the next card from the front of the deck.
func (d *Deck) draw() Card {
	c := d.cards[d.pos]
	d.pos++
	return c
}

// Remaining reports how many cards are left before the deck is exhausted.
func (d *Deck) Remaining() int { return len(d.cards) - d.pos }
