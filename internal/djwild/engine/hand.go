package engine

import "sort"

// HandCategory ranks a five-card DJ Wild hand's type. Values are ordered so that
// a higher constant is a stronger hand. With five wild cards in the deck (four
// deuces and the joker) the ladder extends past the usual top: Five of a Kind
// sits between a Straight Flush and a Royal Flush, and Five Wilds — all five
// wild cards — beats everything.
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
	FiveOfAKind                       // five of one rank (needs wilds)
	RoyalFlush                        // 10-J-Q-K-A of one suit (natural or wild)
	FiveWilds                         // the four deuces and the joker
)

// String returns the category's canonical name.
func (c HandCategory) String() string {
	switch c {
	case FiveWilds:
		return "Five Wilds"
	case RoyalFlush:
		return "Royal Flush"
	case FiveOfAKind:
		return "Five of a Kind"
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

// HandValue is the fully evaluated strength of a five-card hand, with every
// wild card already played as whatever makes the hand strongest.
type HandValue struct {
	Category HandCategory
	// Tiebreak holds the ordered comparison key (most significant first), the
	// ranks the hand plays as once its wilds are assigned:
	//   High Card / Flush: the five ranks, descending.
	//   Straight / Straight Flush / Royal Flush: the straight's high rank (5 for
	//     the A-2-3-4-5 wheel).
	//   Five / Four / Three of a Kind, Full House, Two Pair, Pair: the grouped
	//     ranks, largest group first, then kickers descending.
	//   Five Wilds: empty.
	Tiebreak []int

	// Natural reports that the hand makes its category without leaning on a
	// wild card: it holds no joker, and any deuces count as plain Twos. A natural
	// hand ties a wild hand of the same rank head-to-head, but the Trips side bet
	// pays natural hands more.
	Natural bool
	// Wilds is the number of wild cards (deuces and the joker) in the hand.
	Wilds int
}

// Evaluate classifies a five-card hand, playing each wild card as whatever rank
// and suit makes the strongest hand.
func Evaluate(cards [5]Card) HandValue {
	var naturals []Card
	for _, c := range cards {
		if !c.IsWild() {
			naturals = append(naturals, c)
		}
	}
	wilds := 5 - len(naturals)
	if wilds == 5 {
		return HandValue{Category: FiveWilds, Wilds: 5}
	}

	natRanks := make([]int, len(naturals))
	for i, c := range naturals {
		natRanks[i] = int(c.Rank)
	}
	suited := sameSuit(naturals)

	// Try every multiset of ranks for the wild cards (at most C(16,4) = 1820 for
	// four wilds). Suits only matter for a flush, so each assignment is scored
	// off-suit and — when every natural shares a suit and the wilds take distinct
	// new ranks, so they could be real cards of that suit — as a flush too.
	var best HandValue
	found := false
	consider := func(v HandValue) {
		if !found || Compare(v, best) > 0 {
			best, found = v, true
		}
	}
	assign := make([]int, wilds)
	var walk func(i, lo int)
	walk = func(i, lo int) {
		if i == wilds {
			all := append(append(make([]int, 0, 5), natRanks...), assign...)
			consider(classify(all, false))
			if suited && distinct(all) {
				consider(classify(all, true))
			}
			return
		}
		for r := lo; r <= int(Ace); r++ {
			assign[i] = r
			walk(i+1, r)
		}
	}
	walk(0, int(Two))

	best.Wilds = wilds
	best.Natural = isNatural(cards, best.Category)
	return best
}

// isNatural reports whether the hand reaches category without a wild card: no
// joker, and scoring every deuce as a plain Two still makes the same category.
func isNatural(cards [5]Card, category HandCategory) bool {
	ranks := make([]int, 5)
	for i, c := range cards {
		if c.IsJoker() {
			return false
		}
		ranks[i] = int(c.Rank)
	}
	return classify(ranks, sameSuit(cards[:])).Category == category
}

// sameSuit reports whether every card shares one suit (true for no cards).
func sameSuit(cards []Card) bool {
	for _, c := range cards[1:] {
		if c.Suit != cards[0].Suit {
			return false
		}
	}
	return true
}

// distinct reports whether no rank repeats.
func distinct(ranks []int) bool {
	var seen uint32
	for _, r := range ranks {
		if seen&(1<<uint(r)) != 0 {
			return false
		}
		seen |= 1 << uint(r)
	}
	return true
}

// classify scores five plain ranks (wilds already assigned). flush reports that
// all five share a suit. It is the evaluator's inner loop (run once per wild
// assignment), so it avoids maps and allocates only the returned tiebreak.
func classify(ranks []int, flush bool) HandValue {
	var rs [5]int
	copy(rs[:], ranks)
	// Insertion sort, descending.
	for i := 1; i < 5; i++ {
		for j := i; j > 0 && rs[j] > rs[j-1]; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}

	straight, high := straightInfo(rs[:])
	groups := rankGroups(rs)
	byRank := make([]int, len(groups))
	for i, g := range groups {
		byRank[i] = g.rank
	}

	switch {
	case groups[0].count == 5:
		return HandValue{Category: FiveOfAKind, Tiebreak: byRank}
	case straight && flush && high == int(Ace):
		return HandValue{Category: RoyalFlush, Tiebreak: []int{high}}
	case straight && flush:
		return HandValue{Category: StraightFlush, Tiebreak: []int{high}}
	case groups[0].count == 4:
		return HandValue{Category: FourOfAKind, Tiebreak: byRank}
	case groups[0].count == 3 && groups[1].count == 2:
		return HandValue{Category: FullHouse, Tiebreak: byRank}
	case flush:
		return HandValue{Category: Flush, Tiebreak: rs[:]}
	case straight:
		return HandValue{Category: Straight, Tiebreak: []int{high}}
	case groups[0].count == 3:
		return HandValue{Category: ThreeOfAKind, Tiebreak: byRank}
	case groups[0].count == 2 && groups[1].count == 2:
		return HandValue{Category: TwoPair, Tiebreak: byRank}
	case groups[0].count == 2:
		return HandValue{Category: Pair, Tiebreak: byRank}
	default:
		return HandValue{Category: HighCard, Tiebreak: rs[:]}
	}
}

// rankGroup pairs a rank with how many times it appears in a hand.
type rankGroup struct {
	rank  int
	count int
}

// rankGroups counts each rank of a descending hand and returns the groups
// sorted by count descending, then rank descending — tie-break priority order
// for every "of a kind" category. Because the input is already descending, a
// stable sort by count keeps equal-count groups in rank order.
func rankGroups(desc [5]int) []rankGroup {
	groups := make([]rankGroup, 0, 5)
	for _, r := range desc {
		if n := len(groups); n > 0 && groups[n-1].rank == r {
			groups[n-1].count++
		} else {
			groups = append(groups, rankGroup{rank: r, count: 1})
		}
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].count > groups[j].count })
	return groups
}

