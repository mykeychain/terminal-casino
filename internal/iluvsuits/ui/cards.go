package ui

import (
	"strings"

	"github.com/mykeychain/terminal-casino/internal/iluvsuits/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// face converts an engine card into the shared renderer's neutral Face, the only
// coupling between this game's engine and the shared card renderer.
func face(c engine.Card) tui.Face {
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

// flushMarker draws a gold rule under the first n of shown card slots — the
// cards that make up the hand's flush. Hands are displayed grouped by suit with
// the best flush first (engine.SortForDisplay), so the rule always underlines a
// contiguous run from the left edge. Gaps match the card row's (none when
// compact) so the rule lines up with the cards above it.
func flushMarker(n, shown int, compact bool) string {
	if n > shown {
		n = shown
	}
	if n <= 0 {
		return ""
	}
	gap := " "
	if compact {
		gap = ""
	}
	span := n*cardWidth + (n-1)*len(gap)
	return markerStyle.Render(strings.Repeat("▔", span))
}
