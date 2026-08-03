# Feel tier — animation & result juice (v2.1)

Presentational polish for the blackjack UI: animate the deal, give the dealer's play
suspense, and add a result "juice" moment. **UI-only** — the engine resolves the round
instantly as today; the UI reveals the already-computed state progressively. No engine
changes.

## Locked decisions
1. **Dealer draws are UI-revealed**, not a live engine step. When a player action settles
   the round, the engine has already played the dealer and set outcomes; the UI animates
   revealing that final dealer hand. Zero engine risk.
2. **Player actions stay instant.** Hit/Double reveal their card immediately (responsive).
   Only the **deal**, the **dealer play**, and the **result** animate.
3. **Input is locked while an animation runs** (except `q`/`ctrl+c`). Keys pressed during
   an animation are ignored (not queued).
4. **Timing constants** (named, tunable) — starting values:
   - `dealBeat = 150ms` (per card during the initial deal)
   - `dealerBeat = 350ms` (per card as the dealer draws — suspense)
   - `flipPause = 300ms` (hole-card reveal beat before the dealer draws)
   - `resultHold = 600ms` (banner appears; brief hold before controls unlock)
   - `bankrollTick = 450ms` total for the bankroll count animation

## Effects

### 1. Deal reveal
After `Deal()`, reveal cards one at a time in deal order — each spot's first card, the
dealer up-card, each spot's second card, then the dealer hole card (as a face-down back) —
`dealBeat` apart. The action bar / prompts are hidden until the deal finishes. Then:
- dealer up-card is Ace and insurance affordable → show the insurance prompt;
- else route to the player turn, OR, if the engine already resolved (dealer blackjack /
  player natural), go straight to the **dealer reveal + result**.

### 2. Dealer reveal + suspense
When the round settles from a player action (engine phase → `PhaseRoundOver` with the
dealer having played): flip the hole card (`flipPause`), then reveal each dealer draw card
(the cards beyond the initial two) `dealerBeat` apart. If the dealer didn't draw (all
player hands busted, or dealer stood pat), just flip the hole card. Then → result.

### 3. Result juice
- A brief **centered headline banner** for the round's outcome — e.g. `BLACKJACK! +$75`,
  `WIN +$50`, `PUSH`, `BUST`, `DEALER WINS` (for multi-hand, a net-oriented headline like
  `YOU WIN +$25` / `YOU LOSE -$25` / `PUSH`), styled with the existing win/lose/push colors.
- The **bankroll number ticks** from its pre-settlement value to the new value over
  `bankrollTick`. (The Model snapshots bankroll before any settling action and animates the
  displayed value toward `game.Bankroll()`.)
- After `resultHold`, unlock the `Next hand` control.

## Implementation (internal/blackjack32/ui)
- Add an animation sub-state to the `Model` (e.g. `animState`: idle / dealReveal /
  dealerReveal / result), independent of the engine `Phase`, with reveal counters and a
  `displayBankroll` that ticks toward `game.Bankroll()`.
- Drive it with `tea.Tick` producing typed messages; gate input while `animState != idle`.
- Thread reveal state into the render functions (dealer/hand rendering shows only revealed
  cards; hole stays a back until its flip; action bar/prompt hidden until relevant).
- Snapshot the pre-settlement bankroll before actions that can settle (Deal, Hit, Stand,
  Double, Split, Insurance) so the tick has a start value.
- Keep it contained: no engine imports change; the engine remains the source of truth for
  the final state, the UI only controls *when* each piece becomes visible.

## Testing
- Existing engine + casino tests stay green.
- Unit-test what's deterministic: input is ignored while `animState != idle`; feeding the
  typed tick messages advances reveal counters and converges `displayBankroll` to
  `game.Bankroll()`; the sequence reaches idle with the full final state shown.
- Visual verification (orchestrator): capture the frame sequence step-by-step.

## Out of scope
Sound; per-glyph card-flip animation; animating player hits; any live engine dealer stepping.
