// Package ui is the Bubble Tea front end for the Three-Card Poker table. It
// renders the engine's state and forwards player intent as engine actions; it
// computes no game logic. The presentation mirrors the 3:2 Blackjack table — an
// anchored top/middle/bottom frame, an arrow-navigated action bar, and a feel
// tier that cascades the deal in and flips the dealer's hand at showdown.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/poker3card/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// compactWidthThreshold: below this terminal width, cards are packed edge-to-edge
// so the player's three and the dealer's three still fit side by side.
const compactWidthThreshold = 80

// coarseBetStep is the larger bet increment used by Up/Down (PgUp/PgDn) for
// moving a wager quickly; plain Left/Right steps by MinBet. It is a multiple of
// MinBet so wagers always stay on the MinBet grid.
const coarseBetStep = 5 * engine.MinBet

// Bet-tile indices for the two independent wagers set during betting.
const (
	spotAnte     = 0
	spotPairPlus = 1
	numSpots     = 2
)

// Model is the Bubble Tea model wrapping a frozen engine.Game. Navigation is
// arrow-driven: the currently-legal choices are shown as a horizontal menu and
// the cursor selects one, which is dispatched to the matching engine method.
type Model struct {
	game    *engine.Game
	newGame func() *engine.Game // produces a fresh game for restart (seeded by main)

	width  int
	height int

	// cursor indexes the current phase's horizontal menu; it is clamped every
	// frame and reset to 0 when the menu's signature changes.
	cursor  int
	menuSig string

	// focusedSpot is the wager tile being edited during PhaseBetting: spotAnte or
	// spotPairPlus.
	focusedSpot int

	// msg is a transient status line (e.g. a rejected bet).
	msg string

	// showPaytable toggles the full-screen paytable reference overlay (`?`). It is
	// independent of the game Phase and can be pulled up at any time.
	showPaytable bool

	// ---- Feel-tier animation sub-state (independent of the engine Phase) ----
	animState animState

	// dealShown counts revealed positions during the deal cascade; dealerShown
	// counts dealer cards flipped face up during the showdown reveal.
	dealShown   int
	dealerShown int

	// displayBankroll is the bankroll shown in the header. It is frozen at the
	// pre-deal value for the duration of a round (masking the escrow) and ticks to
	// the settled bankroll during the result effect, so its delta equals the
	// round's net. bankFrom / bankStep drive that count-up.
	displayBankroll int
	bankFrom        int
	bankStep        int
}

// New builds a model over an already-constructed game. newGame produces a fresh,
// freshly-seeded game when the player restarts after game over (keeping the seed
// in the caller preserves the engine's injected-RNG contract). It returns a
// tea.Model; construct it over a NewGameWithDeck game for deterministic frames.
func New(g *engine.Game, newGame func() *engine.Game) tea.Model {
	return newModel(g, newGame)
}

// newModel is the concrete-typed constructor used internally and by tests.
func newModel(g *engine.Game, newGame func() *engine.Game) Model {
	return Model{game: g, newGame: newGame, displayBankroll: g.Bankroll()}
}

// Init implements tea.Model. No initial command is needed.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	if mm, cmd, handled := m.updateAnim(msg); handled {
		return mm, cmd
	}
	return m, nil
}

func (m Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()

	// Quit and return-to-lobby keys (q, Q, Esc, Ctrl+C) are handled by the lobby
	// before they reach this model, so they are not handled here.

	// Paytable overlay: `?` toggles it from any phase (even mid-animation). While
	// it is up it acts as a modal — enter/space (or `?`) dismisses it and every
	// other key is swallowed so nothing acts behind it.
	if key == "?" {
		m.showPaytable = !m.showPaytable
		return m, nil
	}
	if m.showPaytable {
		if key == "enter" || key == " " {
			m.showPaytable = false
		}
		return m, nil
	}

	// Input is locked while an animation runs (except quit). During the result
	// the Next-hand control is shown, so the confirm key finishes the count-up
	// and advances — the UI is always actionable at resolution.
	if m.animState != animIdle {
		if m.animState == animResult && (key == "enter" || key == " ") {
			m.animState = animIdle
			m.displayBankroll = m.game.Bankroll()
			return m.handleRoundOver(key)
		}
		return m, nil
	}

	switch m.game.Phase() {
	case engine.PhaseBetting:
		return m.handleBetting(key)
	case engine.PhaseDecision:
		return m.handleDecision(key)
	case engine.PhaseRoundOver:
		return m.handleRoundOver(key)
	case engine.PhaseGameOver:
		return m.handleGameOver(key)
	}
	return m, nil
}

