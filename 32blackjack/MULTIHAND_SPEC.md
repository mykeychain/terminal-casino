# 32blackjack — Multi-hand play (v1.2)

An additive feature on top of the v1.1 game: let one player play **up to 3 hands**
in a single round against one dealer, each hand with its own bet, reached by
*opening extra hands at the betting screen*. Nothing about the core rules, payouts,
dealer behavior, RNG, or the single-hand experience changes.

This spec is authoritative for v1.2. Build engine-first with tests, then the UI.

---

## 0. Locked decisions

1. **Cap: up to 3 opened hands** (`MaxSpots = 3`). The per-hand split cap stays 4
   (`MaxHands = 4`, now enforced **per opened hand/spot**).
2. **Each opened hand has its own bet**, each ≥ `MinBet` ($3), in `$1` increments.
   The **sum of all bets must be ≤ bankroll**, and the **total is escrowed at deal**.
3. **Insurance is per hand.** When the dealer shows an Ace, step through each hand
   that can afford `spotBet/2`, offering an independent `Yes/No`.
4. **Naturals resolve per hand** at peek (each natural auto-paid 3:2, no turn); the
   remaining hands play out.
5. **No "box" chrome.** During play, opened hands render as ordinary hand blocks
   left-to-right (the same look splits already have), labeled `Hand N`.
6. Everything stays **UI-agnostic**, no global state, deterministic stacked-shoe
   path preserved (locked decision 5 of v1.1 still holds).

Terminology: an **opened hand / spot** is a betting position the player sets up
before the deal. During play a spot may split into more hands (up to `MaxHands`
per spot). The UI calls them all "hands"; the engine may use "spot" internally for
the initial positions.

---

## 1. Engine changes

### Constants
- `MaxSpots = 3` — maximum opened hands (new).
- `MaxHands = 4` — per-spot split cap (unchanged value; now scoped per spot).

### Betting model (replaces the single pending `bet`)
The game holds a list of pending per-spot bets during `PhaseBetting`, length 1..`MaxSpots`,
initialized to **one spot** at `MinBet` (or the previous round's first bet, clamped).
- `NumSpots() int`
- `SpotBet(i int) int`
- `SetSpotBet(i, amount int) error` — validates `amount >= MinBet`, multiple of
  `BetIncrement`, and that the **new total across all spots ≤ bankroll**.
- `AddSpot() error` — appends a spot at `MinBet` if `NumSpots() < MaxSpots` and the
  new total ≤ bankroll; else `ErrIllegalAction` / `ErrInvalidBet`.
- `RemoveSpot(i int) error` — removes spot `i`; illegal if `NumSpots() == 1`.
- **Back-compat:** keep `Bet()` = `SpotBet(0)` and `SetBet(amount)` = `SetSpotBet(0, amount)`
  so existing single-hand call sites and tests keep working.

### Deal
- Escrow **sum of pending bets**.
- Create one `playerHand` per spot, each carrying its spot's bet and a `spot int` tag.
- **Deal order:** one card to each spot (L→R), dealer up-card, second card to each spot
  (L→R), dealer hole card, then subsequent draws in order. (Deterministic; document it
  for stacked-shoe tests.)
- Mark each spot's initial hand as a natural where applicable (per hand, not a single bool).

### Insurance (per hand)
- Entered only when the dealer up-card is an Ace **and at least one spot can afford**
  `spotBet/2` against current (post-escrow) bankroll.
- Track the spot currently being offered: `InsuranceSpot() int` (index into hands/spots).
  The offered amount is `SpotBet(InsuranceSpot())/2`.
- `Insurance(take bool)` applies to the current insurance spot (posting `spotBet/2` if
  `take` and affordable), then advances to the **next affordable spot**; when none remain,
  proceed to the dealer peek. Track per-spot insurance taken/amount/net.
- Affordability is sequential: each taken insurance lowers bankroll for the next offer.

### Peek & naturals
- Peek unchanged (Ace or ten up-card).
- **Dealer has blackjack:** resolve all hands at once — a hand holding a natural pushes,
  every other hand loses. Each taken insurance pays 2:1.
- **No dealer blackjack:** each spot's initial hand that is a natural is **auto-paid 3:2**,
  marked resolved (outcome set, hand done); remaining hands go to the player turn. Any
  insurance taken is lost.

### Player turn
- Walk hands left-to-right exactly as today (auto-advance on 21/bust/split-ace).
- Legal actions per active hand unchanged. **Split cap is per spot:** Split is legal only
  while the count of hands sharing the active hand's `spot` is `< MaxHands` (and funds/rank
  rules as before).

