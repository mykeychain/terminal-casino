package engine

import (
	"fmt"
	"sort"
)

// In I Luv Suits only one thing about a seven-card hand matters for the main
// game: its best flush. A hand's flush is the set of its cards in a single suit;
// the best flush is the suit holding the MOST cards, and among suits of equal
// length the one whose cards rank highest (compared high card first, then the
// next, and so on). Pairs, straights and the like count for nothing — a
// straight flush is just a flush, valued by its length and its cards.

// FlushHand is the evaluated best flush of a seven-card hand.
type FlushHand struct {
	Suit  Suit   // the flush's suit
	Ranks []Rank // the flush's ranks, highest first; len(Ranks) is the flush length
}

// Len is the number of cards in the flush (2 through 7 for a seven-card hand).
func (f FlushHand) Len() int { return len(f.Ranks) }

// High is the flush's top card rank (0 for an empty flush).
func (f FlushHand) High() Rank {
	if len(f.Ranks) == 0 {
		return 0
	}
	return f.Ranks[0]
}

// Name returns a human-readable description, e.g. "5-card ♥ flush, King-high".
func (f FlushHand) Name() string {
	if len(f.Ranks) == 0 {
		return ""
	}
	return fmt.Sprintf("%d-card %s flush, %s-high", f.Len(), f.Suit, f.High().Name())
}

// Evaluate finds the best flush among the cards (normally seven).
func Evaluate(cards []Card) FlushHand {
	var best FlushHand
	for _, s := range suits {
		f := FlushHand{Suit: s, Ranks: suitRanks(cards, s)}
		if len(f.Ranks) == 0 {
			continue
		}
		if len(best.Ranks) == 0 || Compare(f, best) > 0 {
			best = f
		}
	}
	return best
}

// suitRanks returns the ranks of the cards in suit s, highest first.
func suitRanks(cards []Card, s Suit) []Rank {
	var rs []Rank
	for _, c := range cards {
		if c.Suit == s {
			rs = append(rs, c.Rank)
		}
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i] > rs[j] })
	return rs
}

// Compare orders two flushes: >0 when a is stronger, <0 when b is stronger, 0
// for an exact tie (a push). More cards wins outright; at equal length the ranks
// are compared from the top down. Suits never break a tie.
func Compare(a, b FlushHand) int {
	if a.Len() != b.Len() {
		return a.Len() - b.Len()
	}
	for i := range a.Ranks {
		if a.Ranks[i] != b.Ranks[i] {
			return int(a.Ranks[i]) - int(b.Ranks[i])
		}
	}
	return 0
}

// Qualifies reports whether a dealer flush qualifies: a three-card flush that
// is Nine-high or better, or any flush of four or more cards.
func Qualifies(f FlushHand) bool {
	switch {
	case f.Len() >= 4:
		return true
	case f.Len() == 3:
		return f.High() >= QualifyHigh
	default:
		return false
	}
}

// StraightFlushLen returns the length of the longest run of consecutive ranks
// within a single suit — the hand's longest straight flush, which drives the
// Super Flush Rush side bet. The Ace plays high (Q-K-A) or low (A-2-3); there is
// no wrap-around (K-A-2 is not a run). Every hand has a run of at least 1.
func StraightFlushLen(cards []Card) int {
	best := 0
	for _, s := range suits {
		var mask uint32 // bit r set when rank r (2..14) is held in suit s
		for _, c := range cards {
			if c.Suit == s {
				mask |= 1 << uint(c.Rank)
			}
		}
		if mask&(1<<uint(Ace)) != 0 {
			mask |= 1 << 1 // the Ace also plays low, below the Two
		}
		// Each AND with the shifted mask shortens every run by one; the number of
		// steps until the mask empties is the longest run.
		n := 0
		for m := mask; m != 0; m &= m << 1 {
			n++
		}
		if n > best {
			best = n
		}
	}
	return best
}

// SortForDisplay returns the cards grouped by suit, the best flush's suit first
// and the remaining suits by descending length, each group highest rank first.
// It is a presentation helper: the flush the hand is judged on reads left to
// right at the front of the hand.
func SortForDisplay(cards []Card) []Card {
	best := Evaluate(cards)
	count := map[Suit]int{}
	for _, c := range cards {
		count[c.Suit]++
	}
	out := append([]Card(nil), cards...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Suit != b.Suit {
			if a.Suit == best.Suit || b.Suit == best.Suit {
				return a.Suit == best.Suit
			}
			if count[a.Suit] != count[b.Suit] {
				return count[a.Suit] > count[b.Suit]
			}
			return a.Suit < b.Suit
		}
		return a.Rank > b.Rank
	})
	return out
}