// syncCursor resets the cursor to 0 when the menu signature changed since the
// last frame, then clamps it into [0, n).
func (m Model) syncCursor(sig string, n int) Model {
	if sig != m.menuSig {
		m.menuSig = sig
		m.cursor = 0
	}
	m.cursor = tui.ClampIdx(m.cursor, n)
	return m
}

// ---- Betting ----

func (m Model) handleBetting(key string) (tea.Model, tea.Cmd) {
	m.focusedSpot = tui.ClampIdx(m.focusedSpot, numSpots)
	switch key {
	case "right":
		m.adjustBet(engine.MinBet)
	case "left":
		m.adjustBet(-engine.MinBet)
	case "up", "pgup":
		m.adjustBet(coarseBetStep)
	case "down", "pgdown":
		m.adjustBet(-coarseBetStep)
	case "tab":
		m.focusedSpot = (m.focusedSpot + 1) % numSpots
	case "shift+tab":
		m.focusedSpot = (m.focusedSpot + numSpots - 1) % numSpots
	case "enter", " ":
		// Snapshot the bankroll before Deal escrows the stake; the deal reveal
		// holds this value in the header so it does not visibly drop, and it
		// becomes the start of the result count-up.
		pre := m.game.Bankroll()
		if err := m.game.Deal(); err != nil {
			m.msg = dealError(err)
			return m, nil
		}
		m.msg = ""
		m.cursor = 0
		return m.startDealReveal(pre)
	}
	return m, nil
}

// dealError translates a rejected Deal into a player-facing message.
func dealError(err error) string {
	if err == engine.ErrInvalidBet {
		return fmt.Sprintf("Place an Ante or Pair Plus wager (min $%d) to deal.", engine.MinBet)
	}
	return "Cannot deal: " + err.Error()
}

// adjustBet moves the focused wager by delta, snapping to the MinBet grid and
// capping so Ante + Pair Plus never exceeds the bankroll. A wager can be cleared
// to $0 (no wager) or raised to at least MinBet; it is then pushed through the
// engine's SetAnte / SetPairPlus.
func (m *Model) adjustBet(delta int) {
	cur := m.spotBet(m.focusedSpot)
	other := m.spotBet(1 - m.focusedSpot)

	target := cur + delta
	if target < 0 {
		target = 0
	}
	if maxForSpot := m.game.Bankroll() - other; target > maxForSpot {
		target = maxForSpot
	}
	// Snap to a MinBet multiple; anything in (0, MinBet) collapses to 0 (cleared).
	target = (target / engine.MinBet) * engine.MinBet

	var err error
	if m.focusedSpot == spotAnte {
		err = m.game.SetAnte(target)
	} else {
		err = m.game.SetPairPlus(target)
	}
	if err != nil {
		m.msg = "bet: " + err.Error()
		return
	}
	m.msg = ""
}

// spotBet returns the pending wager for a bet-tile index.
func (m Model) spotBet(spot int) int {
	if spot == spotAnte {
		return m.game.Ante()
	}
	return m.game.PairPlus()
}

// ---- Decision ----

func (m Model) handleDecision(key string) (tea.Model, tea.Cmd) {
	actions := m.decisionActions()
	m = m.syncCursor("decision|"+decisionSig(actions), len(actions))
	switch key {
	case "left", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "right", "down":
		if m.cursor < len(actions)-1 {
			m.cursor++
		}
	case "p":
		if m.game.CanPlay() {
			return m.dispatchDecision(actionPlay)
		}
	case "f":
		return m.dispatchDecision(actionFold)
	case "enter", " ":
		if len(actions) == 0 {
			return m, nil
		}
		return m.dispatchDecision(actions[m.cursor])
	}
	return m, nil
}

// decisionAction is the small set of decision-phase intents (kept local to the
// UI; the engine exposes Play / Fold directly).
type decisionAction int

const (
	actionPlay decisionAction = iota
	actionFold
)

func (m Model) dispatchDecision(a decisionAction) (tea.Model, tea.Cmd) {
	var err error
	switch a {
	case actionPlay:
		err = m.game.Play()
	case actionFold:
		err = m.game.Fold()
	}
	if err != nil {
		m.msg = decisionLabel(a) + ": " + err.Error()
		return m, nil
	}
	m.msg = ""
	return m.afterDecision()
}

// decisionActions lists the legal decision-phase choices in menu order. Play is
// offered only when the bankroll can cover it; Fold is always available.
func (m Model) decisionActions() []decisionAction {
	var out []decisionAction
	if m.game.CanPlay() {
		out = append(out, actionPlay)
	}
	out = append(out, actionFold)
	return out
}