### Dealer turn & settlement
- Dealer plays iff **some hand is still unresolved and not bust** (outcome `Pending` &&
  not bust). If every remaining hand is bust or already resolved (naturals), dealer skips.
- Settlement iterates hands with outcome `Pending` and settles each vs the dealer as today.
  Hands already resolved (naturals paid at peek) are **skipped** — do not re-settle them.
- No engine "round net" needed; the UI sums per-hand `Net`.

### View accessors
- `Player() []HandView` — `HandView` gains `Spot int` (the originating spot index).
  The UI labels hands sequentially (`Hand N`) by position; `Spot` is available for logic.
- Betting: `NumSpots()`, `SpotBet(i)`, plus `Bet()` (spot 0) for back-compat.
- Insurance: `InsuranceSpot()` and the derived amount `SpotBet(InsuranceSpot())/2`.

---

## 2. UI changes

### Betting
- **One hand (default):** a single gold bet tile with `◀ $ ▶` — same feel as today.
  Hints: `← → bet ±$1 · shift ±$25 · a add hand · enter deal · q quit`.
- **2+ hands:** a **row of bet tiles** labeled `Hand 1..N`; the **focused tile is
  gold-highlighted and shows `◀ $ ▶`**, others sit flat. A `Total staked $X · after
  deal $Y` line appears.
- **Controls:** `Tab` / `Shift+Tab` switch focused hand · `←/→` adjust focused bet
  (`Shift` = ±$25) · **`a`** add a hand · **`x`** (or `Backspace`) remove focused hand ·
  `Enter` deal all · `q` quit.
- Contextual hints: `x remove` only with 2+ hands; `a add` hidden at the 3-hand cap.
- UI tracks the focused spot index; clamps on add/remove.

### Insurance
- One `Yes/No` panel per offered hand, titled with the hand: e.g.
  `╭─ Insurance · Hand 2 ─╮  Dealer shows an Ace — insure Hand 2 for $12?  Yes  No`.
  Driven by `InsuranceSpot()`.

### Play & settlement
- Opened hands render as hand blocks left-to-right (existing style), labeled `Hand N`,
  active one highlighted; the action panel names the active hand.
- Settlement shows per-hand outcomes (existing footers) plus a **round-net line** summed
  from per-hand `Net`.
- **Overflow:** past a width threshold, use compact cards and, if still too wide, stack
  hand blocks vertically so up to 3 hands (with splits) stay readable.

---

## 3. Testing additions (engine)
- Multi-spot deal deals the right cards in order and escrows the summed total.
- Mixed per-hand settlement in one round (win / lose / push across spots).
- Per-hand natural: one spot's natural paid 3:2 while other spots still play.
- Spot + split interaction; per-spot 4-hand cap (splitting one spot doesn't affect another).
- Per-hand insurance: taken on some spots, dealer BJ pays those 2:1; dealer no-BJ loses them.
- Affordability across spots: `AddSpot`/`SetSpotBet` rejected when total would exceed bankroll.
- `RemoveSpot` rejected at one spot; add/remove index handling.
- All-hands-bust → dealer skips; some bust / some stand → dealer plays.
- Back-compat: existing single-spot tests still pass via `SetBet`/`Bet`.

---

## 4. Out of scope (unchanged from v1.1)
Core rules and payouts (3:2, insurance 2:1, dealer H17, peek), $3 min / $1 step,
RNG/shoe, split mechanics, and the entire single-hand experience.
