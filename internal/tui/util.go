// Package tui holds the engine-agnostic Bubble Tea / Lip Gloss chrome shared by
// every casino game's front end: text padding, the anchored-frame height fit,
// cursor clamping, the titled action panel, the horizontal menu, and card
// rendering. It imports internal/theme for color and nothing game-specific, so a
// game's ui package composes these helpers rather than re-implementing them.
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HandIndent is how far free-standing rows (a result banner, a settlement line)
// sit from the left so they align with the padded card rows.
const HandIndent = "  "

// PadRight returns s padded with spaces on the right to width display cells.
func PadRight(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// PadLeft returns s padded with spaces on the left to width display cells.
func PadLeft(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return strings.Repeat(" ", gap) + s
	}
	return s
}

// Center returns s centered within width display cells.
func Center(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	left := gap / 2
	right := gap - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// ClampIdx pins i into [0, n) (returns 0 for an empty menu).
func ClampIdx(i, n int) int {
	if n <= 0 || i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// FitHeight pads s with blank lines (top-aligned) up to h rows, or clips it to
// the first h rows when it is taller. It is how the anchored frame's middle
// region is sized and how the whole frame is forced to exactly the terminal
// height.
func FitHeight(s string, h int) string {
	if h <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
