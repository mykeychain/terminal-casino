// Package ui is the Bubble Tea front end for the I Luv Suits table. It renders
// the engine's state and forwards player intent as engine actions; it computes no
// game logic. The presentation mirrors the Three-Card Poker table — an anchored
// top/middle/bottom frame, an arrow-navigated action bar, and a feel tier that
// cascades the deal in and flips the dealer's hand at showdown — with seven-card
// hands grouped by suit and the best flush underlined in gold.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/iluvsuits/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// compactWidthThreshold: below this terminal width, cards are packed edge-to-edge
// so a seven-card hand still fits.
const compactWidthThreshold = 80

// coarseBetStep is the larger bet increment used by Up/Down (PgUp/PgDn) for
// moving a wager quickly; plain Left/Right steps by MinBet. It is a multiple of
// MinBet so wagers always stay on the MinBet grid.
const coarseBetStep = 5 * engine.MinBet

// Bet-tile indices for the three wagers set during betting.
const (
	spotAnte = iota
	spotFlushRush
	spotSuperFlushRush
	numSpots
)

// spotLabels names each bet tile.
var spotLabels = [numSpots]string{"Ante", "Flush Rush", "Super Flush Rush"}

// foldAction is the sentinel decision value for Fold; any other value is a Play
// multiple (1, 2, or 3) of the Ante.
const foldAction = 0

// Model is the Bubble Tea model wrapping an engine.Game. Navigation is
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

	// focusedSpot is the wager tile being edited during PhaseBetting.
	focusedSpot int

	// msg is a transient status line (e.g. a rejected bet).
	msg string

	// showPaytable toggles the full-screen paytable reference overlay (`?`).
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
// freshly-seeded game when the player restarts after game over. It returns a
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

	// Input is locked while an animation runs. During the result the Next-hand
	// control is shown, so the confirm key finishes the count-up and advances.
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
		// holds this value in the header so it does not visibly drop.
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
		return fmt.Sprintf("Ante must be at least $%d and all wagers within your bankroll.", engine.MinBet)
	}
	return "Cannot deal: " + err.Error()
}

// adjustBet moves the focused wager by delta, snapping to the MinBet grid and
// capping so the three wagers never exceed the bankroll. The Ante floors at the
// table minimum; a side bet can be cleared to $0 (no wager).
func (m *Model) adjustBet(delta int) {
	others := 0
	for i := 0; i < numSpots; i++ {
		if i != m.focusedSpot {
			others += m.spotBet(i)
		}
	}
	target := m.spotBet(m.focusedSpot) + delta
	if maxForSpot := m.game.Bankroll() - others; target > maxForSpot {
		target = maxForSpot
	}
	// Snap to a MinBet multiple; anything in (0, MinBet) collapses to 0.
	target = (target / engine.MinBet) * engine.MinBet
	if target < 0 {
		target = 0
	}

	var err error
	switch m.focusedSpot {
	case spotAnte:
		if target < engine.MinBet {
			target = engine.MinBet
		}
		err = m.game.SetAnte(target)
	case spotFlushRush:
		err = m.game.SetFlushRush(target)
	default:
		err = m.game.SetSuperFlushRush(target)
	}
	if err != nil {
		m.msg = "bet: " + err.Error()
		return
	}
	m.msg = ""
}

// spotBet returns the pending wager for a bet-tile index.
func (m Model) spotBet(spot int) int {
	switch spot {
	case spotAnte:
		return m.game.Ante()
	case spotFlushRush:
		return m.game.FlushRush()
	default:
		return m.game.SuperFlushRush()
	}
}

// ---- Decision ----

func (m Model) handleDecision(key string) (tea.Model, tea.Cmd) {
	actions := m.decisionActions()
	m = m.syncCursor(fmt.Sprintf("decision|%v", actions), len(actions))
	switch key {
	case "left", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "right", "down":
		if m.cursor < len(actions)-1 {
			m.cursor++
		}
	case "f":
		return m.dispatchDecision(foldAction)
	case "1", "2", "3":
		if mult := int(key[0] - '0'); m.game.CanPlay(mult) {
			return m.dispatchDecision(mult)
		}
	case "enter", " ":
		return m.dispatchDecision(actions[m.cursor])
	}
	return m, nil
}

// decisionActions lists the legal choices in menu order: each affordable Play
// multiple the flush permits, then Fold (always legal).
func (m Model) decisionActions() []int {
	return append(m.game.LegalPlays(), foldAction)
}

// dispatchDecision applies a menu choice: foldAction folds, any other value Plays
// that multiple of the Ante. Both settle the round, so the dealer reveal begins.
func (m Model) dispatchDecision(action int) (tea.Model, tea.Cmd) {
	var err error
	if action == foldAction {
		err = m.game.Fold()
	} else {
		err = m.game.Play(action)
	}
	if err != nil {
		m.msg = decisionLabel(action, m.game.Ante()) + ": " + err.Error()
		return m, nil
	}
	m.msg = ""
	return m.afterDecision()
}

