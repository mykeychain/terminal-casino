package engine

import "fmt"

// Hand is a set of cards with blackjack value semantics. It is a pure value
// type: it knows nothing about bets, splits, or the game state machine.
type Hand struct {
	Cards []Card
}

// add appends a card to the hand.
func (h *Hand) add(c Card) { h.Cards = append(h.Cards, c) }

// eval computes the best (highest not-busting when possible) total and whether
// the hand is soft. A hand is soft when at least one ace is still counted as 11.
func (h Hand) eval() (total int, soft bool) {
	aces := 0
	for _, c := range h.Cards {
		total += c.Value()
		if c.Rank == Ace {
			aces++
		}
	}
	// Reduce aces from 11 to 1 while busting.
	for total > 21 && aces > 0 {
		total -= 10
		aces--
	}
	return total, aces > 0
}

// Value returns the best blackjack total for the hand (aces reduced as needed).
func (h Hand) Value() int {
	total, _ := h.eval()
	return total
}

// IsSoft reports whether the hand's value counts an ace as 11.
func (h Hand) IsSoft() bool {
	_, soft := h.eval()
	return soft
}

// IsBust reports whether the hand's value exceeds 21.
func (h Hand) IsBust() bool { return h.Value() > 21 }

// IsBlackjack reports whether the hand is a two-card 21. Whether such a hand
// counts as a *natural* (paying 3:2) is decided by the Game, since a two-card
// 21 produced by a split counts as an ordinary 21, not a blackjack.
func (h Hand) IsBlackjack() bool {
	return len(h.Cards) == 2 && h.Value() == 21
}

// Describe returns a human-readable value description, e.g. "soft 17",
// "hard 20", "blackjack", or "bust". UI-friendly but UI-agnostic.
func (h Hand) Describe() string {
	if h.IsBust() {
		return "bust"
	}
	if h.IsBlackjack() {
		return "blackjack"
	}
	v, soft := h.eval()
	if soft {
		return fmt.Sprintf("soft %d", v)
	}
	return fmt.Sprintf("hard %d", v)
}
