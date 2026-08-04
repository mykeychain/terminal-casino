package engine

// Payout tables for Three-Card Poker. All multipliers are expressed as the
// profit paid per unit staked (an "X:1" payout is the integer X). They are
// exported and defined as keyed tables so the odds are easy to tune in one
// place. All money in the engine is integer.

// AnteBonusMultipliers maps a qualifying hand category to its Ante Bonus payout,
// expressed as profit per unit of the Ante. The bonus is paid on the Ante
// whenever the player Played (not on a fold), regardless of whether the dealer
// qualifies or wins. Categories absent from the table pay no bonus.
//
// Straight 1:1, Three of a Kind 4:1, Straight Flush 5:1.
var AnteBonusMultipliers = map[HandCategory]int{
	Straight:      1,
	ThreeOfAKind:  4,
	StraightFlush: 5,
}

// PairPlusMultipliers maps a hand category to its Pair Plus payout, expressed as
// profit per unit of the Pair Plus wager. Pair Plus is resolved on the player's
// dealt hand alone, independent of the dealer and of fold/play. Anything below a
// pair (a High Card) is absent and loses.
//
// Straight Flush 40:1, Three of a Kind 30:1, Straight 6:1, Flush 4:1, Pair 1:1
// (the classic/original Pair Plus table).
var PairPlusMultipliers = map[HandCategory]int{
	StraightFlush: 40,
	ThreeOfAKind:  30,
	Straight:      6,
	Flush:         4,
	Pair:          1,
}

// anteBonusMultiplier returns the Ante Bonus profit-per-unit for a category
// (0 when the category earns no bonus).
func anteBonusMultiplier(c HandCategory) int { return AnteBonusMultipliers[c] }

// pairPlusMultiplier returns the Pair Plus profit-per-unit for a category, and
// whether the category wins any Pair Plus payout at all.
func pairPlusMultiplier(c HandCategory) (int, bool) {
	m, ok := PairPlusMultipliers[c]
	return m, ok
}