func decisionSig(actions []decisionAction) string { return fmt.Sprintf("%v", actions) }

func decisionLabel(a decisionAction) string {
	switch a {
	case actionPlay:
		return "Play"
	default:
		return "Fold"
	}
}

// ---- Round over / game over ----

func (m Model) handleRoundOver(key string) (tea.Model, tea.Cmd) {
	m = m.syncCursor("round-over", 1)
	if key == "enter" || key == " " {
		if err := m.game.NextHand(); err != nil {
			m.msg = err.Error()
		} else {
			m.msg = ""
			m.focusedSpot = spotAnte
			m.displayBankroll = m.game.Bankroll()
		}
	}
	return m, nil
}

func (m Model) handleGameOver(key string) (tea.Model, tea.Cmd) {
	m = m.syncCursor("game-over", 2)
	switch key {
	case "left", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "right", "down":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor == 0 {
			m.game = m.newGame()
			m.menuSig = ""
			m.cursor = 0
			m.focusedSpot = spotAnte
			m.msg = ""
			m.animState = animIdle
			m.displayBankroll = m.game.Bankroll()
		} else {
			return m, tea.Quit
		}
	}
	return m, nil
}

// ---- View ----

// compact reports whether cards should be packed tightly to fit the terminal.
func (m Model) compact() bool {
	return m.width > 0 && m.width < compactWidthThreshold
}

// View implements tea.Model. It composes the screen as three anchored zones — a
// fixed top (title + header), a fixed bottom (banner + control panel), and a
// flexible middle (the table) — sized to exactly m.height × m.width. Before the
// first WindowSizeMsg it falls back to simple top-to-bottom stacking.
func (m Model) View() string {
	f := m.frame()
	if m.showPaytable {
		return m.viewPaytable(f)
	}
	if m.width == 0 || m.height == 0 {
		return m.viewStacked(f)
	}

	top := m.renderTop(f)
	bottom := m.renderBottom(f)
	topH := lipgloss.Height(top)
	bottomH := lipgloss.Height(bottom)
	middleH := m.height - topH - bottomH
	if middleH < 0 {
		middleH = 0
	}

	middle := m.renderTable(f, false)
	if lipgloss.Height(middle) > middleH {
		middle = m.renderTable(f, true)
	}
	middleRegion := tui.FitHeight(middle, middleH)

	body := lipgloss.JoinVertical(lipgloss.Left, top, middleRegion, bottom)
	body = tui.FitHeight(body, m.height)
	return lipgloss.NewStyle().Width(m.width).Render(body)
}

// viewStacked is the pre-size fallback used only before the first WindowSizeMsg.
func (m Model) viewStacked(f revealFrame) string {
	var b strings.Builder
	b.WriteString(m.renderTop(f))
	b.WriteString("\n")
	b.WriteString(m.renderTable(f, false))
	bottom := m.renderBottom(f)
	if bottom != "" {
		b.WriteString("\n")
		b.WriteString(bottom)
	}
	return b.String()
}

// renderTop builds the fixed top zone: the title pinned to row 0 and the header
// beneath it.
func (m Model) renderTop(f revealFrame) string {
	return titleStyle.Render("Three-Card Poker") + "\n" + m.renderHeader(f)
}

// renderHeader shows the persistent standing: bankroll, the current wagers, and
// the player's hand name once the deal has fully landed.
func (m Model) renderHeader(f revealFrame) string {
	parts := []string{"Bankroll " + moneyStyle.Render(fmt.Sprintf("$%d", m.displayBankroll))}

	wagers := []string{}
	if a := m.game.Ante(); a > 0 {
		wagers = append(wagers, betStyle.Render(fmt.Sprintf("Ante $%d", a)))
	}
	if p := m.game.PairPlus(); p > 0 {
		wagers = append(wagers, betStyle.Render(fmt.Sprintf("Pair+ $%d", p)))
	}
	if pb := m.game.PlayBet(); pb > 0 {
		wagers = append(wagers, betStyle.Render(fmt.Sprintf("Play $%d", pb)))
	}
	if len(wagers) > 0 {
		parts = append(parts, strings.Join(wagers, "  "))
	}

	// Show the hand name once every player card is on the table (not mid-deal).
	if f.playerCards >= handSize {
		if name := m.game.Player().Name; name != "" {
			parts = append(parts, dimStyle.Render("You have ")+nameStyle.Render(name))
		}
	}
	return strings.Join(parts, "    ")
}

