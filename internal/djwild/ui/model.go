// Package ui is the Bubble Tea front end for the DJ Wild table. It renders the
// engine's state and forwards player intent as engine actions; it computes no
// game logic. The presentation mirrors the Three-Card Poker table — an anchored
// top/middle/bottom frame, an arrow-navigated action bar, and a feel tier that
// cascades the deal in and flips the dealer's hand at showdown — with every wild
// card (the deuces and the joker) labeled beneath the hand.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/djwild/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// compactWidthThreshold: below this terminal width, cards are packed edge-to-edge.
const compactWidthThreshold = 80

// coarseBetStep is the larger bet increment used by Up/Down (PgUp/PgDn) for
// moving a wager quickly; plain Left/Right steps by MinBet. It is a multiple of
// MinBet so wagers always stay on the MinBet grid.
const coarseBetStep = 5 * engine.MinBet

// Bet-tile indices for the wagers set during betting. The Blind is not a tile
// of its own: it always matches the Ante, so the Ante tile sets both.
const (
	spotAnte = iota
	spotTrips
	numSpots
)

// spotLabels names each bet tile.
var spotLabels = [numSpots]string{"Ante = Blind", "Trips"}

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
	case "tab", "shift+tab":
		m.focusedSpot = (m.focusedSpot + 1) % numSpots
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
		return fmt.Sprintf("Ante must be at least $%d, with Ante + Blind + Trips within your bankroll.", engine.MinBet)
	}
	return "Cannot deal: " + err.Error()
}

// adjustBet moves the focused wager by delta, snapping to the MinBet grid and
// capping so Ante + Blind + Trips never exceed the bankroll. The Ante floors at
// the table minimum; Trips can be cleared to $0 (no wager).
func (m *Model) adjustBet(delta int) {
	var err error
	if m.focusedSpot == spotAnte {
		target := m.game.Ante() + delta
		if maxAnte := (m.game.Bankroll() - m.game.Trips()) / 2; target > maxAnte {
			target = maxAnte
		}
		target = (target / engine.MinBet) * engine.MinBet
		if target < engine.MinBet {
			target = engine.MinBet
		}
		err = m.game.SetAnte(target)
	} else {
		target := m.game.Trips() + delta
		if maxTrips := m.game.Bankroll() - 2*m.game.Ante(); target > maxTrips {
			target = maxTrips
		}
		// Snap to a MinBet multiple; anything in (0, MinBet) collapses to 0.
		target = (target / engine.MinBet) * engine.MinBet
		if target < 0 {
			target = 0
		}
		err = m.game.SetTrips(target)
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
	return m.game.Trips()
}

// ---- Decision ----

// decisionAction is the small set of decision-phase intents (kept local to the
// UI; the engine exposes Play / Fold directly).
type decisionAction int

const (
	actionPlay decisionAction = iota
	actionFold
)

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
	case "p":
		if m.game.CanPlay() {
			return m.dispatchDecision(actionPlay)
		}
	case "f":
		return m.dispatchDecision(actionFold)
	case "enter", " ":
		return m.dispatchDecision(actions[m.cursor])
	}
	return m, nil
}

// decisionActions lists the legal choices in menu order. Play is offered only
// when the bankroll covers it; Fold is always available.
func (m Model) decisionActions() []decisionAction {
	var out []decisionAction
	if m.game.CanPlay() {
		out = append(out, actionPlay)
	}
	return append(out, actionFold)
}

// dispatchDecision applies a menu choice. Both Play and Fold settle the round,
// so the dealer reveal begins.
func (m Model) dispatchDecision(a decisionAction) (tea.Model, tea.Cmd) {
	var err error
	if a == actionPlay {
		err = m.game.Play()
	} else {
		err = m.game.Fold()
	}
	if err != nil {
		m.msg = m.decisionLabel(a) + ": " + err.Error()
		return m, nil
	}
	m.msg = ""
	return m.afterDecision()
}

// decisionLabel renders a menu choice. Play shows its dollar cost (2× the Ante).
func (m Model) decisionLabel(a decisionAction) string {
	if a == actionPlay {
		return fmt.Sprintf("Play %d× $%d", engine.PlayMultiple, engine.PlayMultiple*m.game.Ante())
	}
	return "Fold"
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
	return titleStyle.Render("DJ Wild") + "  " + dimStyle.Render("deuces & joker wild") + "\n" + m.renderHeader(f)
}

