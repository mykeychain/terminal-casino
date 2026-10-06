# DJ Wild

Five-card stud against the dealer with **five wild cards**: the four **deuces** and
the **Joker** (a 53-card deck). Wild cards turn mediocre hands into monsters, so the
hand ladder goes past a Royal Flush to **Five of a Kind** and **Five Wilds**.

Single player vs. the dealer, matching the other tables. Fresh **$1000** bankroll per
session, no persistence.

## Game flow

1. **Bet** — place the **Ante**; an equal **Blind** goes up with it automatically.
   Optionally add the **Trips** side bet ($0 or ≥ $3).
2. **Deal** — five cards each from a freshly shuffled 53-card deck. Yours are face
   up, with every wild card labeled `wild` beneath it; the dealer's are face down.
3. **Decide** — **Fold** (forfeit the Ante and Blind) or **Play** for **2× the Ante**.
4. **Showdown** — the dealer flips. **The dealer always plays** — there is no
   qualifying hand.
   - **You win:** Ante and Play pay **1:1**; the Blind pays the table below.
   - **Dealer wins:** Ante, Blind and Play all lose.
   - **Tie:** everything pushes.
5. **Trips** is paid on your five cards alone, regardless of the dealer and **even if
   you fold**.

## Hand rankings

High to low: **Five Wilds** · **Royal Flush** · **Five of a Kind** · Straight Flush ·
Four of a Kind · Full House · Flush · Straight · Three of a Kind · Two Pair · Pair ·
High Card.

- Each wild card plays as whatever rank and suit makes the hand strongest. A wild in
  an open-ended straight plays at the top; a wild in a flush plays as the Ace.
- **Natural and wild hands of the same rank are equal.** A wild nine-high straight
  flush ties a natural one.
- A hand is **natural** when it makes its rank without a wild card. A deuce counts as
  a plain Two if the hand doesn't need it wild (`2♠ 3♠ 4♠ 5♠ 6♠` is a natural straight
  flush). The Joker is always wild. Being natural only matters for Trips.

## Pay tables

| Hand | Blind (when you win) | Trips — natural | Trips — wild |
|---|---|---|---|
| Five Wilds | 1000:1 | — | 2000:1 |
| Royal Flush | 50:1 | 1000:1 | 100:1 |
| Five of a Kind | 10:1 | — | 100:1 |
| Straight Flush | 9:1 | 200:1 | 20:1 |
| Four of a Kind | 4:1 | 60:1 | 8:1 |
| Full House | 3:1 | 30:1 | 6:1 |
| Flush | 2:1 | 25:1 | 4:1 |
| Straight | 1:1 | 15:1 | 3:1 |
| Three of a Kind or less | push | 5:1 (trips) / lose | 1:1 (trips) / lose |

The Blind table is the standard one. Trips tables differ a lot between casinos, so
this one was tuned by **exhaustively enumerating all 2,869,685 five-card hands**
(`TestTripsReturn` in `engine/hand_test.go`): it returns **95.76%** (house edge
≈ 4.24%). All three tables are exported maps in
[`engine/payouts.go`](engine/payouts.go), and the `?` overlay reads them live.

## Strategy & house edge

**Play any pair of Fours or better; fold everything else.** In a 3,000,000-hand
simulation this gave a house edge of **1.76% of the Ante + Blind**, close to the
published 1.73% for the game. Playing every hand costs about 17%.

## Controls

| Phase | Keys | Action |
|---|---|---|
| Betting | `←` / `→` | Lower / raise the focused wager by $3 |
| Betting | `↑` / `↓` (or `PgUp`/`PgDn`) | Lower / raise it by $15 |
| Betting | `Tab` | Switch between Ante (= Blind) and Trips |
| Betting | `Enter` | Deal |
| Decision | `p` / `f` | Play 2× / Fold |
| Decision | `←` / `→` then `Enter` | Move the highlight and confirm |
| Round over | `Enter` | Next hand |
| Game over | `←` / `→` then `Enter` | `Restart` (fresh $1000) or `Quit` |
| Anytime | `?` | Toggle the paytable / rules overlay |
| In a game | `q` / `Esc` | Back to the lobby |
| Anytime | `Q` / `Ctrl+C` | Quit the casino |

## Money

- Starting bankroll **$1000**, table minimum **$3** Ante (so at least $6 with the
  Blind). All money is whole dollars.
- Ante, Blind and Trips are escrowed on the deal; the Play is escrowed when placed.
  If you can't cover 2× the Ante after the deal, only Fold is offered.
- Wagers carry over to the next hand, trimmed to fit the bankroll (Trips first).
  When the bankroll can't cover a $3 Ante plus Blind, the table ends with a restart
  option.

## Layout

```
internal/djwild/
  djwild.go        adapter → game.Game (seeds the RNG; registers in the lobby)
  engine/          pure game logic — no TUI imports, fully unit-tested
    card.go        Card (incl. the Joker), Rank, Suit, 53-card Deck (injected RNG)
    hand.go        wild-card evaluator (best assignment of every wild), natural check
    payouts.go     Blind and Trips (natural / wild) pay tables
    game.go        bet → deal → fold/play → settle state machine
  ui/              Bubble Tea model/update/view; reuses the shared card renderer
```
