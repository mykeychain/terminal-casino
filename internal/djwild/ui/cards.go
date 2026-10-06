package ui

import (
	"strings"

	"github.com/mykeychain/terminal-casino/internal/djwild/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// face converts an engine card into the shared renderer's neutral Face, the only
// coupling between this game's engine and the shared card renderer. The joker
// renders as "JK" with a red star.
func face(c engine.Card) tui.Face {
	if c.IsJoker() {
		return tui.Face{Rank: "JK", Suit: c.Suit.String(), Red: true}
	}
	return tui.Face{Rank: c.Rank.String(), Suit: c.Suit.String(), Red: c.Suit.Red()}
}

// faces converts a slice of engine cards into Faces for tui.RenderHand / RenderSlots.
func faces(cs []engine.Card) []tui.Face {
	out := make([]tui.Face, len(cs))
	for i, c := range cs {
		out[i] = face(c)
	}
	return out
}

// cardWidth is the rendered width of one card (its inner body plus the two
// vertical border columns).
const cardWidth = tui.CardInnerWidth + 2

// wildMarker labels each wild card among the first shown cards with a gold
// "wild" centered beneath it. Gaps match the card row's (none when compact) so
// the labels line up with the cards above. It is empty when no shown card is
// wild.
func wildMarker(cards []engine.Card, shown int, compact bool) string {
	if shown > len(cards) {
		shown = len(cards)
	}
	gap := " "
	if compact {
		gap = ""
	}
	var b strings.Builder
	any := false
	for i := 0; i < shown; i++ {
		if i > 0 {
			b.WriteString(gap)
		}
		if cards[i].IsWild() {
			b.WriteString(markerStyle.Render(tui.Center("wild", cardWidth)))
			any = true
		} else {
			b.WriteString(strings.Repeat(" ", cardWidth))
		}
	}
	if !any {
		return ""
	}
	return b.String()
}
