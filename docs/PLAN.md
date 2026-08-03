# 32blackjack — Build Plan & Progress Tracker

Source of truth for scope: `SPEC.md` (v1.1, with locked decisions 1–5).
Module path: `github.com/mykeychain/terminal-casino/32blackjack`
Branch: `claude/32blackjack-spec-review-slp749`

Roles: I (orchestrator) act as liaison/manager/reviewer. Subagents do the building.
Gate discipline: no phase starts until the prior phase's gate passes review.

Status legend: ⬜ not started · 🔨 in progress · 🔎 in review · ✅ done · ⛔ blocked

---

# Part 1 — Casino SSH server + repo reorg (v2.0). Spec: `docs/PART1_SSH_SPEC.md`.
New module path: `github.com/mykeychain/terminal-casino`.

## P1.1 — Repo reorg (root module, relocate packages, docs → docs/)  ✅
Owner: subagent.
- go.mod → root, module `github.com/mykeychain/terminal-casino`; `32blackjack/internal/engine`
  → `internal/blackjack32/engine`; `internal/ui` → `internal/blackjack32/ui`;
  `cmd/32blackjack` → `cmd/casino` (still direct-to-blackjack for now); `*.md` → `docs/`.
- Update all import paths. Engine/UI code UNCHANGED.
**Gate:** `go build/vet/test` green, zero behavior change.

## P1.2 — game interface + theme + blackjack32 adapter  ✅
**Gate:** builds; blackjack reachable via game.Game.

## P1.3 — casino lobby app (+ tests); cmd/casino lands in lobby  ✅
Reviewed: app.go state machine clean (global q/ctrl+c quit, Esc→lobby intercept, size +
async msgs forwarded, games unaware of lobby); casino tests pass; engine untouched.
**Gate:** transitions tested.

## P1.4 — SSH server (cmd/casino-ssh, 4A ops)  ⬜
**Gate:** builds; SSH round-trip frames captured.

## P1.5 — README + polish  ⬜
**Gate:** full green + SSH round-trip verified.

---

