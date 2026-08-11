package engine

import "testing"

func cards(rs ...Rank) []Card {
	c := make([]Card, len(rs))
	for i, r := range rs {
		c[i] = Card{Rank: r, Suit: Spades}
	}
	return c
}

func TestHandValue(t *testing.T) {
	tests := []struct {
		name  string
		cards []Rank
		value int
		soft  bool
		bust  bool
		bj    bool
	}{
		{"hard 20", []Rank{Ten, King}, 20, false, false, false},
		{"blackjack A+K", []Rank{Ace, King}, 21, true, false, true},
		{"blackjack A+10", []Rank{Ten, Ace}, 21, true, false, true},
		{"soft 17 A+6", []Rank{Ace, Six}, 17, true, false, false},
		{"hard 17 A+6+10", []Rank{Ace, Six, Ten}, 17, false, false, false},
		{"pair of aces = soft 12", []Rank{Ace, Ace}, 12, true, false, false},
		{"three aces = soft 13", []Rank{Ace, Ace, Ace}, 13, true, false, false},
		{"A+A+9 = soft 21 not bj", []Rank{Ace, Ace, Nine}, 21, true, false, false},
		{"four aces + 7 = soft 21", []Rank{Ace, Ace, Ace, Ace, Seven}, 21, true, false, false},
		{"five aces + 6 = soft 21", []Rank{Ace, Ace, Ace, Ace, Ace, Six}, 21, true, false, false},
		{"bust 10+5+10", []Rank{Ten, Five, Ten}, 25, false, true, false},
		{"soft becomes hard A+9+K", []Rank{Ace, Nine, King}, 20, false, false, false},
		{"21 three cards not bj 7+7+7", []Rank{Seven, Seven, Seven}, 21, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := Hand{Cards: cards(tc.cards...)}
			if got := h.Value(); got != tc.value {
				t.Errorf("Value = %d, want %d", got, tc.value)
			}
			if got := h.IsSoft(); got != tc.soft {
				t.Errorf("IsSoft = %v, want %v", got, tc.soft)
			}
			if got := h.IsBust(); got != tc.bust {
				t.Errorf("IsBust = %v, want %v", got, tc.bust)
			}
			if got := h.IsBlackjack(); got != tc.bj {
				t.Errorf("IsBlackjack = %v, want %v", got, tc.bj)
			}
		})
	}
}

func TestHandDescribe(t *testing.T) {
	tests := []struct {
		cards []Rank
		want  string
	}{
		{[]Rank{Ace, Six}, "soft 17"},
		{[]Rank{Ten, King}, "hard 20"},
		{[]Rank{Ace, King}, "blackjack"},
		{[]Rank{Ten, Five, Ten}, "bust"},
	}
	for _, tc := range tests {
		h := Hand{Cards: cards(tc.cards...)}
		if got := h.Describe(); got != tc.want {
			t.Errorf("Describe(%v) = %q, want %q", tc.cards, got, tc.want)
		}
	}
}
