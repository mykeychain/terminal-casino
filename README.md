# Terminal Casino

A terminal casino with rendered cards, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
and [Lip Gloss](https://github.com/charmbracelet/lipgloss). Connect (locally or over SSH),
pick a table from the lobby, and play. Today's table is **3:2 Blackjack** — classic 3:2
rules with up to three hands, split, double, and insurance.

Each session is independent: a fresh $1000 bankroll, no accounts, no persistence
(bankroll resets when you leave). The game engine is a pure, UI-agnostic Go package;
the same engine backs both the local and the SSH front ends.

## Run

Requires Go (1.24+).

**Locally** (lobby → game on your own terminal):

```
go run ./cmd/casino
```

**As an SSH server** (serves the same lobby to anyone who connects):

```
go run ./cmd/casino-ssh            # listens on :23234 by default
# then, from another terminal:
ssh -p 23234 localhost
```

Server flags: `-addr` (listen address, or `CASINO_SSH_ADDR`; default `:23234`) and
`-host-key` (path to the SSH host key, generated on first run; default `.ssh/casino_ed25519`).
Stop the server with Ctrl+C — it shuts down gracefully.

Run the tests:

```
go test ./...
```

## Deploying

To host the SSH server so others can connect (including a free-tier GCP e2-micro
setup that gets you `ssh terminal-casino.<yourdomain>` on port 22), see
[`deploy/README.md`](deploy/README.md). It also ships a `systemd` unit
(`deploy/casino-ssh.service`) and a `Dockerfile`.

## Controls

Navigation is arrow-driven: each phase shows a horizontal menu (or, when betting,
the wager between `◀ ▶`), `←`/`→` move a gold highlight, and `Enter` confirms.

| Phase | Keys | Action |
|---|---|---|
| Betting | `←` / `→` (also `↑`/`↓`) | Lower / raise the focused hand's bet by $1 (min $3) |
| Betting | `Shift`+`←` / `→` (or `PgDn`/`PgUp`) | Lower / raise the bet by $25 (coarse step) |
| Betting | `a` | Open another hand (up to 3; each with its own bet) |
| Betting | `x` / `Backspace` | Close the focused hand (min 1) |
| Betting | `Tab` / `Shift`+`Tab` | Switch the focused hand |
| Betting | `Enter` | Deal all hands |
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
- You may **open up to 3 hands** in a round (`a` to add, `x` to remove, `Tab` to switch),
  each with its own bet; the total staked must fit your bankroll. Insurance is offered
  per hand, and each hand can still split independently.
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
terminal-casino/
  cmd/
    casino/main.go            local entry point (lobby → game on stdout)
    casino-ssh/main.go        SSH server (Charm Wish) serving the same lobby per connection
  internal/
    game/                     the Game interface the lobby dispatches to
    theme/                    shared Lip Gloss palette
    casino/                   the lobby / game-selector Bubble Tea app
    blackjack32/              the "3:2 Blackjack" variant (adapter → game.Game)
      engine/                 pure game logic — no TUI imports, fully unit-tested
        card.go               Card, Rank, Suit, Deck, 6-deck Shoe (injected RNG)
        hand.go               hand value (soft/hard, multi-ace), blackjack/bust
        game.go               state machine, actions, peek/H17, settlement/payouts
        *_test.go             engine unit tests
      ui/                     Bubble Tea model/update/view + card rendering
```

The engine holds all mutable state on the `Game` value (no global/package state) and
owns its own injected random source, so every connection runs its own independent game
in one process — which is what the SSH server relies on. The UI never computes hand
values, legality, or payouts; it asks the engine and renders the result. Adding a new
game means implementing `game.Game` and registering it in the `cmd/*` wiring — the
lobby and SSH server need no changes.