// straightInfo reports whether five descending ranks form a straight and its
// high rank. The A-2-3-4-5 wheel is the only low-Ace straight (high 5); there
// is no wrap-around.
func straightInfo(desc []int) (bool, int) {
	if !distinct(desc) {
		return false, 0
	}
	if desc[0] == int(Ace) && desc[1] == 5 && desc[4] == 2 {
		return true, 5
	}
	if desc[0]-desc[4] == 4 {
		return true, desc[0]
	}
	return false, 0
}

// Compare orders two evaluated hands: >0 when a is stronger, <0 when b is
// stronger, 0 for a tie. Whether a hand used wild cards never matters here — a
// wild straight flush to the 9 ties a natural one.
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

// Name returns a human-readable description, e.g. "Five Wilds", "Five Aces",
// "Pair of Kings", "Queen-high", "Full House".
func (v HandValue) Name() string {
	switch v.Category {
	case FiveOfAKind:
		return "Five " + pluralRank(Rank(v.Tiebreak[0]))
	case Pair:
		return "Pair of " + pluralRank(Rank(v.Tiebreak[0]))
	case HighCard:
		return Rank(v.Tiebreak[0]).Name() + "-high"
	default:
		return v.Category.String()
	}
}

// pluralRank returns the plural rank name (e.g. "Kings", "Sixes").
func pluralRank(r Rank) string {
	name := r.Name()
	if len(name) > 0 && name[len(name)-1] == 'x' {
		return name + "es" // Six -> Sixes
	}
	return name + "s"
}
