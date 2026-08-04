// Command casino-ssh serves the Terminal Casino TUI over SSH via Charm Wish.
// Each connection gets its own independent lobby + game (fresh $1000); no shared
// state, no accounts. Ops: persisted host key, configurable address, graceful
// shutdown, idle-session timeout, a max-concurrent-sessions cap, and structured
// connection logging.
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"

	"github.com/mykeychain/terminal-casino/internal/blackjack32"
	"github.com/mykeychain/terminal-casino/internal/casino"
	"github.com/mykeychain/terminal-casino/internal/game"
	"github.com/mykeychain/terminal-casino/internal/poker3card"
	"github.com/mykeychain/terminal-casino/internal/theme"
)

func main() {
	addr := flag.String("addr", envOr("CASINO_SSH_ADDR", ":23234"), "SSH listen address")
	hostKey := flag.String("host-key", ".ssh/casino_ed25519", "path to the SSH host key (generated if absent)")
	idleTimeout := flag.Duration("idle-timeout", envDuration("CASINO_SSH_IDLE_TIMEOUT", 15*time.Minute),
		"disconnect a session after this long with no input (0 disables)")
	maxSessions := flag.Int("max-sessions", envInt("CASINO_SSH_MAX_SESSIONS", 50),
		"max concurrent sessions; further connections get a 'full' message (0 = unlimited)")
	flag.Parse()

	// Pin the shared color profile — identical to the local binary, so local and
	// remote render the same (see theme.Profile).
	theme.Apply()

	lim := &sessionLimiter{max: int32(*maxSessions)}

	srv, err := wish.NewServer(
		wish.WithAddress(*addr),
		wish.WithHostKeyPath(*hostKey),
		wish.WithMiddleware(
			bm.Middleware(teaHandler), // innermost: runs the casino TUI for the session
			lim.middleware,            // cap concurrent sessions + structured connect/disconnect logs
			activeterm.Middleware(),   // outermost: require an interactive PTY (before we count it)
		),
	)
	if err != nil {
		log.Fatal("could not create server", "error", err)
	}
	// Reap abandoned sessions: close a connection idle (no input) this long.
	if *idleTimeout > 0 {
		srv.IdleTimeout = *idleTimeout
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("Terminal Casino SSH server listening",
		"addr", *addr, "max_sessions", *maxSessions, "idle_timeout", *idleTimeout)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("shutdown error", "error", err)
	}
}

// sessionLimiter caps the number of concurrent sessions and logs each session's
// connect/disconnect with the live active count and its duration.
type sessionLimiter struct {
	max    int32 // 0 = unlimited
	active atomic.Int32
}

func (sl *sessionLimiter) middleware(next ssh.Handler) ssh.Handler {
	return func(s ssh.Session) {
		n := sl.active.Add(1)
		defer sl.active.Add(-1)

		user, addr := s.User(), s.RemoteAddr().String()
		if sl.max > 0 && n > sl.max {
			log.Warn("session rejected (full)", "user", user, "addr", addr, "max", sl.max)
			wish.Println(s, "The casino is full right now — please try again shortly.")
			return
		}

		start := time.Now()
		log.Info("session connected", "user", user, "addr", addr, "active", n, "max", sl.max)
		next(s)
		log.Info("session disconnected", "user", user, "addr", addr,
			"duration", time.Since(start).Round(time.Second), "active", sl.active.Load()-1)
	}
}

// teaHandler builds a fresh casino App for each SSH session. The game registry is
// wired here, so adding a game later is a one-line change.
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	games := []game.Game{blackjack32.Game(), poker3card.Game()}
	return casino.NewApp(games), []tea.ProgramOption{tea.WithAltScreen()}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