// renderTable builds the flexible middle zone — the dealer area over the player
// area — at the requested vertical density.
func (m Model) renderTable(f revealFrame, short bool) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderDealerArea(f, short),
		m.renderPlayerArea(f, short),
	)
}

func (m Model) renderDealerArea(f revealFrame, short bool) string {
	label := areaLabelStyle.Render("Dealer")
	dv := m.game.Dealer()

	var status string
	switch {
	case f.dealerCards == 0:
		status = dimStyle.Render("waiting for bets")
	case f.dealerFaceUp < handSize:
		status = dimStyle.Render("hand hidden")
	default:
		status = nameStyle.Render(dv.Name)
		if dv.Qualified {
			status += "  " + dimStyle.Render("(qualifies)")
		} else {
			status += "  " + dimStyle.Render("(doesn't qualify)")
		}
	}

	if f.dealerCards == 0 {
		return label + "  " + status
	}
	cards := lipgloss.NewStyle().Height(tui.CardHeightFor(short)).Render(tui.RenderSlots(faces(dv.Cards), f.dealerFaceUp, f.dealerCards, m.compact(), short))
	return label + "  " + status + "\n" + cards
}

func (m Model) renderPlayerArea(f revealFrame, short bool) string {
	label := areaLabelStyle.Render("You")
	if f.playerCards == 0 {
		return label + "  " + dimStyle.Render("place your bets")
	}

	pv := m.game.Player()
	var status string
	if f.playerCards >= handSize {
		status = nameStyle.Render(pv.Name)
	} else {
		status = dimStyle.Render("dealing…")
	}
	cards := lipgloss.NewStyle().Height(tui.CardHeightFor(short)).Render(tui.RenderHand(faces(pv.Cards), f.playerCards, m.compact(), short))
	return label + "  " + status + "\n" + cards
}

