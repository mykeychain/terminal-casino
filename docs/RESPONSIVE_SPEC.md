# Responsive layout (v2.2) — anchored frame + height density + compact cards

Make the blackjack view fit **any** terminal size instead of assuming ≥24 rows. The
title (top) and the controls (bottom) must always be visible; the table in the middle
compacts to fit. Uses `m.height` (currently ignored) and retires the brittle blank-line
trimming. UI-only; engine untouched.

## The three pieces (do all three)

### 1. Anchored header / body / footer frame
Compose the view to **exactly `m.height` rows × `m.width` cols**, in three zones:
- **Top (fixed):** title (`3:2 Blackjack`) + bankroll header. Pinned to the top.
- **Bottom (fixed):** the phase's control area — the result banner (when shown) + the
  action panel / hint line (or betting panel, insurance prompt, next-hand, game-over).
  Pinned to the bottom.
- **Middle (flex):** the table — dealer area + player area. Fills the space between.

Mechanics (Lip Gloss, no `\n\n` guesswork):
- `topH = lipgloss.Height(top)`, `bottomH = lipgloss.Height(bottom)`.
- `middleH = max(0, m.height - topH - bottomH)`.
- Render the middle (table), size its region to `middleH` (top-align; pad if shorter).
- `lipgloss.JoinVertical(lipgloss.Left, top, middleRegion, bottom)`, then force the whole
  thing to `m.width` × `m.height` (`lipgloss.Place`/`Width`/`Height`).

Result: the title is always row 0, the controls are always the bottom rows, and the frame
never over- or under-flows the terminal.

### 2. Height-aware density
Choose the table's card density from the room available in the middle:
- Render the table at **full density** (today's 5-row cards). If
  `height(tableFull) > middleH`, render it at **compact density** instead.
- If compact *still* exceeds `middleH`, clip the middle to `middleH` (lose bottom table
  rows, never the chrome). A friendly min-size guard is a follow-up, out of scope here.
- Keep the existing **width**-based compact (edge-to-edge cards) orthogonal — it handles
  horizontal overflow; this new tier handles vertical.

### 3. Compact card rendering
A 3-row card variant (vs today's 5), used at compact density:
```
╭────╮
│10♥ │      rank+suit on one line, same red/default tint
╰────╯
```
Face-down back: a 3-row hatch. Add a compact path to the card + hand + dealer renderers
(a density parameter or parallel `…Compact` functions). Halving card-row height is the
main lever that makes the table fit small terminals.

## Behavior notes / decisions to surface
- **Betting screen:** there is no table, so the middle is empty and the betting panel sits
  at the bottom (title top, empty middle, panel bottom). Implement it this way (controls
  always bottom = consistent), but flag it — we may prefer to center the betting panel.
- Reserved fixed-height card rows (from the earlier jump fix) still apply *within* the
  chosen density (5 rows full, 3 rows compact).

## Testing
- **Height sweep:** for betting / player-turn / result / multi-hand, at `m.height` ∈
  {18, 20, 24, 30, 40}: assert `lipgloss.Height(View) == m.height` (exact fit), the title
  is the first line, and the control panel occupies the last lines. Assert compact density
  kicks in at the small heights and full density at the large ones.
- Existing engine / ui / casino tests still pass; only `internal/blackjack32/ui/` changes.

## Optional (recommended follow-up, not required now)
Extract the anchored-frame compose into a small shared helper (`internal/tui`) taking
`(top, middle, bottom, w, h)` so the lobby and future games reuse it. Fine to start inline
in the blackjack UI and lift later.

## Out of scope
Scrolling viewport; the min-size guard message; any engine/rules change.
