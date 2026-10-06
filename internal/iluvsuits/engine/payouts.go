package engine

// Pay tables and table rules for I Luv Suits. Multipliers are the profit paid
// per unit staked (an "X to 1" payout is the integer X). They are exported
// package-level variables so a casino's variant table is a one-line change, and
// the UI's `?` overlay reads them live. All money in the engine is integer.

// QualifyHigh is the lowest top card that qualifies a dealer three-card flush
// (a three-card flush Nine-high or better; any four-card flush qualifies).
const QualifyHigh = Nine

// FlushRushPayouts maps the player's flush length to the Flush Rush side bet's
// payout. It is resolved on the player's seven cards alone, regardless of the
// dealer and of a fold. Lengths absent from the table (three or fewer) lose.
//
// 7 cards 100:1, 6 cards 20:1, 5 cards 10:1, 4 cards 2:1 (house edge ≈ 5.30%,
// by exhaustive enumeration of all 133,784,560 seven-card hands).
var FlushRushPayouts = map[int]int{
	7: 100,
	6: 20,
	5: 10,
	4: 2,
}

// SuperFlushRushPayouts maps the player's longest straight flush to the Super
// Flush Rush side bet's payout. Like Flush Rush it is resolved on the player's
// seven cards alone. Runs shorter than three cards lose.
//
// 7 cards 500:1, 6 cards 200:1, 5 cards 100:1, 4 cards 50:1, 3 cards 9:1
// (house edge ≈ 6.35%, by exhaustive enumeration).
var SuperFlushRushPayouts = map[int]int{
	7: 500,
	6: 200,
	5: 100,
	4: 50,
	3: 9,
}

// MaxPlayMultiple is the largest Play wager, as a multiple of the Ante, that a
// flush of the given length permits: 3× for a six- or seven-card flush, 2× for
// a five-card flush, and 1× for anything shorter.
func MaxPlayMultiple(flushLen int) int {
	switch {
	case flushLen >= 6:
		return 3
	case flushLen == 5:
		return 2
	default:
		return 1
	}
}
