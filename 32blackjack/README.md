# 32blackjack — 3:2 Blackjack (terminal)

A single-player, local, terminal Blackjack game with rendered cards, played with
classic **3:2** rules. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
and [Lip Gloss](https://github.com/charmbracelet/lipgloss). This is v1 of a larger
"terminal casino" project; the game engine is a pure, UI-agnostic Go package so a
later SSH phase can reuse it unchanged.

Local only — no networking, no persistence (bankroll resets to $1000 every launch).

## Run

Requires Go (1.24+). From this directory:

```
go run ./cmd/32blackjack
```

Run the engine's unit tests:

```
go test ./...
```

## Controls

Navigation is arrow-driven: each phase shows a horizontal menu (or, when betting,
the wager between `◀ ▶`), `←`/`→` move a gold highlight, and `Enter` confirms.

| Phase | Keys | Action |
|---|---|---|
| Betting | `←` / `→` (also `↑`/`↓`) | Lower / raise the bet by $1 (min $3, max = bankroll) |
| Betting | `Enter` | Deal |
| Insurance (dealer shows Ace) | `←` / `→` then `Enter` | Select `Yes` / `No` (a fixed half-bet) |
| Playing | `←` / `→` then `Enter` | Move the highlight over the legal actions (Hit / Stand / Double / Split) and confirm |
| Round over | `Enter` | Next hand |
| Game over | `←` / `→` then `Enter` | Select `Restart` (fresh $1000) or `Quit` |
| Anytime | `q` / `Ctrl+C` | Quit |

The player-turn menu only lists actions that are currently legal — it is driven
directly by the engine's legal-action set, so an option never appears for a move
the engine would reject.

## Rules implemented (exact)

| Rule | Value |
|---|---|
| Decks | 6-deck shoe (312 cards), reshuffled at the start of the next hand once a cut card at ~75% depth is reached |
| Dealer on soft 17 | **Hits** (H17) |
| Blackjack payout | **3:2** |
| Double down | Any first two cards; requires funds for the extra bet |
| Split | Equal **rank** only (K+K yes, K+Q no); requires funds for the extra bet |
| Re-split | Non-ace pairs up to a maximum of **4 hands**; double-after-split allowed on non-ace hands |
| Split aces | One card each, no hit / double / re-split; A+10 from a split is **21, not blackjack** |
| Surrender | Not offered |
| Insurance | Offered when the dealer's up-card is an Ace; fixed half-bet, pays 2:1 |
| Dealer peek | Dealer peeks for blackjack on a 10- or Ace-value up-card before the player acts |
| Player natural | Auto-resolved after the peek, paid 3:2, no turn taken; natural-vs-natural pushes |

### Money & betting

- Starting bankroll **$1000** (resets every launch — no persistence).
- **Minimum bet $3**, maximum is your current bankroll. Bets move in $1 increments.
- The main bet is escrowed from your bankroll when the hand is dealt; settlement
  returns your stake plus any winnings.
- Payouts: win 1:1, blackjack 3:2, push returns your bet, insurance 2:1.
- Split, double, and insurance each require enough bankroll to fund the extra bet —
  if you can't cover it, the action isn't offered.
- If your bankroll falls below the $3 minimum, the game ends with a restart option.

> Money is tracked in whole dollars, so a 3:2 payout on an odd bet is floored to
> the nearest dollar (a $3 blackjack pays $4). Even-dollar bets ($10, $100, …)
> pay exactly.

## Architecture

```
32blackjack/
  cmd/32blackjack/main.go     entry point; starts the Bubble Tea program
  internal/engine/            pure game logic — no TUI imports, fully unit-tested
    card.go                   Card, Rank, Suit, Deck, 6-deck Shoe (injected RNG)
    hand.go                   hand value (soft/hard, multi-ace), blackjack/bust
    game.go                   state machine, actions, peek/H17, settlement/payouts
    *_test.go                 engine unit tests
  internal/ui/                Bubble Tea model/update/view
    model.go                  the Model over the engine
    render_card.go            box-drawing card rendering
    styles.go                 Lip Gloss styles
```

The engine holds all mutable state on the `Game` value (no global/package state) and
owns its own injected random source, so many independent games can run in one process
— what a later multiplayer/SSH layer will need. The UI never computes hand values,
legality, or payouts; it asks the engine and renders the result.
