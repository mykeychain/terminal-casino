# Mississippi Stud

A house-banked, five-card stud poker table. There is **no dealer and no opponent** —
you build one five-card hand from **2 hole cards + 3 community cards** and are paid
strictly by a fixed pay table. The only decisions are, at each of three betting
streets, whether to **fold** or **raise 1× / 2× / 3× the ante** as the community
cards are revealed one at a time.

The tension is that bets **compound**. Raise 3× on every street and you have **10×
your ante** riding on one hand (1 ante + 3 + 3 + 3). Folding forfeits everything
wagered so far — but folding is often the mathematically correct play.

Single-player vs. the pay table, matching the other tables. Fresh **$1000** bankroll
per session, no persistence.

## Game flow

1. **Ante** — place the ante (≥ $3, ≤ bankroll). This is the only up-front bet.
2. **Deal** — 2 hole cards land face up; 3 community cards are placed **face down**.
3. **3rd Street** — on the 2 hole cards, **fold** or **raise 1× / 2× / 3×**. A raise
   reveals the **first** community card.
4. **4th Street** — with 3 cards visible, fold or raise 1× / 2× / 3×. A raise reveals
   the **second** community card.
5. **5th Street** — with 4 cards visible, fold or raise 1× / 2× / 3×. A raise reveals
   the **final** community card and settles the hand.
6. **Settlement** — the final five-card hand is scored against the pay table and paid
   on your **total amount wagered** (see below). `Enter` deals the next hand.

Folding at any street ends the hand immediately and forfeits **everything wagered so
far** (there is no partial refund). Each street's multiple is chosen independently —
1× on 3rd, 3× on 4th, 2× on 5th is perfectly legal.

## Controls

| Phase | Keys | Action |
|---|---|---|
| Ante | `←` / `→` | Lower / raise the ante by $3 (the table minimum) |
| Ante | `↑` / `↓` (or `PgUp`/`PgDn`) | Lower / raise the ante by $15 (coarse step) |
| Ante | `Enter` | Deal |
| 3rd / 4th / 5th Street | `1` / `2` / `3` | Raise 1× / 2× / 3× the ante (only if affordable) |
| 3rd / 4th / 5th Street | `f` | Fold (forfeit everything wagered so far) |
| 3rd / 4th / 5th Street | `←` / `→` then `Enter` | Move the highlight over the legal choices and confirm |
| Round over | `Enter` | Next hand |
| Game over | `←` / `→` then `Enter` | Select `Restart` (fresh $1000) or `Quit` |
| Anytime | `?` | Toggle the pay-table reference overlay |
| Anytime | `q` / `Ctrl+C` | Quit |

The street menu only offers the raise multiples your bankroll can currently cover
(plus Fold, always). A **running total** — the ante, each street bet placed, and the
**total at risk** — is shown throughout, so you always see how much is on the line.

## Hand rankings & pay table

Standard **five-card** poker rankings apply (a flush beats a straight, unlike the
three-card game). A win requires **a pair of Jacks or better**.

| Hand | Pays (to 1) |
|---|---|
| Royal Flush | 500 |
| Straight Flush | 100 |
| Four of a Kind | 40 |
| Full House | 10 |
| Flush | 6 |
| Straight | 4 |
| Three of a Kind | 3 |
| Two Pair | 2 |
| Pair of Jacks or better | 1 |
| Pair of 6s – 10s | **Push** (stake returned) |
| Pair of 5s or lower / anything else | **Loss** |

The **pair tiers** are the part new players miss: a pair of 6s through 10s **pushes**
(you get your total wager back, no profit), and a pair of 5s or lower **loses**. Only
a pair of Jacks or better actually pays.

- **Straights.** Both `A-2-3-4-5` (the wheel) and `10-J-Q-K-A` (broadway) are valid.
  There is no wrap-around: `Q-K-A-2-3` is **not** a straight.
- **Royal Flush** (`10-J-Q-K-A` suited) is paid separately from a lower straight
  flush; the suited wheel `A-2-3-4-5` is a straight flush, not a royal.

### The payout applies to your TOTAL WAGERED — not just the ante

This is the whole point of the game and the single most important rule. The pay-table
multiplier is applied to **ante + every street bet combined**, not to the ante alone.

> **Example.** Ante $25, then raise 3× / 3× / 3×:
> total wagered = 25 + 75 + 75 + 75 = **$250**.
> A Flush pays 6:1 → 6 × 250 = **$1,500 profit** (and your $250 stake stays), for a
> bankroll swing of **+$1,750**.

Bet sizing is therefore the entire game: a big final hand is worth far more when you
raised into it, and a fold late on a hopeless hand saves the whole accumulated stake.

## Money & affordability

- Starting bankroll **$1000**, table minimum **$3** (matching the other tables). All
  money is whole-dollar and settled atomically through the shared bankroll layer.
- Payouts are computed on the total wagered, per above. **Win:** stake back + profit.
  **Push:** stake back, no profit. **Loss / fold:** the total wager is forfeited.
- **Affordability policy — cap raises as you go.** You may set any ante ≥ the minimum;
  each street then offers only the raise multiples (1×, 2×, 3×) your **current**
  bankroll can cover. If you can't afford even 1× the ante at a street, only **Fold**
  is legal — the game never offers a partial or all-in raise (a raise is always
  exactly 1×, 2×, or 3× the ante). This is the more forgiving of the two policies the
  spec allows; it never blocks the ante itself.
- If your bankroll falls below the $3 minimum, the table ends with a restart option.

## Configurable pay table (regional variants)

The pay-table multipliers are exported package-level variables in
[`engine/payouts.go`](engine/payouts.go), so a regional variant is a one-line change.
For example, the Barona table pays a **Straight 5:1** instead of 4:1:

```go
engine.StraightPayout = 5
```

The `?` overlay reads these values live, so any override is reflected in-game
automatically.

## A gentle nudge

Because a made **high pair guarantees at least a push**, the table shows a subtle,
non-nagging note when your visible cards already lock in a paying or pushing hand
(e.g. *"pair of Kings — already paying"*). There is no built-in optimal-strategy
advisor; the real game has a known per-street strategy, which could be a later
feature.

## Out of scope (v1)

- The optional **3-Card Bonus** side bet and other side wagers.
- Progressive jackpots.
- Multiplayer / shared tables.
- A built-in strategy advisor.

## Layout

```
internal/mississippistud/
  mississippistud.go          adapter → game.Game (seeds the RNG; registers in the lobby)
  engine/                     pure game logic — no TUI imports, fully unit-tested
    card.go                   Card, Rank, Suit, 52-card Deck (injected RNG)
    hand.go                   5-card evaluator (rankings, straights, royal, pair rank)
    payouts.go                pay-table constants + qualification tiers
    game.go                   ante → 3rd/4th/5th street state machine, total-wagered settlement
    *_test.go                 engine unit tests
  ui/                         Bubble Tea model/update/view; reuses the shared card renderer
```

The engine holds all state on the `Game` value with its own injected RNG (no global
state, no wall-clock), so every session runs an independent game. The UI computes no
game logic — it reads the engine's accessors and renders them.
