package engine

import "sort"

// HandCategory ranks a three-card hand's type. Values are ordered so that a
// higher constant is a stronger hand. NOTE: this is the THREE-card ordering, in
// which a straight beats a flush (unlike five-card poker).
type HandCategory int

const (
	HighCard      HandCategory = iota // no pair, straight, or flush
	Pair                              // two cards of equal rank
	Flush                             // three cards of one suit, not a straight
	Straight                          // three sequential ranks, mixed suits
	ThreeOfAKind                      // three cards of equal rank
	StraightFlush                     // straight, all one suit
)

// String returns the category's canonical name.
func (c HandCategory) String() string {
	switch c {
	case StraightFlush:
		return "Straight Flush"
	case ThreeOfAKind:
		return "Three of a Kind"
	case Straight:
		return "Straight"
	case Flush:
		return "Flush"
	case Pair:
		return "Pair"
	case HighCard:
		return "High Card"
	default:
		return "Unknown"
	}
}

// HandValue is the fully evaluated strength of a three-card hand. Category is
// the primary key; Tiebreak is an ordered list of ranks (most significant
// first) used to break ties within a category. Standard suit ranking is never
// used, so two hands with equal Category and Tiebreak are an exact tie.
type HandValue struct {
	Category HandCategory
	// Tiebreak holds the ordered comparison key:
	//   High Card / Flush: the three ranks, descending.
	//   Straight / Straight Flush: a single element, the straight's high rank
	//     (3 for the A-2-3 wheel, since the Ace plays low).
	//   Three of a Kind: a single element, the tripled rank.
	//   Pair: the pair rank, then the kicker.
	Tiebreak []int

	// pairRank / highRank feed the human-readable Name; both are set during
	// evaluation.
	pairRank Rank
	highRank Rank
}

// Evaluate classifies a three-card hand and computes its comparison key.
func Evaluate(cards [3]Card) HandValue {
	rs := []int{int(cards[0].Rank), int(cards[1].Rank), int(cards[2].Rank)}
	sort.Sort(sort.Reverse(sort.IntSlice(rs))) // descending

	flush := cards[0].Suit == cards[1].Suit && cards[1].Suit == cards[2].Suit
	straight, straightHigh := straightInfo(rs)

	switch {
	case straight && flush:
		return HandValue{
			Category: StraightFlush,
			Tiebreak: []int{straightHigh},
			highRank: Rank(straightHigh),
		}
	case rs[0] == rs[1] && rs[1] == rs[2]:
		return HandValue{
			Category: ThreeOfAKind,
			Tiebreak: []int{rs[0]},
			highRank: Rank(rs[0]),
		}
	case straight:
		return HandValue{
			Category: Straight,
			Tiebreak: []int{straightHigh},
			highRank: Rank(straightHigh),
		}
	case flush:
		return HandValue{
			Category: Flush,
			Tiebreak: []int{rs[0], rs[1], rs[2]},
			highRank: Rank(rs[0]),
		}
	}

	// Pair detection: find the rank that appears twice, plus the kicker.
	if pair, kicker, ok := pairInfo(rs); ok {
		return HandValue{
			Category: Pair,
			Tiebreak: []int{pair, kicker},
			pairRank: Rank(pair),
			highRank: Rank(rs[0]),
		}
	}

	return HandValue{
		Category: HighCard,
		Tiebreak: []int{rs[0], rs[1], rs[2]},
		highRank: Rank(rs[0]),
	}
}

// straightInfo reports whether the descending ranks form a three-card straight
// and returns the straight's high rank. The Ace is high or low: Q-K-A is the
// highest straight (high 14) and A-2-3 the lowest (high 3, Ace playing low).
// K-A-2 is NOT a straight.
func straightInfo(desc []int) (bool, int) {
	a, b, c := desc[0], desc[1], desc[2]
	// A-2-3 wheel: ranks arrive as {14,3,2}. The Ace plays low, high card is 3.
	if a == int(Ace) && b == 3 && c == 2 {
		return true, 3
	}
	if a-1 == b && b-1 == c {
		return true, a
	}
	return false, 0
}

// pairInfo returns the paired rank and the kicker for a descending rank slice
// that contains exactly one pair. ok is false when there is no pair.
func pairInfo(desc []int) (pair, kicker int, ok bool) {
	switch {
	case desc[0] == desc[1]:
		return desc[0], desc[2], true
	case desc[1] == desc[2]:
		return desc[1], desc[0], true
	default:
		return 0, 0, false
	}
}

// Compare orders two evaluated hands: >0 when a is stronger, <0 when b is
// stronger, 0 for an exact tie (a push). It compares Category first, then the
// Tiebreak key element by element.
func Compare(a, b HandValue) int {
	if a.Category != b.Category {
		return int(a.Category) - int(b.Category)
	}
	for i := 0; i < len(a.Tiebreak) && i < len(b.Tiebreak); i++ {
		if a.Tiebreak[i] != b.Tiebreak[i] {
			return a.Tiebreak[i] - b.Tiebreak[i]
		}
	}
	return 0
}

// Name returns a human-readable description for showdown display, e.g.
// "Straight Flush", "Three of a Kind", "Straight", "Flush", "Pair of Kings",
// "Queen-high", "Ace-high".
func (v HandValue) Name() string {
	switch v.Category {
	case StraightFlush, ThreeOfAKind, Straight, Flush:
		return v.Category.String()
	case Pair:
		return "Pair of " + pluralRank(v.pairRank)
	default: // HighCard
		return v.highRank.Name() + "-high"
	}
}

// pluralRank returns the plural rank name used for pair descriptions
// (e.g. "Kings", "Sixes").
func pluralRank(r Rank) string {
	name := r.Name()
	if len(name) > 0 && name[len(name)-1] == 'x' {
		return name + "es" // Six -> Sixes
	}
	return name + "s"
}
