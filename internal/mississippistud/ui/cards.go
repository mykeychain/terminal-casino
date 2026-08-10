package ui

import (
	"strings"

	"github.com/mykeychain/terminal-casino/internal/mississippistud/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// face converts an engine card into the shared renderer's neutral Face, the only
// coupling between this game's engine and the shared card renderer.
func face(c engine.Card) tui.Face {
	return tui.Face{Rank: c.Rank.String(), Suit: c.Suit.String(), Red: c.Suit.Red()}
}

// faces converts a slice of engine cards into Faces for tui.RenderHand.
func faces(cs []engine.Card) []tui.Face {
	out := make([]tui.Face, len(cs))
	for i, c := range cs {
		out[i] = face(c)
	}
	return out
}

// communityFaces converts the three community cards into Faces, dropping the
// per-card reveal flag (the caller passes a faceUp count to tui.RenderSlots).
func communityFaces(cs []engine.CommunityCard) []tui.Face {
	out := make([]tui.Face, len(cs))
	for i, c := range cs {
		out[i] = face(c.Card)
	}
	return out
}

// revealedCount reports how many of the community cards are currently face up.
// It is the faceUp argument for tui.RenderSlots, so the three backs flip one at a
// time as each street's Raise reveals a card.
func revealedCount(cs []engine.CommunityCard) int {
	n := 0
	for _, c := range cs {
		if c.Revealed {
			n++
		}
	}
	return n
}

// highestVisiblePair returns the plural name of the highest pair among the
// currently visible cards (both hole cards plus any revealed community cards),
// and whether such a pair exists. It feeds the subtle made-hand hint, naming the
// pair that has already locked in a win or push floor.
func highestVisiblePair(hole []engine.Card, community []engine.CommunityCard) (string, bool) {
	counts := map[engine.Rank]int{}
	for _, c := range hole {
		counts[c.Rank]++
	}
	for _, c := range community {
		if c.Revealed {
			counts[c.Card.Rank]++
		}
	}
	best, found := engine.Rank(0), false
	for r, n := range counts {
		if n >= 2 && r > best {
			best, found = r, true
		}
	}
	if !found {
		return "", false
	}
	return pluralRank(best), true
}

// pluralRank turns a rank into its plural display name (e.g. "Kings", "Sixes"),
// matching the engine's own pair phrasing.
func pluralRank(r engine.Rank) string {
	name := r.Name()
	if strings.HasSuffix(name, "x") {
		return name + "es" // Six -> Sixes
	}
	return name + "s"
}
