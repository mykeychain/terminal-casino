// Command casino-ssh serves the Terminal Casino TUI over SSH via Charm Wish.
// Each connection gets its own independent lobby + game (fresh $1000); no shared
// state, no accounts. Part 1 ops: persisted host key, configurable address,
// graceful shutdown.
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"github.com/mykeychain/terminal-casino/internal/blackjack32"
	"github.com/mykeychain/terminal-casino/internal/casino"
	"github.com/mykeychain/terminal-casino/internal/game"
	"github.com/mykeychain/terminal-casino/internal/theme"
)

func main() {
	addr := flag.String("addr", envOr("CASINO_SSH_ADDR", ":23234"), "SSH listen address")
	hostKey := flag.String("host-key", ".ssh/casino_ed25519", "path to the SSH host key (generated if absent)")
	flag.Parse()

	// Pin the shared color profile — identical to the local binary, so local and
	// remote render the same (see theme.Profile).
	theme.Apply()

	srv, err := wish.NewServer(
		wish.WithAddress(*addr),
		wish.WithHostKeyPath(*hostKey),
		wish.WithMiddleware(
			bm.Middleware(teaHandler), // runs the casino TUI for the session
			activeterm.Middleware(),   // require an interactive PTY
			logging.Middleware(),      // connect/disconnect log line
		),
	)
	if err != nil {
		log.Fatal("could not create server", "error", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("Terminal Casino SSH server listening", "addr", *addr)
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

// teaHandler builds a fresh casino App for each SSH session. The game registry is
// wired here, so adding a game later is a one-line change.
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	games := []game.Game{blackjack32.Game()}
	return casino.NewApp(games), []tea.ProgramOption{tea.WithAltScreen()}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
