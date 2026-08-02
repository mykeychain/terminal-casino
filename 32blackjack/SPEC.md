# 32blackjack — Build Specification (v1.1)

A single-player, local, terminal-based Blackjack game with rendered cards, played with classic **3:2 blackjack** rules (hence the name — it will appear as "3:2 Blackjack" in the casino's game selector). This is v1 of a larger "terminal casino" project; it must be built so game logic is cleanly separable from the UI, because a later phase will serve the same engine over SSH (Charm Wish). **This phase is local only — no SSH, no networking, no persistence.**

**Project/module directory name:** `32blackjack`

---

## 0. Locked decisions (v1.1 review resolutions)

These override anything below that conflicts with them. They are the result of a spec review and are non-negotiable for v1:

1. **Insurance is a fixed half-bet, yes/no.** Insurance is exactly `mainBet / 2`, offered as a yes/no prompt. Engine method stays `Insurance(bool)`.
2. **Player natural blackjack auto-resolves.** A player dealt a natural 21 takes **no** turn. After the dealer peek confirms no dealer blackjack, the player natural is paid **3:2** immediately. Natural-vs-natural is a push.
3. **Split / Double / Insurance require full funds.** Each posts an additional bet (Split and Double = another `mainBet`; Insurance = `mainBet/2`). If current bankroll cannot cover it, the action is **illegal** and must not appear in the engine's legal-action set. No "double for less," no "split for less," no free bets.
4. **Bet is escrowed at deal.** The main bet is removed from bankroll at the moment of the deal. "Current bankroll" for the purpose of affordability checks (rule 3) and the status bar is the post-escrow balance. Settlement pays winnings/returns back into bankroll.
5. **RNG is per-game and injected; the shoe is riggable for tests.** The engine must own its own random source, supplied at construction (e.g. `NewGame(seed int64)` or `NewGame(src rand.Source)`), and must **not** call the global `math/rand` functions or read `time.Now()` / `crypto/rand` internally. Production supplies a time-based seed; tests supply a fixed seed **or a pre-stacked shoe**. This is an architecture requirement (§2), not just a test detail.

---

## 1. Tech Stack

- **Language:** Go (latest stable; toolchain here is go1.24.x).
- **TUI framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm architecture: model / update / view).
- **Styling:** [Lip Gloss](https://github.com/charmbracelet/lipgloss) for colors, borders, layout.
- **Components (optional):** [Bubbles](https://github.com/charmbracelet/bubbles) if useful (e.g. help bar), not required.
- No database, no config files, no network calls.

---

## 2. Architecture (important)

Keep a hard separation between **game engine** and **UI**. The engine must be a pure, UI-agnostic Go package with no Bubble Tea imports, fully unit-testable on its own. The TUI renders engine state and sends player actions into it.

Suggested layout:

```
32blackjack/
    go.mod                      # module path: github.com/mykeychain/terminal-casino/32blackjack
    SPEC.md
    README.md
    cmd/32blackjack/main.go     # entry point; starts Bubble Tea program
    internal/engine/            # pure game logic, no TUI imports
        card.go                 # Card, Rank, Suit, Deck, Shoe
        hand.go                 # Hand, value calc (soft/hard), blackjack detection
        game.go                 # Game state machine, actions, payouts
        game_test.go            # unit tests for engine
    internal/ui/                # Bubble Tea model/update/view
        model.go
        render_card.go          # ASCII/box-drawing card rendering
        styles.go               # Lip Gloss styles
```

The engine exposes state + methods like `Deal()`, `Hit()`, `Stand()`, `Double()`, `Split()`, `Insurance(bool)`, and reports whose turn it is and what actions are currently legal. The UI never computes hand values or legality itself — it asks the engine.

**RNG / construction (locked decision 5).** The engine owns its randomness. A constructor takes a seed or `rand.Source`, and the engine holds its own `*rand.Rand` (or uses `math/rand/v2` with an owned instance). No global `rand.Seed`/`rand.Intn`, no `time.Now()` or `crypto/rand` reads inside the engine. Provide a test-only path to construct a `Game` with a **pre-stacked shoe** (e.g. an unexported field set by a same-package test helper, or a `NewGameWithShoe(...)` used from tests) so specific deals can be asserted deterministically.

**No global state (§9).** All mutable state lives on the `Game` value so many engine instances can run in one process later (multiplayer). This is why the RNG must be per-instance.

**Legal-action set.** The engine exposes the current `Phase` and a set of currently-legal `Action`s. This set MUST already account for bankroll affordability (locked decision 3): Split/Double appear only when the player can fund the extra bet; Insurance appears only when the player can fund `mainBet/2`.

---

## 3. Rules (exact)

These are fixed for v1. Implement precisely.

| Rule | Value |
|---|---|
| Decks | 6-deck shoe |
| Dealer on soft 17 | **Hits** (H17) |
| Blackjack payout | **3:2** |
| Insurance | Offered when dealer upcard is an Ace; fixed half-bet; pays 2:1 |
| Double down | Allowed on any first two cards (requires full funds) |
| Split | Allowed (see below; requires full funds) |
| Surrender | **Not** offered |
| Dealer peek | Dealer peeks for blackjack on 10/Ace upcard before player acts |

### Split rules (isolate in engine so caps are easy to tune)

- Split allowed on any two cards of equal **rank** (equal rank only — e.g. K+K splits, K+Q does not).
- **Re-splitting allowed** for non-ace pairs: if a split hand draws another matching-rank card, it can be split again, up to a **maximum of 4 hands total**. Keep the cap a single named constant (`MaxHands = 4`).
- **Split aces are one-and-done:** each split ace receives exactly one card and cannot hit, double, or re-split. (You cannot re-split aces even if the drawn card is another ace.)
- **Double after split (DAS): allowed** on non-ace split hands (requires full funds).
- Each hand is played out independently, left to right; newly created split hands are inserted and played in order.
- A 10 + Ace resulting from a split is **21, not a blackjack** (counts as a normal 21, pays 1:1, loses to / pushes a dealer blackjack per normal 21 rules).
- Every split posts an additional bet equal to the original main bet; if the player cannot fund it, Split is illegal (locked decision 3).

### Shoe / shuffle

- 6 decks = 312 cards, freshly shuffled at game start.
- Place a **cut card** at a fixed penetration depth ~75%; expose it as a single named constant (e.g. `CutCardPosition = 234`). When the cut card is reached mid-hand, reshuffle a fresh 6-deck shoe at the **start of the next hand** (not mid-hand).
- Shuffle uses the engine's injected RNG (locked decision 5). No global RNG, no internal time/crypto seeding.

### Hand value

- Number cards = face value; J/Q/K = 10; Ace = 11 or 1.
- "Soft" hand = an Ace counted as 11 without busting. Track and display soft/hard correctly (e.g. "soft 17").
- Bust > 21.

---

## 4. Money & Betting

- Starting bankroll: **$1000**.
- **No persistence** — resets to $1000 every launch.
- Player places a bet **each hand** before the deal.
- **Minimum bet: $3** (v1.1 tweak — lowered from $25). Maximum bet: capped at current bankroll (no separate table max in v1).
- **Bet is escrowed at deal** (locked decision 4): the main bet leaves bankroll when the hand is dealt; settlement returns stake + winnings.
- **Bet increment: $1** (v1.1 tweak — was $5). The wager adjusts up/down in whole-dollar steps, clamped to `[MinBet, bankroll]`.
- Payouts:
  - Win: 1:1 (even money on bet).
  - Blackjack: 3:2.
  - Push: bet returned.
  - **Insurance: fixed side bet of exactly half the main bet** (locked decision 1); pays 2:1 if dealer has blackjack; requires funds beyond the escrowed main bet (locked decision 3).
  - Double: bet doubled, exactly one card drawn; requires funds for the additional bet (locked decision 3).
- **Player natural blackjack** (locked decision 2): auto-resolved after peek, paid 3:2, no player turn; natural-vs-natural pushes.
- If bankroll falls **below the $3 minimum**, show a game-over state with a restart option.

---

## 5. Game Flow (state machine)

1. **Betting** — player sets bet (≥ $3, ≤ bankroll). Confirms to deal.
2. **Deal** — main bet escrowed from bankroll. Two cards to player (face up), two to dealer (one up, one down). If the player has a natural, mark it for auto-resolution at step 4.
3. **Insurance check** — if dealer upcard is Ace, offer insurance (yes/no, fixed `mainBet/2`, only if affordable) before dealer peek resolution.
4. **Dealer peek** — if upcard is Ace or 10-value, dealer checks hole card for blackjack.
   - If dealer has blackjack: hand ends immediately. Resolve insurance (pays 2:1). A player natural pushes; any non-natural player hand loses.
   - If dealer does **not** have blackjack: insurance (if taken) loses. If the player has a **natural**, pay it 3:2 now and skip the player turn (locked decision 2). Otherwise proceed to step 5.
5. **Player turn(s)** — legal actions offered contextually (each gated by affordability per locked decision 3):
   - Always: **Hit**, **Stand**.
   - **Double**: only on first two cards (and after split on non-ace hands); requires full funds.
   - **Split**: on any valid equal-rank pair, including re-splitting, up to the 4-hand cap (aces excepted — one split, one card each, no re-split); requires full funds.
   - Handle all hands left-to-right, including hands newly created by a re-split.
   - Player bust ends that hand immediately.
6. **Dealer turn** — reveal hole card; dealer hits until hard 17+ or soft 18+ (i.e. **hits soft 17**), then stands. Skip if all player hands busted.
7. **Settlement** — compare each player hand vs dealer, adjust bankroll, show results per hand.
8. **Next hand** — return to Betting. Reshuffle if cut card was passed.

The engine exposes the current phase and the set of currently-legal actions so the UI can show/hide controls correctly.

---

## 6. TUI / Visual Requirements

This is a **full TUI with rendered cards**, not a text log. Aesthetic reference: clean rounded-border cards like a polished terminal solitaire.

### Cards

- Render each card as a multi-line box using box-drawing characters, with **rank in top-left and bottom-right**, and a **suit symbol** (♠ ♥ ♦ ♣). Reserve a **fixed 2-character field for rank** so "10" does not shift the box borders.
- **Color:** hearts/diamonds red, spades/clubs white/default. Use Lip Gloss.
- **Face-down card** (dealer hole card): render a distinct patterned back (e.g. a hatch/▚ fill).
- Cards may sit side by side — fine for v1; keep it readable.

### Layout

- **Dealer area** (top): dealer's hand, hole card hidden until dealer's turn. Show dealer total (or just upcard value while hidden).
- **Player area** (bottom): player's hand(s). When split, show all hands and clearly indicate which is active. Provide a graceful fallback (compact cards or shrink) when up to 4 split hands would overflow the terminal width.
- **Status bar:** bankroll, current bet, current hand value (with soft/hard), and result messages ("Blackjack!", "Bust", "Push", "You win $X", "Dealer wins").
- **Action bar:** contextual key hints for legal actions only.

### Controls (keyboard)

- Betting: adjust bet up/down (`+`/`-` by chip denomination or arrows), `Enter` to deal.
- Playing: `h` hit, `s` stand, `d` double, `p` split, `y`/`n` for insurance prompt.
- `r` restart when game over, `q` quit anytime.

### Feel

- Redraw cleanly each frame (Bubble Tea handles this).
- Optional: short pause/animation as dealer draws. Not required for v1.
- Handle terminal resize gracefully (Bubble Tea `WindowSizeMsg`).

---

## 7. Testing

Unit-test the **engine** package thoroughly (no TUI), using the deterministic construction path (fixed seed or pre-stacked shoe):

- Hand value calc incl. soft/hard and multi-ace, blackjack detection.
- Dealer H17 behavior (hits soft 17, stands hard 17 and soft 18).
- Split/double legality, including **affordability gating** (Split/Double/Insurance illegal when underfunded — locked decision 3).
- Re-split up to the 4-hand cap; aces-split-one-card-only and no re-split of aces.
- 10+A from a split scored as 21, not blackjack (loses to / pushes dealer natural).
- Player natural auto-resolution: paid 3:2 with no turn; natural-vs-natural push (locked decision 2).
- Payout math: blackjack 3:2, even-money win, push return, double, insurance 2:1.
- **Insurance taken and dealer does NOT have blackjack** → insurance lost, main hand continues.
- Multi-hand mixed settlement in one round (one wins, one busts, one pushes).
- Bankroll → game-over transition (below $3 min).
- Cut-card reshuffle trigger.

Provide a way to run the engine deterministically in tests (injectable seed/source or a way to stack the shoe).

---

## 8. Deliverables

- Compilable Go module (`go.mod`), runs with `go run ./cmd/32blackjack`.
- `README.md`: how to run, the controls, and the exact rules implemented.
- Engine package with passing unit tests (`go test ./...`).
- Clean separation so a later SSH/Wish layer can reuse `/internal/engine` unchanged.

---

## 9. Explicitly Out of Scope (v1)

- No SSH / networking / multiplayer.
- No persistence (bankroll resets each run).
- No chat, leaderboards, or accounts.
- No card counting hints / basic-strategy advisor.
- No sound.

Keep these out, but **don't architect in a way that blocks them** — especially keep the engine UI-agnostic and free of global state, since multiplayer will later run many engine instances in one process.
