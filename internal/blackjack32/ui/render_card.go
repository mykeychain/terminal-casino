package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/blackjack32/engine"
)

// padRight returns s padded with spaces on the right to `width` display cells.
func padRight(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// padLeft returns s padded with spaces on the left to `width` display cells.
func padLeft(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return strings.Repeat(" ", gap) + s
	}
	return s
}

// center returns s centered within `width` display cells.
func center(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	left := gap / 2
	right := gap - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// rankField formats a rank into the fixed 2-cell field so "10" and "A" never
// shift the card borders. `right` right-justifies it (for the bottom corner).
func rankField(r engine.Rank, right bool) string {
	label := r.String() // "A", "2".."10", "J", "Q", "K"
	if right {
		return padLeft(label, rankFieldWidth)
	}
	return padRight(label, rankFieldWidth)
}

// renderCard renders a single face-up card as a 3-line body wrapped in a rounded
// border: rank in the top-left AND bottom-right (fixed 2-char field) with a
// centered suit. Every card shares the same soft-white border; only the rank
// text and suit glyph are tinted — red for hearts/diamonds, soft white for
// spades/clubs.
func renderCard(c engine.Card) string {
	top := padRight(rankField(c.Rank, false), cardInnerWidth)
	mid := center(c.Suit.String(), cardInnerWidth)
	bot := padLeft(rankField(c.Rank, true), cardInnerWidth)
	body := strings.Join([]string{top, mid, bot}, "\n")

	style := cardDefaultStyle
	if c.Suit.Red() {
		style = cardRedStyle
	}
	return style.Render(body)
}

// renderCardBack renders a face-down card with a distinct hatch pattern so it is
// unmistakably a hidden card. Same footprint as a face-up card.
func renderCardBack() string {
	row := strings.Repeat("▚", cardInnerWidth)
	body := strings.Join([]string{row, row, row}, "\n")
	return cardBackStyle.Render(body)
}

// handGap is the number of blank columns between adjacent cards. Compact mode
// (used when up to four split hands would overflow) drops it to keep everything
// on screen.
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

// renderHand lays out a player hand as side-by-side face-up cards.
func renderHand(cards []engine.Card, compact bool) string {
	boxes := make([]string, len(cards))
	for i, c := range cards {
		boxes[i] = renderCard(c)
	}
	return joinCards(boxes, compact)
}

// renderDealerHand lays out the dealer hand. While the hole card is hidden, the
// up-card is shown face up and every other card is a face-down back.
func renderDealerHand(dv engine.DealerView, compact bool) string {
	boxes := make([]string, 0, len(dv.Cards))
	for i, c := range dv.Cards {
		if !dv.Revealed && i > 0 {
			boxes = append(boxes, renderCardBack())
			continue
		}
		boxes = append(boxes, renderCard(c))
	}
	return joinCards(boxes, compact)
}