// renderBottom builds the fixed bottom zone: the result banner + settlement
// breakdown (when shown), the transient status line, and the phase's control
// panel + hint. It is empty mid-animation (before the result), when neither the
// banner nor the controls are shown.
func (m Model) renderBottom(f revealFrame) string {
	var parts []string
	if f.showBanner {
		parts = append(parts, m.renderBanner())
		parts = append(parts, m.renderSettlement())
	}
	if f.showControls {
		if m.msg != "" {
			parts = append(parts, dimStyle.Render(m.msg))
		}
		parts = append(parts, m.renderActionBar())
	}
	if len(parts) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ---- Result banner + settlement breakdown ----

// renderBanner builds the centered result headline for the settled round from
// the net, plus the (ticking) bankroll line beneath it.
func (m Model) renderBanner() string {
	net := m.game.Result().Net
	var text string
	var style lipgloss.Style
	win := false
	switch {
	case net > 0:
		text, style, win = fmt.Sprintf("YOU WIN +$%d", net), winStyle, true
	case net < 0:
		text, style = fmt.Sprintf("YOU LOSE -$%d", -net), loseStyle
	default:
		text, style = "PUSH", pushStyle
	}
	base := style
	if win {
		style = style.Blink(true)
	}
	outcome := tui.HandIndent + style.Render(text)
	bankroll := tui.HandIndent + base.Render(fmt.Sprintf("$%d", m.displayBankroll))
	return outcome + "\n" + bankroll
}

// renderSettlement breaks out every payout component from the engine's
// Settlement: the dealer-qualification note, the Ante, the Play, the Ante Bonus
// (only if it applies), and the Pair Plus, each with its own colored money delta.
func (m Model) renderSettlement() string {
	s := m.game.Result()
	var lines []string

	// Dealer qualification / fold context.
	switch {
	case s.Folded:
		lines = append(lines, dimStyle.Render("Folded — Ante forfeited."))
	case s.Played && !s.DealerQualified:
		lines = append(lines, dimStyle.Render("Dealer doesn't qualify — Ante pays, Play returned."))
	case s.Played && s.DealerQualified:
		lines = append(lines, dimStyle.Render("Dealer qualifies — hands compared."))
	}

	if s.Ante.Bet > 0 {
		lines = append(lines, componentLine("Ante", s.Ante))
	}
	if s.Played {
		lines = append(lines, componentLine("Play", s.Play))
	}
	if s.AnteBonus.Applies {
		lines = append(lines, tui.HandIndent+bonusStyle.Render(
			fmt.Sprintf("Ante Bonus (%s)  +$%d", s.AnteBonus.Category.String(), s.AnteBonus.Net)))
	}
	if s.PairPlus.Bet > 0 {
		lines = append(lines, componentLine("Pair Plus", s.PairPlus))
	}

	net := netStyle(s.Net).Render(fmt.Sprintf("Net %s", signed(s.Net)))
	lines = append(lines, tui.HandIndent+net)
	return strings.Join(lines, "\n")
}

// componentLine renders one settled wager as "Label  $bet  <delta>", coloring
// the delta green for a win, grey for a push, red for a loss.
func componentLine(label string, c engine.ComponentResult) string {
	var delta string
	switch c.Outcome {
	case engine.OutcomeWin:
		delta = winStyle.Render(fmt.Sprintf("win +$%d", c.Net))
	case engine.OutcomePush:
		delta = pushStyle.Render("push")
	case engine.OutcomeLoss:
		delta = loseStyle.Render(fmt.Sprintf("loss -$%d", -c.Net))
	default:
		delta = dimStyle.Render("—")
	}
	return tui.HandIndent + fmt.Sprintf("%-10s %s  %s",
		label, dimStyle.Render(fmt.Sprintf("$%d", c.Bet)), delta)
}

// netStyle picks the color for a net figure: green positive, grey zero, red
// negative.
func netStyle(net int) lipgloss.Style {
	switch {
	case net > 0:
		return winStyle
	case net < 0:
		return loseStyle
	default:
		return pushStyle
	}
}

// signed formats a money delta with an explicit sign (+$5, -$3, $0).
func signed(n int) string {
	switch {
	case n > 0:
		return fmt.Sprintf("+$%d", n)
	case n < 0:
		return fmt.Sprintf("-$%d", -n)
	default:
		return "$0"
	}
}

// ---- Action bar ----

// renderActionBar renders the phase's control panel with a small dim key hint.
func (m Model) renderActionBar() string {
	var title, body, hint string
	switch m.game.Phase() {
	case engine.PhaseBetting:
		title = "Place your wagers"
		body = m.renderBetTiles()
		hint = m.bettingHint()
	case engine.PhaseDecision:
		title = "Your move"
		actions := m.decisionActions()
		labels := make([]string, len(actions))
		for i, a := range actions {
			labels[i] = decisionLabel(a)
		}
		body = tui.RenderMenu(labels, tui.ClampIdx(m.cursor, len(actions)))
		if !m.game.CanPlay() {
			body += "\n" + dimStyle.Render("Not enough bankroll to Play — Fold only.")
		}
		hint = "← → choose · p play · f fold · enter confirm · ? paytable · q lobby · Q quit"
	case engine.PhaseRoundOver:
		title = "Round over"
		body = tui.RenderMenu([]string{"Next hand"}, 0)
		hint = "enter continue · q lobby · Q quit"
	case engine.PhaseGameOver:
		title = "Game over"
		body = tui.RenderMenu([]string{"Restart", "Quit"}, tui.ClampIdx(m.cursor, 2))
		hint = "← → choose · enter confirm · q lobby · Q quit"
	}
	return tui.TitledBox(title, body) + "\n " + dimStyle.Render(hint)
}

// renderBetTiles lays out the Ante and Pair Plus wager tiles. The focused tile
// is gold-bordered and shows ◀ $N ▶; the other sits in a plain border. A
// staked/after-deal total line sits beneath them.
func (m Model) renderBetTiles() string {
	labels := [numSpots]string{"Ante", "Pair Plus"}
	tiles := make([]string, numSpots)
	for i := 0; i < numSpots; i++ {
		var value string
		if i == m.focusedSpot {
			value = betStyle.Render(fmt.Sprintf("◀ $%d ▶", m.spotBet(i)))
		} else {
			value = fmt.Sprintf("$%d", m.spotBet(i))
		}
		w := lipgloss.Width(labels[i])
		if vw := lipgloss.Width(value); vw > w {
			w = vw
		}
		inner := tui.Center(labels[i], w) + "\n" + tui.Center(value, w)
		if i == m.focusedSpot {
			tiles[i] = focusedTileStyle.Render(inner)
		} else {
			tiles[i] = plainTileStyle.Render(inner)
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, tiles...)

	total := m.game.Ante() + m.game.PairPlus()
	after := m.game.Bankroll() - total
	totalLine := dimStyle.Render(fmt.Sprintf("Staked $%d · after deal $%d", total, after))
	return row + "\n" + totalLine
}

// bettingHint builds the contextual key hint for the betting screen.
func (m Model) bettingHint() string {
	parts := []string{
		fmt.Sprintf("← → ±$%d", engine.MinBet),
		fmt.Sprintf("↑ ↓ ±$%d", coarseBetStep),
		"tab ⇄ switch",
		"enter deal",
		"? paytable",
		"q lobby · Q quit",
	}
	return strings.Join(parts, " · ")
}