// renderHeader shows the persistent standing: bankroll and the current wagers.
func (m Model) renderHeader(f revealFrame) string {
	parts := []string{"Bankroll " + moneyStyle.Render(fmt.Sprintf("$%d", m.displayBankroll))}

	wagers := []string{
		betStyle.Render(fmt.Sprintf("Ante $%d", m.game.Ante())),
		betStyle.Render(fmt.Sprintf("Blind $%d", m.game.Blind())),
	}
	if v := m.game.Trips(); v > 0 {
		wagers = append(wagers, betStyle.Render(fmt.Sprintf("Trips $%d", v)))
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
		status = dimStyle.Render("hand hidden · the dealer always plays")
	default:
		status = handStatus(dv.Value)
	}
	if f.dealerCards == 0 {
		return label + "  " + status
	}

	cards := tui.RenderSlots(faces(dv.Cards), f.dealerFaceUp, f.dealerCards, m.compact(), short)
	return label + "  " + status + "\n" + m.cardRow(cards, short) + "\n" +
		wildMarker(dv.Cards, f.dealerFaceUp, m.compact())
}

func (m Model) renderPlayerArea(f revealFrame, short bool) string {
	label := areaLabelStyle.Render("You")
	if f.playerCards == 0 {
		return label + "  " + dimStyle.Render("place your bets")
	}

	pv := m.game.Player()
	status := dimStyle.Render("dealing…")
	if f.playerCards >= handSize {
		status = handStatus(pv.Value)
	}
	cards := tui.RenderHand(faces(pv.Cards), f.playerCards, m.compact(), short)
	return label + "  " + status + "\n" + m.cardRow(cards, short) + "\n" +
		wildMarker(pv.Cards, f.playerCards, m.compact())
}

// handStatus names an evaluated hand, noting whether it leans on wild cards.
func handStatus(v engine.HandValue) string {
	name := nameStyle.Render(v.Name())
	switch {
	case v.Category == engine.FiveWilds:
		return name
	case v.Wilds == 0:
		return name
	case v.Natural:
		return name + "  " + dimStyle.Render("(natural)")
	default:
		return name + "  " + dimStyle.Render("(with wilds)")
	}
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
// Settlement: the showdown context, the Ante, the Blind, the Play, and Trips,
// each with its own colored money delta.
func (m Model) renderSettlement() string {
	s := m.game.Result()
	pv := m.game.Player().Value

	var context string
	switch {
	case s.Folded:
		context = "Folded — Ante and Blind forfeited."
	case s.Comparison > 0:
		context = "Your hand beats the dealer's."
	case s.Comparison < 0:
		context = "The dealer's hand beats yours."
	default:
		context = "Hands tie — every wager pushes."
	}
	lines := []string{tui.HandIndent + dimStyle.Render(context)}

	lines = append(lines, componentLine("Ante", s.Ante, ""))
	var blindNote string
	switch {
	case s.Blind.Outcome == engine.OutcomeWin:
		blindNote = fmt.Sprintf("%s %d:1", pv.Category, s.Blind.Multiplier)
	case s.Blind.Outcome == engine.OutcomePush && s.Comparison > 0:
		blindNote = "pays on a Straight or better"
	}
	lines = append(lines, componentLine("Blind", s.Blind, blindNote))
	if s.Played {
		lines = append(lines, componentLine(fmt.Sprintf("Play %d×", engine.PlayMultiple), s.Play, ""))
	}
	if s.Trips.Bet > 0 {
		note := ""
		if s.Trips.Outcome == engine.OutcomeWin {
			kind := "wild"
			if pv.Natural {
				kind = "natural"
			}
			note = fmt.Sprintf("%s %s %d:1", kind, pv.Category, s.Trips.Multiplier)
			if pv.Category == engine.FiveWilds {
				note = fmt.Sprintf("Five Wilds %d:1", s.Trips.Multiplier)
			}
		}
		lines = append(lines, componentLine("Trips", s.Trips, note))
	}
	return strings.Join(lines, "\n")
}

// componentLine renders one settled wager as "Label  $bet  <delta>  note",
// coloring the delta green for a win, grey for a push, red for a loss.
func componentLine(label string, c engine.ComponentResult, note string) string {
	line := tui.HandIndent + fmt.Sprintf("%-8s %s  %s",
		label, dimStyle.Render(fmt.Sprintf("$%d", c.Bet)), delta(c))
	if note != "" {
		line += "  " + dimStyle.Render(note)
	}
	return line
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
			labels[i] = m.decisionLabel(a)
		}
		body = tui.RenderMenu(labels, tui.ClampIdx(m.cursor, len(actions)))
		if !m.game.CanPlay() {
			body += "\n" + dimStyle.Render("Not enough bankroll to Play — Fold only.")
		}
		hint = "← → choose · p play · f fold · enter confirm · ? paytable · q lobby · Q quit"
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

// renderBetTiles lays out the Ante (= Blind) and Trips wager tiles. The focused
// tile is gold-bordered and shows ◀ $N ▶; the other sits in a plain border. A
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

	total := 2*m.game.Ante() + m.game.Trips()
	totalLine := dimStyle.Render(fmt.Sprintf("Staked $%d · after deal $%d · Play costs 2× Ante", total, m.game.Bankroll()-total))
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
