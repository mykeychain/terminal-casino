# I Luv Suits

A seven-card flush showdown against the dealer. Poker hands don't matter here —
**only suits do**. You and the dealer each get **seven cards**; whoever holds **more
cards of one suit** wins. The bigger your flush, the more you're allowed to bet on it.

Single player vs. the dealer, matching the other tables. Fresh **$1000** bankroll per
session, no persistence.

## Game flow

1. **Bet** — place the **Ante** (≥ $3, required) and, optionally, the **Flush Rush**
   and **Super Flush Rush** side bets (each $0 or ≥ $3).
2. **Deal** — seven cards each. Yours are face up, grouped by suit with your best
   flush first and **underlined in gold**; the dealer's are face down.
3. **Decide** — **Fold** (forfeit the Ante) or **Play**. The Play wager is a multiple
   of the Ante, capped by the length of your flush:

   | Your flush | Play wager |
   |---|---|
   | 6 or 7 cards | 1×, 2× or 3× the Ante |
   | 5 cards | 1× or 2× |
   | 4 cards or fewer | 1× |

4. **Showdown** — the dealer's hand flips. The dealer **qualifies with a three-card
   flush, Nine-high or better** (any four-card flush qualifies).
   - Dealer doesn't qualify → the **Ante pays 1:1** and the **Play is returned**.
   - Dealer qualifies → compare flushes. Win: Ante and Play pay **1:1**. Lose: both
     lose. Tie: both push.
5. **Side bets** are paid on your seven cards alone — regardless of the dealer, and
   **even if you fold**.

## Comparing hands

- A hand's flush is its cards in one suit; its **best** flush is the suit with the
  **most** cards.
- **More cards wins.** A five-card flush beats any four-card flush, however high.
- At equal length, compare the flush cards **highest first**, then the next, and so
  on. Identical ranks tie — suits never break a tie.
- A straight flush is just a flush here; its sequence only matters for Super Flush
  Rush.

## Side bets

**Flush Rush** — pays on the length of your best flush:

| Flush | Pays |
|---|---|
| 7 cards | 100:1 |
| 6 cards | 20:1 |
| 5 cards | 10:1 |
| 4 cards | 2:1 |

**Super Flush Rush** — pays on your longest **straight flush** (consecutive ranks in
one suit; the Ace plays high `Q-K-A` or low `A-2-3`, no wrap-around):

| Straight flush | Pays |
|---|---|
| 7 cards | 500:1 |
| 6 cards | 200:1 |
| 5 cards | 100:1 |
| 4 cards | 50:1 |
| 3 cards | 9:1 |

Both tables appear in casinos. Their house edges, **5.30%** (Flush Rush) and **6.35%**
(Super Flush Rush), come from exhaustive enumeration of all 133,784,560 seven-card
hands. The tables are exported variables in [`engine/payouts.go`](engine/payouts.go),
so a different casino's table is a one-line change, and the `?` overlay reads them
live.

## Strategy & house edge

A simple strategy gets close to the best play: **play the maximum with any flush of
four or more cards; play a three-card flush only if it is Jack-high or better; fold
everything else.** In a 3,000,000-hand simulation this gave a house edge of about
**2.75% of the Ante**. Playing every hand costs about 10%.

## Controls

| Phase | Keys | Action |
|---|---|---|
| Betting | `←` / `→` | Lower / raise the focused wager by $3 |
| Betting | `↑` / `↓` (or `PgUp`/`PgDn`) | Lower / raise it by $15 |
| Betting | `Tab` / `Shift`+`Tab` | Switch between Ante, Flush Rush, Super Flush Rush |
| Betting | `Enter` | Deal |
| Decision | `1` / `2` / `3` | Play 1× / 2× / 3× the Ante (when allowed and affordable) |
| Decision | `f` | Fold |
| Decision | `←` / `→` then `Enter` | Move the highlight over the legal choices and confirm |
| Round over | `Enter` | Next hand |
| Game over | `←` / `→` then `Enter` | `Restart` (fresh $1000) or `Quit` |
| Anytime | `?` | Toggle the paytable / rules overlay |
| In a game | `q` / `Esc` | Back to the lobby |
| Anytime | `Q` / `Ctrl+C` | Quit the casino |

## Money

- Starting bankroll **$1000**, table minimum **$3**. All money is whole dollars.
- The Ante and side bets are escrowed on the deal; the Play is escrowed when placed.
  The menu only offers Play multiples that your flush allows **and** your bankroll
  covers. If even 1× is unaffordable, only Fold is offered.
- Wagers carry over to the next hand, trimmed to fit the bankroll (side bets first).
  Below the $3 minimum the table ends with a restart option.

## Layout

```
internal/iluvsuits/
  iluvsuits.go     adapter → game.Game (seeds the RNG; registers in the lobby)
  engine/          pure game logic — no TUI imports, fully unit-tested
    card.go        Card, Rank, Suit, 52-card Deck (injected RNG)
    hand.go        best-flush evaluator, comparison, qualification, straight-flush runs
    payouts.go     side-bet pay tables, qualifying card, Play caps
    game.go        bet → deal → fold/play → settle state machine
  ui/              Bubble Tea model/update/view; reuses the shared card renderer
```
