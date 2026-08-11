package ui

import (
	"github.com/mykeychain/terminal-casino/internal/blackjackfreebet/engine"
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
