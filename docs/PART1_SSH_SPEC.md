# Part 1 — Casino SSH server + repo reorg (v2.0)

Serve the existing game over SSH via Charm Wish, behind a game-selector lobby, after
reorganizing the repo into a single root module. **The engine and blackjack UI code do
not change** — they move and are re-wired. Authoritative for Part 1.

## Locked decisions
1. **Session model:** independent per-connection games. Each SSH connection (and each local
   run) gets its own `casino` App; each game launch seeds its own `engine.NewGame(seed)`.
   No shared state. (Zero engine changes.)
2. **Access:** open & anonymous. No auth; accept any interactive connection. Fresh $1000 per
   game. (Identity/handles deferred to the 4B follow-up.)
3. **Entry:** a real **lobby / game-selector** (not a throwaway welcome). Lists games (one
   today: "3:2 Blackjack"), arrow-select + Enter to launch. Both local and SSH land here.
4. **Ops (this pass = 4A):** persisted host key (generated if absent), configurable listen
   address, graceful shutdown. (Idle timeout, session cap, logging/metrics = deferred 4B.)
5. **Color:** fixed server-wide profile — **TrueColor** — pinned once at startup
   (`lipgloss.SetColorProfile`). Package-global styles stay as-is; no per-session renderer.
6. **Naming:** flat, lowercase, no-separator variant packages so each directory doubles as
   its adapter package. This variant = `blackjack32`. Future: `blackjackfreebet`,
   `poker3card`, `pokerholdem`, … Never nest a digit-leading folder.

## Target repo structure (single root module)
```
terminal-casino/
  go.mod                      # module github.com/mykeychain/terminal-casino  (moved to root)
  README.md
  docs/                       # SPEC.md, MULTIHAND_SPEC.md, PART1_SSH_SPEC.md, PLAN.md
  cmd/
    casino/main.go            # LOCAL: run the casino TUI on stdout (lobby → game)
    casino-ssh/main.go        # SSH server: serves the same casino TUI per connection
  internal/
    game/                     # package game — the Game interface (neutral, breaks cycles)
    theme/                    # package theme — shared palette hex constants
    casino/                   # package casino — lobby / selector Bubble Tea app
    blackjack32/              # package blackjack32 — adapter: Game() game.Game, Title "3:2 Blackjack"
      engine/                 # package engine  (moved from 32blackjack/internal/engine, UNCHANGED)
      ui/                     # package ui       (moved from 32blackjack/internal/ui, UNCHANGED)
```

## Contracts
- `internal/game`: `type Game interface { Title() string; Description() string; New(width, height int) tea.Model }`.
  (Neutral package imported by both `casino` and `blackjack32`; no concrete game imports here.)
- `internal/blackjack32`: `func Game() game.Game` — returns an adapter whose `New(w,h)` builds
  the existing blackjack `ui.Model` over a fresh `engine.NewGame(time.Now().UnixNano())`,
  seeded with the last known window size.
- `internal/casino`: `func NewApp(games []game.Game) tea.Model` — the lobby App. States
  **Lobby ↔ Playing**. Lobby renders title + selectable game list (↑/↓ or ←/→, Enter to
  launch). While Playing it embeds the selected game's model and delegates Update/View.
  Navigation: `Esc` returns to the lobby (intercepted at the App level, blackjack model
  untouched); `q`/`Ctrl+C` quits the session. Forwards `WindowSizeMsg` to the active model.
- `cmd/casino` / `cmd/casino-ssh`: pin the color profile, build `[]game.Game{ blackjack32.Game() }`,
  and run `casino.NewApp(...)`.

## SSH server (4A)
`wish.NewServer` with middlewares: `bubbletea` (per-session `casino` App), `activeterm`
(require a PTY; reject non-interactive), and a minimal connect/disconnect log line.
- Host key: `wish.WithHostKeyPath(path)`, path configurable (flag `-host-key`, default under
  a local data dir, e.g. `./.ssh/casino_ed25519`), generated if absent.
- Listen: flag `-addr` / env `CASINO_SSH_ADDR`, default `:23234`.
- Shutdown: `signal.NotifyContext` (SIGINT/SIGTERM) → `server.Shutdown(ctx)` with a timeout.

## Dependencies
`github.com/charmbracelet/wish` (+ `wish/bubbletea`, `wish/activeterm`, `wish/logging`),
pulling `charmbracelet/ssh` and `golang.org/x/crypto`. `go mod tidy`.

## Testing & verification
- Existing engine + blackjack-UI tests pass unchanged after the move (import paths updated).
- New unit tests: `casino` App transitions (select → launch → Esc back → quit); registry.
- SSH round-trip (orchestrator review gate): start the server on a localhost port, connect a
  scripted SSH client through a PTY, capture the lobby + a dealt blackjack hand as frames.
- README: run local (`go run ./cmd/casino`), run server (`go run ./cmd/casino-ssh`), connect
  (`ssh -p 23234 localhost`), host-key path, `-addr`.

## Phases (gated)
1. **Reorg** — root module, relocate packages to the target structure, update imports, docs → `docs/`.
   Gate: `go build/vet/test` green, zero behavior change; `cmd/casino` still plays blackjack directly (temporary).
2. **`game` + `theme` + `blackjack32` adapter.** Gate: builds; blackjack reachable via `game.Game`.
3. **`casino` lobby app** + unit tests; `cmd/casino` now lands in the lobby. Gate: transitions tested.
4. **SSH server** (`cmd/casino-ssh`, 4A ops). Gate: builds; SSH round-trip frames captured.
5. **README + polish.** Gate: full green + SSH round-trip verified.

## Deferred (4B follow-up)
Idle-session timeout · max-concurrent-sessions cap ("tables full") · structured connection
logging/metrics · player identity/handles (toward leaderboards). Separate plan when ready.
