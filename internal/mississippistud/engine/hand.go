package engine

import "sort"

// HandCategory ranks a five-card hand's type. Values are ordered so that a
// higher constant is a stronger hand. This is the standard FIVE-card ordering
// (a flush beats a straight, unlike the three-card game). Royal Flush is kept
// distinct from a lower Straight Flush because they pay differently (500 vs 100).
type HandCategory int

const (
	HighCard      HandCategory = iota // no pair, straight, or flush
	Pair                              // one pair
	TwoPair                           // two distinct pairs
	ThreeOfAKind                      // three of a kind
	Straight                          // five sequential ranks, mixed suits
	Flush                             // five cards of one suit, not sequential
	FullHouse                         // three of a kind plus a pair
	FourOfAKind                       // four of a kind
	StraightFlush                     // straight, all one suit (below broadway)
	RoyalFlush                        // 10-J-Q-K-A of one suit
)

// String returns the category's canonical name.
func (c HandCategory) String() string {
	switch c {
	case RoyalFlush:
		return "Royal Flush"
	case StraightFlush:
		return "Straight Flush"
	case FourOfAKind:
		return "Four of a Kind"
	case FullHouse:
		return "Full House"
	case Flush:
		return "Flush"
	case Straight:
		return "Straight"
	case ThreeOfAKind:
		return "Three of a Kind"
	case TwoPair:
		return "Two Pair"
	case Pair:
		return "Pair"
	case HighCard:
		return "High Card"
	default:
		return "Unknown"
	}
}

// HandValue is the fully evaluated strength of a five-card hand. Category is the
// primary key; Tiebreak is an ordered list of ranks (most significant first)
// used to break ties within a category. Standard suit ranking is never used, so
// two hands with equal Category and Tiebreak are an exact tie.
type HandValue struct {
	Category HandCategory
	// Tiebreak holds the ordered comparison key:
	//   High Card / Flush: the five ranks, descending.
	//   Straight / Straight Flush / Royal Flush: a single element, the straight's
	//     high rank (5 for the A-2-3-4-5 wheel, since the Ace plays low).
	//   Four of a Kind: the quad rank, then the kicker.
	//   Full House: the trip rank, then the pair rank.
	//   Three of a Kind: the trip rank, then the two kickers descending.
	//   Two Pair: the high pair, the low pair, then the kicker.
	//   Pair: the pair rank, then the three kickers descending.
	Tiebreak []int

	// pairRank / highRank feed the human-readable Name and the Pair pay tier;
	// both are set during evaluation. pairRank is the rank of the (single) pair
	// for the Pair category and is 0 for every other category.
	pairRank Rank
	highRank Rank
}

// Evaluate classifies a five-card hand and computes its comparison key.
func Evaluate(cards [5]Card) HandValue {
	rs := make([]int, 5)
	for i, card := range cards {
		rs[i] = int(card.Rank)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(rs))) // descending

	flush := true
	for i := 1; i < 5; i++ {
		if cards[i].Suit != cards[0].Suit {
			flush = false
			break
		}
	}
	straight, straightHigh := straightInfo(rs)

	// Group ranks by frequency. groups is sorted by count descending, then rank
	// descending, so its ranks are already in tie-break priority order for every
	// "of a kind" category (quads, full house, trips, two pair, one pair).
	groups := rankGroups(rs)
	byRank := make([]int, len(groups))
	for i, g := range groups {
		byRank[i] = g.rank
	}

	switch {
	case straight && flush:
		// A broadway straight flush (high card Ace) is the Royal Flush.
		if straightHigh == int(Ace) {
			return HandValue{Category: RoyalFlush, Tiebreak: []int{straightHigh}, highRank: Ace}
		}
		return HandValue{Category: StraightFlush, Tiebreak: []int{straightHigh}, highRank: Rank(straightHigh)}
	case groups[0].count == 4:
		return HandValue{Category: FourOfAKind, Tiebreak: byRank, highRank: Rank(groups[0].rank)}
	case groups[0].count == 3 && groups[1].count == 2:
		return HandValue{Category: FullHouse, Tiebreak: byRank, highRank: Rank(groups[0].rank)}
	case flush:
		return HandValue{Category: Flush, Tiebreak: rs, highRank: Rank(rs[0])}
	case straight:
		return HandValue{Category: Straight, Tiebreak: []int{straightHigh}, highRank: Rank(straightHigh)}
	case groups[0].count == 3:
		return HandValue{Category: ThreeOfAKind, Tiebreak: byRank, highRank: Rank(groups[0].rank)}
	case groups[0].count == 2 && groups[1].count == 2:
		return HandValue{Category: TwoPair, Tiebreak: byRank, highRank: Rank(groups[0].rank)}
	case groups[0].count == 2:
		return HandValue{
			Category: Pair,
			Tiebreak: byRank,
			pairRank: Rank(groups[0].rank),
			highRank: Rank(rs[0]),
		}
	default:
		return HandValue{Category: HighCard, Tiebreak: rs, highRank: Rank(rs[0])}
	}
}

// rankGroup pairs a rank with how many times it appears in a hand.
type rankGroup struct {
	rank  int
	count int
}

// rankGroups counts each rank in a descending rank slice and returns the groups
// sorted by count descending, breaking ties by rank descending.
func rankGroups(desc []int) []rankGroup {
	counts := map[int]int{}
	for _, r := range desc {
		counts[r]++
	}
	groups := make([]rankGroup, 0, len(counts))
	for r, n := range counts {
		groups = append(groups, rankGroup{rank: r, count: n})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count > groups[j].count
		}
		return groups[i].rank > groups[j].rank
	})
	return groups
}

// straightInfo reports whether the five descending ranks form a straight and
// returns the straight's high rank. Ranks must be distinct (a duplicate rank is
// never a straight). The Ace is high, so 10-J-Q-K-A is the highest straight
// (high 14); the A-2-3-4-5 wheel is the only low-Ace straight (high 5). There is
// no wrap-around: Q-K-A-2-3 and K-A-2-3-4 are NOT straights.
func straightInfo(desc []int) (bool, int) {
	for i := 0; i+1 < len(desc); i++ {
		if desc[i] == desc[i+1] {
			return false, 0 // a repeated rank cannot be a straight
		}
	}
	// Wheel: ranks arrive as {14,5,4,3,2}. The Ace plays low, high card is 5.
	if desc[0] == int(Ace) && desc[1] == 5 && desc[2] == 4 && desc[3] == 3 && desc[4] == 2 {
		return true, 5
	}
	// Distinct ranks spanning exactly four apart are consecutive.
	if desc[0]-desc[4] == 4 {
		return true, desc[0]
	}
	return false, 0
}

// Compare orders two evaluated hands: >0 when a is stronger, <0 when b is
// stronger, 0 for an exact tie. It compares Category first, then the Tiebreak
// key element by element.
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

// PairRank returns the rank of the hand's single pair (valid for the Pair
// category, 0 otherwise). The pay table uses it to decide the pair tier.
func (v HandValue) PairRank() Rank { return v.pairRank }

// Name returns a human-readable description for display, e.g. "Royal Flush",
// "Four of a Kind", "Full House", "Pair of Kings", "King-high", "Ace-high".
func (v HandValue) Name() string {
	switch v.Category {
	case Pair:
		return "Pair of " + pluralRank(v.pairRank)
	case HighCard:
		return v.highRank.Name() + "-high"
	default:
		return v.Category.String()
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