## Phase 1 — Engine core (card / hand / shoe) + deterministic construction  ✅
Owner: subagent
- go.mod + directory scaffold, compiles.
- `card.go`: Card, Rank, Suit, Deck, 6-deck Shoe, injected RNG (locked #5), cut card constant.
- `hand.go`: value calc (soft/hard, multi-ace), blackjack detection, bust.
- Deterministic construction path: fixed seed + pre-stacked shoe for tests.
**Gate:** `go build ./...` clean; RNG per-instance, no global rand / time / crypto in engine.

## Phase 2 — Game state machine + legal actions + payouts  ✅
Owner: subagent (continues Phase 1)
- `game.go`: Phase enum, Deal/Hit/Stand/Double/Split/Insurance, dealer peek, H17 dealer play.
- Split logic isolated (MaxHands=4 constant), split-ace one-and-done, DAS.
- Legal-action set gated by bankroll affordability (locked #3).
- Bet escrow at deal (locked #4), natural auto-resolve (locked #2), fixed half insurance (locked #1).
- Settlement + payout math (3:2, 1:1, push, insurance 2:1, double), game-over < $25.
**Gate:** engine API stable and reviewed; `go build ./...` clean; `go vet` clean.

## Phase 3 — Engine unit tests  ✅
Owner: subagent
- Full §7 test matrix (value calc, H17, legality+affordability, re-split cap, ace rules,
  10+A=21, natural auto-resolve + natural push, payouts, insurance-lost, mixed settlement,
  game-over, cut-card reshuffle).
**Gate:** `go test ./internal/engine/...` passes; meaningful coverage of the §7 list.

## Phase 4 — TUI (model / update / view / card render / styles)  ✅
Owner: subagent (starts only after engine API frozen in Phase 3)
- Bubble Tea model over the engine; renders state, sends actions; no game logic in UI.
- Card rendering (2-char rank field), face-down back, suit colors, dealer/player/status/action bars.
- Controls per §6; insurance prompt; game-over restart; WindowSizeMsg resize.
**Gate:** `go build ./...` clean; `go run ./cmd/32blackjack` launches; manual smoke of a hand.

## Phase 5 — README + final polish  ✅
Owner: orchestrator or subagent
- README: run, controls, exact rules implemented.
- Final `go build ./... && go vet ./... && go test ./...` green. Tidy go.mod.
**Gate:** all deliverables (§8) satisfied.

## Phase 6 — v1.1 feedback: money tweak + palette + arrow controls  ✅
Owner: subagent
- Engine: `MinBet` 25→3, bet increment 5→1; fix affected engine tests; logic otherwise unchanged.
- UI palette (approved): gold=focus, green=money-good, red=loss/red-suits, slate card back.
- Red cards: same soft-white border as all suits; only rank + suit glyph red.
- Controls: replace letter keys with arrow-navigation + Enter (menu driven off LegalActions()).
- Update README + SPEC money/controls to match.
**Gate:** `go build/vet/test` green; engine still UI-agnostic; new frames reviewed.

## Phase 7 — Multi-hand engine (per-spot bets, deal N, per-hand naturals/insurance)  ✅
Owner: subagent. Spec: `MULTIHAND_SPEC.md` §0–1, §3.
- MaxSpots=3; per-spot bets + Add/Remove/SetSpotBet; Deal N hands; per-hand naturals;
  per-spot split cap; per-hand insurance loop; settlement skips resolved naturals.
- Back-compat Bet()/SetBet on spot 0; all existing engine tests still pass + new §3 tests.
**Gate:** build/vet/test green; engine UI-agnostic; API reviewed & frozen for UI.

## Phase 8 — Multi-hand UI (bet-tile row, tab switch, add/remove, per-hand insurance)  ✅
Owner: subagent (after Phase 7 frozen). Spec: `MULTIHAND_SPEC.md` §2.
- Betting bet-tile row (1 hand = single tile), Tab/Shift+Tab switch, ←/→ adjust,
  a add / x(Backspace) remove; per-hand insurance panel; round-net at settlement; overflow.
**Gate:** build/vet green; frames reviewed by user.

---

**Phase 6 — APPROVED.** Reviewed by orchestrator: build/vet/test green; engine `game.go` diff
is ONLY `MinBet` 25→3 and `BetIncrement` 5→1 (logic untouched), all 28 tests still pass with
updated thresholds. UI verified by reading source: palette centralized as named color vars per
approved roles; red cards use soft-white border with only rank+suit red (`cardRedStyle`); arrow
menu driven off `LegalActions()`, dispatched to engine methods with `CanAct` re-check, cursor
clamped/reset by menu signature — no game logic in UI. Fresh frames sent for user review.

## Review log

**Phase 7 (multi-hand engine) — APPROVED.** Reviewed by orchestrator by reading game.go:
28 existing + 10 new tests pass; only game.go changed + multihand_test.go added; no UI
imports, no global rand/time/crypto. Verified: single-hand path preserved (Bet()/SetBet →
spot 0, deal order reduces to classic for 1 spot); per-hand naturals resolve at peek without
revealing hole (payNatural), settlement skips outcome!=Pending so no double-settle;
per-hand insurance via advanceInsurance with sequential affordability; per-spot split cap
(spotHandCount); betting validates sum-of-bets ≤ bankroll. **API FROZEN for Phase 8 UI:**
NumSpots/SpotBet/SetSpotBet/AddSpot/RemoveSpot, InsuranceSpot, HandView.Spot + existing.

**Phases 1–3 (engine) — APPROVED.** Reviewed by orchestrator (not just trusting subagent report):
build/vet/test run independently → green; `go test` 28 test funcs, 81% stmt coverage.
Verified by reading source: per-instance PCG RNG (no global rand / time / crypto), zero UI
imports, equal-**rank** split (face cards have distinct Rank constants), H17 dealer play,
peek routing, affordability-gated legal actions, split-ace one-and-done, 4-hand cap,
natural auto-resolve + natural-vs-natural push, escrow at deal, settlement math.
Engine API is **FROZEN** for the TUI.

**Phase 4 (TUI) — APPROVED.** Reviewed by orchestrator: build/vet/test green; engine files
unchanged (`git diff` empty); no game logic in UI — Update only routes keys to engine methods,
re-checks `CanAct` before each call, action bar driven off `LegalActions()`, all displayed
numbers from engine accessors. Card renderer uses fixed 2-char rank field; face-down hatch
back; suit colors; active-hand highlight; compact fallback for ≤4 split hands; WindowSizeMsg
handled; restart builds a fresh `NewGame` (time seed only in main).
- Open flag (non-blocking): integer-dollar money floors 3:2 on odd-$25 bets ($25 BJ → $37,
  insurance on $25 → $12). Documented by subagent; tests use $100 to assert exact ratios.
  Decision pending with user; does not block UI (UI only displays engine values).