// decisionLabel renders a menu choice. A Play shows its multiple and its dollar
// cost at the current Ante (e.g. "Play 2× $20").
func decisionLabel(action, ante int) string {
	if action == foldAction {
		return "Fold"
	}
	return fmt.Sprintf("Play %d× $%d", action, action*ante)
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
	middleH := m.height - lipgloss.Height(top) - lipgloss.Height(bottom)
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
	if bottom := m.renderBottom(f); bottom != "" {
		b.WriteString("\n")
		b.WriteString(bottom)
	}
	return b.String()
}

// renderTop builds the fixed top zone: the title pinned to row 0 and the header
// beneath it.
func (m Model) renderTop(f revealFrame) string {
	return titleStyle.Render("I Luv Suits") + "\n" + m.renderHeader(f)
}

// renderHeader shows the persistent standing: bankroll and the current wagers.
func (m Model) renderHeader(f revealFrame) string {
	parts := []string{"Bankroll " + moneyStyle.Render(fmt.Sprintf("$%d", m.displayBankroll))}

	wagers := []string{betStyle.Render(fmt.Sprintf("Ante $%d", m.game.Ante()))}
	if v := m.game.FlushRush(); v > 0 {
		wagers = append(wagers, betStyle.Render(fmt.Sprintf("Flush Rush $%d", v)))
	}
	if v := m.game.SuperFlushRush(); v > 0 {
		wagers = append(wagers, betStyle.Render(fmt.Sprintf("Super $%d", v)))
	}
	if v := m.game.PlayBet(); v > 0 {
		wagers = append(wagers, betStyle.Render(fmt.Sprintf("Play $%d", v)))
	}
	parts = append(parts, strings.Join(wagers, "  "))
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
			status += "  " + dimStyle.Render("(doesn't qualify — needs a 3-card 9-high flush)")
		}
	}
	if f.dealerCards == 0 {
		return label + "  " + status
	}

	cards := tui.RenderSlots(faces(dv.Cards), f.dealerFaceUp, f.dealerCards, m.compact(), short)
	marker := ""
	if f.dealerFaceUp >= handSize {
		marker = flushMarker(dv.Flush.Len(), handSize, m.compact())
	}
	return label + "  " + status + "\n" + m.cardRow(cards, short) + "\n" + marker
}

func (m Model) renderPlayerArea(f revealFrame, short bool) string {
	label := areaLabelStyle.Render("You")
	if f.playerCards == 0 {
		return label + "  " + dimStyle.Render("place your bets")
	}

	pv := m.game.Player()
	var status, marker string
	if f.playerCards >= handSize {
		status = nameStyle.Render(pv.Name)
		if m.game.Phase() == engine.PhaseDecision {
			status += "  " + dimStyle.Render(fmt.Sprintf("(play up to %d×)", m.game.MaxPlay()))
		}
		marker = flushMarker(pv.Flush.Len(), handSize, m.compact())
	} else {
		status = dimStyle.Render("dealing…")
	}
	cards := tui.RenderHand(faces(pv.Cards), f.playerCards, m.compact(), short)
	return label + "  " + status + "\n" + m.cardRow(cards, short) + "\n" + marker
}

// cardRow reserves a fixed-height row for a hand so the layout does not jump as
// cards arrive.
func (m Model) cardRow(cards string, short bool) string {
	return lipgloss.NewStyle().Height(tui.CardHeightFor(short)).Render(cards)
}

// renderBottom builds the fixed bottom zone: the result banner + settlement
// breakdown (when shown), the transient status line, and the phase's control
// panel + hint. It is empty mid-animation (before the result).
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

// renderBanner builds the result headline for the settled round from the net,
// with the (ticking) bankroll beside it.
func (m Model) renderBanner() string {
	net := m.game.Result().Net
	var text string
	var style lipgloss.Style
	switch {
	case net > 0:
		text, style = fmt.Sprintf("YOU WIN +$%d", net), winStyle.Blink(true)
	case net < 0:
		text, style = fmt.Sprintf("YOU LOSE -$%d", -net), loseStyle
	default:
		text, style = "PUSH", pushStyle
	}
	return tui.HandIndent + style.Render(text) + "   " + dimStyle.Render(fmt.Sprintf("bankroll $%d", m.displayBankroll))
}

