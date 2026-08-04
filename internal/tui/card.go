package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/theme"
)

// Card geometry. Every card (face-up or face-down) occupies the same footprint
// so hands line up and "10" never shifts a border.
const (
	CardInnerWidth = 5 // characters between the vertical borders
	RankFieldWidth = 2 // reserved field so "10" and "A" both occupy 2 cells
	CardHeight     = 5 // full-density card height: 3 body rows + 2 border rows

	// CompactCardHeight is a height-density card: a single body row (rank+suit)
	// between the two border rows. Halving the card height is the main lever that
	// lets the table fit short terminals.
	CompactCardHeight = 3
)

// Face is the minimal per-card data the renderer needs — a game converts its own
// engine card into a Face at the UI boundary, so this package stays independent
// of any engine.
type Face struct {
	Rank string // "A", "2".."10", "J", "Q", "K"
	Suit string // "♠" "♥" "♦" "♣"
	Red  bool   // hearts / diamonds — tinted red
}

// Card styles. Every card shares the same soft-white rounded border; only the
// pip (rank + suit glyph) color differs by suit. The face-down back keeps the
// same border and fills with a slate hatch.
var (
	cardRedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.SoftWhite).
			Foreground(theme.Red)

	cardDefaultStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.SoftWhite).
				Foreground(theme.CardPip)

	cardBackStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.SoftWhite).
			Foreground(theme.Slate)
)

// rankField formats a rank label into the fixed 2-cell field so "10" and "A"
// never shift the card borders. right right-justifies it (for the bottom corner).
func rankField(rank string, right bool) string {
	if right {
		return PadLeft(rank, RankFieldWidth)
	}
	return PadRight(rank, RankFieldWidth)
}

// CardHeightFor reports the rendered card height for the chosen vertical density:
// the full 5-row card, or the 3-row compact card. Callers use it to size the
// reserved (fixed-height) card rows so the layout does not jump.
func CardHeightFor(short bool) int {
	if short {
		return CompactCardHeight
	}
	return CardHeight
}

// RenderCard renders a single face-up card wrapped in a rounded border. At full
// density it is a 3-line body: rank in the top-left AND bottom-right (fixed
// 2-char field) with a centered suit. At compact (short) density it collapses to
// a single body row carrying rank+suit together (e.g. "10♥"). Every card shares
// the same soft-white border; only the rank text and suit glyph are tinted — red
// for hearts/diamonds, soft white for spades/clubs.
func RenderCard(f Face, short bool) string {
	style := cardDefaultStyle
	if f.Red {
		style = cardRedStyle
	}
	if short {
		body := PadRight(f.Rank+f.Suit, CardInnerWidth)
		return style.Render(body)
	}
	top := PadRight(rankField(f.Rank, false), CardInnerWidth)
	mid := Center(f.Suit, CardInnerWidth)
	bot := PadLeft(rankField(f.Rank, true), CardInnerWidth)
	body := strings.Join([]string{top, mid, bot}, "\n")
	return style.Render(body)
}

// RenderCardBack renders a face-down card with a distinct hatch pattern so it is
// unmistakably a hidden card. Same footprint as a face-up card at the chosen
// density (3 hatch rows full, 1 hatch row compact).
func RenderCardBack(short bool) string {
	row := strings.Repeat("▚", CardInnerWidth)
	rows := []string{row, row, row}
	if short {
		rows = []string{row}
	}
	return cardBackStyle.Render(strings.Join(rows, "\n"))
}

// handGap is the number of blank columns between adjacent cards. Compact mode
// drops it to keep everything on screen.
func handGap(compact bool) string {
	if compact {
		return ""
	}
	return " "
}

// joinCards lays a slice of pre-rendered card boxes out side by side.
func joinCards(cards []string, compact bool) string {
	if len(cards) == 0 {
		return ""
	}
	gap := handGap(compact)
	parts := make([]string, 0, len(cards)*2-1)
	for i, card := range cards {
		if i > 0 && gap != "" {
			parts = append(parts, gap)
		}
		parts = append(parts, card)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// RenderHand lays out the first count faces as side-by-side face-up cards. The
// feel tier passes a count below len(faces) so the initial deal can cascade in
// one card at a time.
func RenderHand(faces []Face, count int, compact, short bool) string {
	if count > len(faces) {
		count = len(faces)
	}
	if count < 0 {
		count = 0
	}
	boxes := make([]string, count)
	for i := 0; i < count; i++ {
		boxes[i] = RenderCard(faces[i], short)
	}
	return joinCards(boxes, compact)
}

// RenderSlots lays out a hand as total side-by-side slots, of which [0,faceUp)
// are drawn face up (read from faces) and the rest are face-down backs. It draws
// from a fixed total rather than from len(faces), so backs can sit on the felt
// before any card is known (the dealer hole before a reveal) and then flip one
// at a time. Face-up slots beyond len(faces) fall back to a back, so a caller
// can never index past the known cards.
func RenderSlots(faces []Face, faceUp, total int, compact, short bool) string {
	boxes := make([]string, 0, total)
	for i := 0; i < total; i++ {
		if i < faceUp && i < len(faces) {
			boxes = append(boxes, RenderCard(faces[i], short))
		} else {
			boxes = append(boxes, RenderCardBack(short))
		}
	}
	return joinCards(boxes, compact)
}
