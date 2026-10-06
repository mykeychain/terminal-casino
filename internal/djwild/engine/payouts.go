package engine

// Pay tables for DJ Wild. Multipliers are the profit paid per unit staked (an
// "X to 1" payout is the integer X). They are exported package-level maps so a
// casino's variant table is a one-line change, and the UI's `?` overlay reads
// them live. All money in the engine is integer.

// BlindPayouts maps the player's final hand to the Blind bet's payout when the
// player BEATS the dealer. A winning hand absent from the table (Three of a Kind
// or lower) pushes the Blind; a losing hand loses it, a tie pushes it.
var BlindPayouts = map[HandCategory]int{
	FiveWilds:     1000,
	RoyalFlush:    50,
	FiveOfAKind:   10,
	StraightFlush: 9,
	FourOfAKind:   4,
	FullHouse:     3,
	Flush:         2,
	Straight:      1,
}

// TripsNaturalPayouts and TripsWildPayouts are the Trips side bet's pay table,
// split by whether the hand is natural (made without a wild card) or wild. Trips
// is resolved on the player's five cards alone — regardless of the dealer and of
// a fold — and wins on Three of a Kind or better. Five of a Kind and Five Wilds
// can only be wild. Exhaustive enumeration of all 2,869,685 hands (see
// TestTripsReturn) puts this table's return at 95.76% (house edge ≈ 4.24%).
var TripsNaturalPayouts = map[HandCategory]int{
	RoyalFlush:    1000,
	StraightFlush: 200,
	FourOfAKind:   60,
	FullHouse:     30,
	Flush:         25,
	Straight:      15,
	ThreeOfAKind:  5,
}

var TripsWildPayouts = map[HandCategory]int{
	FiveWilds:     2000,
	RoyalFlush:    100,
	FiveOfAKind:   100,
	StraightFlush: 20,
	FourOfAKind:   8,
	FullHouse:     6,
	Flush:         4,
	Straight:      3,
	ThreeOfAKind:  1,
}

// TripsPayout returns the Trips profit-per-unit for an evaluated hand and
// whether it wins at all.
func TripsPayout(v HandValue) (int, bool) {
	table := TripsWildPayouts
	if v.Natural {
		table = TripsNaturalPayouts
	}
	m, ok := table[v.Category]
	return m, ok
}

// BlindPayout returns the Blind profit-per-unit for a winning hand, and false
// when the Blind merely pushes on that hand.
func BlindPayout(c HandCategory) (int, bool) {
	m, ok := BlindPayouts[c]
	return m, ok
}
