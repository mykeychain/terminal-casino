package engine

// Pay table for Mississippi Stud. The player is paid by a fixed table on the
// TOTAL AMOUNT WAGERED (ante plus every street bet), with no dealer to beat.
// Multipliers are the profit paid per unit staked (an "X to 1" payout is the
// integer X). They are package-level variables rather than constants so a
// regional variant is a one-line change (e.g. the Barona table pays a Straight
// 5:1 instead of 4:1: set StraightPayout = 5). All money in the engine is
// integer.
var (
	RoyalFlushPayout    = 500 // 10-J-Q-K-A suited
	StraightFlushPayout = 100 // suited straight below broadway
	FourOfAKindPayout   = 40
	FullHousePayout     = 10
	FlushPayout         = 6
	StraightPayout      = 4
	ThreeOfAKindPayout  = 3
	TwoPairPayout       = 2
	// PairWinPayout is the profit-per-unit for a qualifying pair (Jacks or
	// better). Pairs below the win tier do not use this (they push or lose).
	PairWinPayout = 1
)

// Pair pay tiers. Unlike the flat multipliers above, a single pair is resolved
// by its rank:
//   - Jacks or better  -> WIN 1:1
//   - Sixes through Tens -> PUSH (stake returned)
//   - Fives or lower    -> LOSS
//
// Everything below a pair (High Card) also loses.
const (
	PairWinFloor  = Jack // lowest pair rank that wins
	PairPushFloor = Six  // lowest pair rank that pushes
)

// HandPayout returns the profit-per-unit multiplier for a flat-paying category
// (Two Pair and above). It reads the live pay-table variables so an overridden
// regional table takes effect immediately. Categories that are not paid flat
// (Pair, High Card) return 0.
func HandPayout(c HandCategory) int {
	switch c {
	case RoyalFlush:
		return RoyalFlushPayout
	case StraightFlush:
		return StraightFlushPayout
	case FourOfAKind:
		return FourOfAKindPayout
	case FullHouse:
		return FullHousePayout
	case Flush:
		return FlushPayout
	case Straight:
		return StraightPayout
	case ThreeOfAKind:
		return ThreeOfAKindPayout
	case TwoPair:
		return TwoPairPayout
	default:
		return 0
	}
}

// payout resolves an evaluated hand to a profit-per-unit multiplier and the
// win/push/loss outcome the pay table awards. The multiplier is meaningful only
// for a win (0 on a push or loss).
func payout(v HandValue) (multiplier int, outcome Outcome) {
	switch {
	case v.Category >= TwoPair:
		return HandPayout(v.Category), OutcomeWin
	case v.Category == Pair:
		switch {
		case v.pairRank >= PairWinFloor:
			return PairWinPayout, OutcomeWin
		case v.pairRank >= PairPushFloor:
			return 0, OutcomePush
		default:
			return 0, OutcomeLoss
		}
	default: // HighCard
		return 0, OutcomeLoss
	}
}
