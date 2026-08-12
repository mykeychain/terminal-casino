# Free Bet Blackjack

A blackjack variant where **the house pays for your doubles and splits** — in
exchange, **the dealer pushes on 22** instead of busting. You get the aggressive
upside of doubling and splitting every promising hand without risking the extra
chips; the dealer's Push 22 rule is what pays for it.

Built on the same 6-deck, H17, 3:2 engine as the [3:2 Blackjack](../blackjack32)
table, with the same up-to-three-hands betting, insurance, and dealer peek. Fresh
**$1000** bankroll per session, no persistence.

## The "free bet"

When you make a qualifying double or split, no additional chips leave your
bankroll — the house posts a **free bet** alongside your hand. If the hand
**wins**, the free bet pays exactly like a real bet. If it **loses**, the free bet
simply vanishes: you risked nothing on it. If it **pushes**, nothing changes
hands. In the table below a free bet shows in green as `$N free`; your real,
at-risk money stays gold.

## The three rules that define the game

### 1. Free double — on a two-card hard 9, 10, or 11

Doubling a hard 9, 10, or 11 is **free**: the house matches your wager, you draw
exactly one card, and a win pays as though you had doubled for real. The action
bar names it **`Free Double`** (tinted green).

You may still double **any other** two-card total (a hard 8, a soft hand, …), but
that one costs **your own money** — a real extra bet is escrowed, so it's offered
only when your bankroll can cover it.

### 2. Free split — on any pair except tens

Splitting **any pair is free** — 2s through 9s, and aces — named
**`Free Split`**. Each split hand rides a free bet; free **re-splitting** is
allowed up to four hands per spot. **Split aces** take exactly one card each and
cannot be hit, doubled, or re-split. A free-split hand can then **free-double** if
it makes a hard 9/10/11.

**Ten-value pairs** (e.g. K+K) are the exception: splitting them costs **your own
money**, so it's offered only when funded. (As in the base table, a split needs an
equal **rank** — K+Q is not a pair.)

> Only one hand per spot ever carries your real bet — the original. Every hand
> added by a free split rides a free bet, so a busted free-split hand costs you
> **nothing**.

### 3. Dealer pushes on 22

This is the trade-off that pays for all the free action. If the dealer's final
total is exactly **22**, it is **not a bust** — every non-busted player hand
**pushes** (your real stake is returned, free bets void). The dealer area shows
**`22 · push`** and the result banner reads **`PUSH · DEALER 22`**.

The important exceptions:

- A **player blackjack still wins** (paid 3:2) against a dealer 22 — naturals are
  paid at the peek, before the dealer ever draws to 22.
- A player **21 or any other non-busted total pushes**.
- A hand that **already busted still loses** — busting is always resolved first.
- A dealer **23 or more is an ordinary bust**; only exactly 22 pushes.

## Base rules (shared with 3:2 Blackjack)

| Rule | Value |
|---|---|
| Decks | 6-deck shoe, reshuffled past a ~75% cut card |
| Dealer on soft 17 | **Hits** (H17) |
| Blackjack payout | **3:2** |
| Insurance | Offered on a dealer Ace; fixed half-bet, pays 2:1 |
| Dealer peek | Peeks for blackjack on a 10- or Ace-value up-card |
| Hands per round | Up to **3** betting spots, each split independently |
| Starting bankroll | **$1000**, table minimum **$3**, whole-dollar money |

## Controls

Identical to the 3:2 table — arrow-driven menus, `Enter` to confirm. The
player-turn menu only lists currently-legal actions, driven by the engine's
legal-action set, and names a free double or split **`Free Double`** /
**`Free Split`** — tinted **green**, the same "on the house" accent as the free
bet on your hand. An own-money double or split keeps the plain **`Double`** /
**`Split`** in white, so free vs. paid reads at a glance.

| Phase | Keys | Action |
|---|---|---|
| Betting | `←` / `→` (also `↑`/`↓` coarse) | Lower / raise the focused hand's bet |
| Betting | `a` / `x` | Add a hand (up to 3) / remove the focused hand |
| Betting | `Tab` | Switch the focused hand · `Enter` deals |
| Insurance | `←` / `→` then `Enter` | Take (fixed half-bet) or decline |
| Playing | `←` / `→` then `Enter` | Move over the legal actions and confirm |
| Round over | `Enter` | Next hand |
| Game over | `←` / `→` then `Enter` | Restart (fresh $1000) or Quit |
| In a game | `q` / `Esc` | Leave the table, back to the lobby |
| Anytime | `Q` / `Ctrl+C` | Quit the casino |

## How a free bet settles

Every hand tracks two amounts: **`bet`** (real money, escrowed at the deal) and
**`free`** (the house-funded free bet, never escrowed).

- **Win** — pays 1:1 on `bet + free` and returns your `bet` stake.
- **Loss** — you forfeit only `bet`; the free bet costs nothing.
- **Push** (including dealer 22) — your `bet` stake is returned; the free bet is void.
- **Blackjack** — a natural pays 3:2 on your bet (naturals are never doubled or split).

> **Example.** Bet $10, free-split a pair of 4s, and each 4 draws a 7 to make a
> hard 11 you free-double into 21. No extra real money is ever escrowed, yet two
> winning 21s pay **+$40** — the free-bet dream. Lose them instead and you're out
> only your original $10.

## Out of scope (v1)

- The optional **Push 22** side bet (which pays when the dealer busts with 22).
- Progressive jackpots and other side wagers.

## Layout

```
internal/blackjackfreebet/
  blackjackfreebet.go         adapter → game.Game (seeds the RNG; registers in the lobby)
  engine/                     pure game logic — no TUI imports, fully unit-tested
    card.go                   Card, Rank, Suit, 6-deck Shoe (injected RNG)
    hand.go                   hand value (soft/hard, multi-ace), blackjack/bust
    game.go                   state machine + free-bet settlement, Push 22, peek/H17
    *_test.go                 engine unit tests (free/own doubles & splits, Push 22)
  ui/                         Bubble Tea model/update/view; reuses the shared card renderer
```

Forked from `blackjack32`: the engine adds a per-hand free-bet component, free
vs own-money double/split legality, and the Push 22 settlement rule. The engine
holds all state on the `Game` value with its own injected RNG (no global state, no
wall-clock), so every session runs an independent game. The UI computes no game
logic — it reads the engine's accessors (including `DoubleIsFree`, `SplitIsFree`,
and `DealerPush22`) and renders them.