// renderSettlement breaks out every payout component from the engine's
// Settlement: the showdown context, the Ante, the Play, and each side bet placed,
// each with its own colored money delta.
func (m Model) renderSettlement() string {
	s := m.game.Result()
	var lines []string

	var context string
	switch {
	case s.Folded:
		context = "Folded — Ante forfeited."
	case !s.DealerQualified:
		context = "Dealer doesn't qualify — Ante pays, Play returned."
	case s.Comparison > 0:
		context = "Your flush beats the dealer's."
	case s.Comparison < 0:
		context = "The dealer's flush beats yours."
	default:
		context = "Flushes tie — Ante and Play push."
	}
	lines = append(lines, tui.HandIndent+dimStyle.Render(context))

	lines = append(lines, componentLine("Ante", s.Ante))
	if s.Played {
		lines = append(lines, componentLine(fmt.Sprintf("Play %d×", s.PlayMultiple), s.Play))
	}
	if s.FlushRush.Bet > 0 {
		lines = append(lines, sideLine("Flush Rush", fmt.Sprintf("%d-card flush", s.FlushRush.Length), s.FlushRush))
	}
	if s.SuperFlushRush.Bet > 0 {
		lines = append(lines, sideLine("Super Rush", runName(s.SuperFlushRush.Length), s.SuperFlushRush))
	}
	return strings.Join(lines, "\n")
}

// runName describes a straight-flush run for the Super Flush Rush line.
func runName(n int) string {
	if n < 2 {
		return "no straight flush"
	}
	return fmt.Sprintf("%d-card straight flush", n)
}

// componentLine renders one settled wager as "Label  $bet  <delta>", coloring
// the delta green for a win, grey for a push, red for a loss.
func componentLine(label string, c engine.ComponentResult) string {
	return tui.HandIndent + fmt.Sprintf("%-11s %s  %s",
		label, dimStyle.Render(fmt.Sprintf("$%d", c.Bet)), delta(c))
}

// sideLine renders a settled side bet with the hand feature it paid on and, for
// a win, the multiplier applied.
func sideLine(label, feature string, r engine.SideResult) string {
	note := feature
	if r.Outcome == engine.OutcomeWin {
		note = fmt.Sprintf("%s %d:1", feature, r.Multiplier)
	}
	return componentLine(label, r.ComponentResult) + "  " + dimStyle.Render(note)
}

// delta renders a component's money change in its outcome color.
func delta(c engine.ComponentResult) string {
	switch c.Outcome {
	case engine.OutcomeWin:
		return winStyle.Render(fmt.Sprintf("win +$%d", c.Net))
	case engine.OutcomePush:
		return pushStyle.Render("push")
	case engine.OutcomeLoss:
		return loseStyle.Render(fmt.Sprintf("loss -$%d", -c.Net))
	default:
		return dimStyle.Render("—")
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
			labels[i] = decisionLabel(a, m.game.Ante())
		}
		body = tui.RenderMenu(labels, tui.ClampIdx(m.cursor, len(actions)))
		if len(actions) == 1 {
			body += "\n" + dimStyle.Render("Not enough bankroll to Play — Fold only.")
		}
		hint = "← → choose · 1/2/3 play · f fold · enter confirm · ? paytable · q lobby · Q quit"
	case engine.PhaseRoundOver:
		title = "Round over"
		body = tui.RenderMenu([]string{"Next hand"}, 0)
		hint = "enter continue · ? paytable · q lobby · Q quit"
	case engine.PhaseGameOver:
		title = "Game over"
		body = tui.RenderMenu([]string{"Restart", "Quit"}, tui.ClampIdx(m.cursor, 2))
		hint = "← → choose · enter confirm · q lobby · Q quit"
	}
	return tui.TitledBox(title, body) + "\n " + dimStyle.Render(hint)
}

// renderBetTiles lays out the three wager tiles. The focused tile is
// gold-bordered and shows ◀ $N ▶; the others sit in a plain border. A
// staked/after-deal total line sits beneath them.
func (m Model) renderBetTiles() string {
	tiles := make([]string, numSpots)
	for i := 0; i < numSpots; i++ {
		var value string
		if i == m.focusedSpot {
			value = betStyle.Render(fmt.Sprintf("◀ $%d ▶", m.spotBet(i)))
		} else {
			value = fmt.Sprintf("$%d", m.spotBet(i))
		}
		w := lipgloss.Width(spotLabels[i])
		if vw := lipgloss.Width(value); vw > w {
			w = vw
		}
		inner := tui.Center(spotLabels[i], w) + "\n" + tui.Center(value, w)
		if i == m.focusedSpot {
			tiles[i] = focusedTileStyle.Render(inner)
		} else {
			tiles[i] = plainTileStyle.Render(inner)
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, tiles...)

	total := m.game.Ante() + m.game.FlushRush() + m.game.SuperFlushRush()
	totalLine := dimStyle.Render(fmt.Sprintf("Staked $%d · after deal $%d", total, m.game.Bankroll()-total))
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
