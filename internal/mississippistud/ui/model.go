// Package ui is the Bubble Tea front end for the Mississippi Stud table. It
// renders the engine's state and forwards player intent as engine actions; it
// computes no game logic. The presentation mirrors the Three-Card Poker table —
// an anchored top/middle/bottom frame and an arrow-navigated action bar — but the
// reveal is player-driven: each street's Raise flips the next community card, so
// no timed animation is needed and every frame is a pure function of engine
// state (deterministic screenshots).
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/mississippistud/engine"
	"github.com/mykeychain/terminal-casino/internal/tui"
)

// compactWidthThreshold: below this terminal width, cards are packed edge-to-edge
// so the two hole cards and three community cards still fit comfortably.
const compactWidthThreshold = 80

// coarseBetStep is the larger Ante increment used by Up/Down (PgUp/PgDn) for
// moving the wager quickly; plain Left/Right steps by MinBet. It is a multiple of
// MinBet so the Ante always stays on the MinBet grid.
const coarseBetStep = 5 * engine.MinBet

// numCommunity is the fixed number of community cards (3rd, 4th, 5th Street).
const numCommunity = 3

// foldAction is the sentinel menu value for Fold; any other value is a raise
// multiple (1, 2, or 3) times the Ante.
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

	// msg is a transient status line (e.g. a rejected bet).
	msg string

	// showPaytable toggles the full-screen paytable reference overlay (`?`). It is
	// independent of the game Phase and can be pulled up at any time.
	showPaytable bool

	// animState is the presentation sub-state (see anim.go). boardShown is the
	// number of card positions still on the felt during the board-clear sweep.
	animState  animState
	boardShown int
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
	return Model{game: g, newGame: newGame}
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

	// Paytable overlay: `?` toggles it from any phase. While it is up it acts as a
	// modal — enter/space (or `?`) dismisses it and every other key is swallowed so
	// nothing acts behind it.
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

	// While the board is being swept clear, input is locked — except enter/space,
	// which skips straight to the next hand.
	if m.animState == animClearing {
		if key == "enter" || key == " " {
			return m.finishClear()
		}
		return m, nil
	}

	switch m.game.Phase() {
	case engine.PhaseBetting:
		return m.handleBetting(key)
	case engine.Phase3rdStreet, engine.Phase4thStreet, engine.Phase5thStreet:
		return m.handleStreet(key)
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
	switch key {
	case "right":
		m.adjustAnte(engine.MinBet)
	case "left":
		m.adjustAnte(-engine.MinBet)
	case "up", "pgup":
		m.adjustAnte(coarseBetStep)
	case "down", "pgdown":
		m.adjustAnte(-coarseBetStep)
	case "enter", " ":
		if err := m.game.Deal(); err != nil {
			m.msg = dealError(err)
			return m, nil
		}
		m.msg = ""
		m.cursor = 0
	}
	return m, nil
}

// dealError translates a rejected Deal into a player-facing message.
func dealError(err error) string {
	if err == engine.ErrInvalidBet {
		return fmt.Sprintf("Ante must be between $%d and your bankroll.", engine.MinBet)
	}
	return "Cannot deal: " + err.Error()
}

// adjustAnte moves the Ante by delta, snapping to the MinBet grid and clamping
// into [MinBet, bankroll]. The Ante is mandatory in Mississippi Stud, so it never
// clears to $0; it only floors at the table minimum.
func (m *Model) adjustAnte(delta int) {
	target := m.game.Ante() + delta
	if target < engine.MinBet {
		target = engine.MinBet
	}
	if target > m.game.Bankroll() {
		target = m.game.Bankroll()
	}
	// Snap down to a MinBet multiple (a bankroll cap need not be one).
	target = (target / engine.MinBet) * engine.MinBet
	if target < engine.MinBet {
		target = engine.MinBet
	}
	if err := m.game.SetAnte(target); err != nil {
		m.msg = "bet: " + err.Error()
		return
	}
	m.msg = ""
}

// ---- Street decisions ----

// handleStreet drives the fold-or-raise choice shared by 3rd, 4th, and 5th
// Street. The menu is Fold followed by the affordable raise multiples; a raise
// reveals that street's community card and advances (settling after 5th Street).
func (m Model) handleStreet(key string) (tea.Model, tea.Cmd) {
	actions := m.streetActions()
	m = m.syncCursor("street|"+m.game.Phase().String()+"|"+fmt.Sprintf("%v", actions), len(actions))
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
		return m.dispatchStreet(foldAction)
	case "1", "2", "3":
		mult := int(key[0] - '0')
		if m.game.CanRaise(mult) {
			return m.dispatchStreet(mult)
		}
	case "enter", " ":
		if len(actions) > 0 {
			return m.dispatchStreet(actions[m.cursor])
		}
	}
	return m, nil
}

// streetActions lists the legal street choices in menu order: Fold first (always
// legal), then each affordable raise multiple. When no raise is affordable only
// Fold is offered.
func (m Model) streetActions() []int {
	out := []int{foldAction}
	out = append(out, m.game.LegalRaises()...)
	return out
}

// dispatchStreet applies a menu choice: foldAction folds, any other value raises
// by that multiple of the Ante. On success the engine reveals the street's card
// and advances (a 5th-Street raise settles the hand).
func (m Model) dispatchStreet(action int) (tea.Model, tea.Cmd) {
	var err error
	if action == foldAction {
		err = m.game.Fold()
	} else {
		err = m.game.Raise(action)
	}
	if err != nil {
		m.msg = streetLabel(action, m.game.Ante()) + ": " + err.Error()
		return m, nil
	}
	m.msg = ""
	m.cursor = 0
	return m, nil
}

// streetLabel renders a menu choice's label. Fold is plain; a raise shows its
// multiple AND its actual dollar cost at the current Ante (e.g. "2× $18"), so the
// player always sees what a raise costs.
func streetLabel(action, ante int) string {
	if action == foldAction {
		return "Fold"
	}
	return fmt.Sprintf("%d× $%d", action, action*ante)
}

// ---- Round over / game over ----

func (m Model) handleRoundOver(key string) (tea.Model, tea.Cmd) {
	m = m.syncCursor("round-over", 1)
	if key == "enter" || key == " " {
		// Sweep the settled hand off the felt before the next hand's betting
		// screen; finishClear (once the sweep completes) advances the engine.
		return m.startClear()
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
			m.msg = ""
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

// dealt reports whether a hand is in progress (hole cards are on the felt).
func (m Model) dealt() bool { return len(m.game.HoleCards()) > 0 }

// View implements tea.Model. It composes the screen as three anchored zones — a
// fixed top (title + header), a fixed bottom (banner + control panel), and a
// flexible middle (the table) — sized to exactly m.height × m.width. Before the
// first WindowSizeMsg it falls back to simple top-to-bottom stacking.
func (m Model) View() string {
	if m.showPaytable {
		return m.viewPaytable()
	}
	if m.width == 0 || m.height == 0 {
		return m.viewStacked()
	}

	top := m.renderTop()
	bottom := m.renderBottom()
	topH := lipgloss.Height(top)
	bottomH := lipgloss.Height(bottom)
	middleH := m.height - topH - bottomH
	if middleH < 0 {
		middleH = 0
	}

	middle := m.renderTable(false)
	if lipgloss.Height(middle) > middleH {
		middle = m.renderTable(true)
	}
	middleRegion := tui.FitHeight(middle, middleH)

	body := lipgloss.JoinVertical(lipgloss.Left, top, middleRegion, bottom)
	body = tui.FitHeight(body, m.height)
	return lipgloss.NewStyle().Width(m.width).Render(body)
}

// viewStacked is the pre-size fallback used only before the first WindowSizeMsg.
func (m Model) viewStacked() string {
	var b strings.Builder
	b.WriteString(m.renderTop())
	b.WriteString("\n")
	b.WriteString(m.renderTable(false))
	bottom := m.renderBottom()
	if bottom != "" {
		b.WriteString("\n")
		b.WriteString(bottom)
	}
	return b.String()
}

// renderTop builds the fixed top zone: the title pinned to row 0 and the header
// beneath it.
func (m Model) renderTop() string {
	return titleStyle.Render("Mississippi Stud") + "\n" + m.renderHeader()
}

// renderHeader shows the persistent standing: the live bankroll, the Ante, and —
// once a hand is under way — the total at risk, the game's core tension figure.
func (m Model) renderHeader() string {
	parts := []string{"Bankroll " + moneyStyle.Render(fmt.Sprintf("$%d", m.game.Bankroll()))}
	parts = append(parts, betStyle.Render(fmt.Sprintf("Ante $%d", m.game.Ante())))
	if total := m.game.TotalWagered(); total > 0 {
		parts = append(parts, riskStyle.Render(fmt.Sprintf("At risk $%d", total)))
	}
	return strings.Join(parts, "    ")
}

// renderTable builds the flexible middle zone at the requested vertical density:
// the community board on top, the running-total ledger in the middle, and the
// player's own hand at the bottom (closest to the player), mirroring the
// board-over-hand layout the other tables use for dealer-over-player.
func (m Model) renderTable(short bool) string {
	if !m.dealt() {
		return areaLabelStyle.Render("Your hand") + "  " + dimStyle.Render("set your ante and deal")
	}
	holeShown, communityShown := m.boardCounts()
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderCommunityArea(short, communityShown),
		m.renderLedger(),
		m.renderHoleArea(short, holeShown),
	)
}

func (m Model) renderHoleArea(short bool, shown int) string {
	head := areaLabelStyle.Render("Your hand")
	// The status (final hand name / made-hand hint) is hidden during the clear
	// sweep, so the cards leave against a plain label.
	if m.animState != animClearing {
		if status := m.holeStatus(); status != "" {
			head += "  " + status
		}
	}
	hole := m.game.HoleCards()
	cards := lipgloss.NewStyle().Height(tui.CardHeightFor(short)).
		Render(tui.RenderHand(faces(hole), shown, m.compact(), short))
	return head + "\n" + cards
}

// holeStatus is the note shown beside the hole-card label. At showdown it names
// the final five-card hand (as the other tables name the player's hand); during
// play it carries the subtle made-hand floor hint, or nothing when no floor is
// locked yet.
func (m Model) holeStatus() string {
	if m.game.Phase() == engine.PhaseRoundOver {
		if s := m.game.Result(); !s.Folded && s.HandName != "" {
			return nameStyle.Render(s.HandName)
		}
		return ""
	}
	return m.madeHandHint()
}

func (m Model) renderCommunityArea(short bool, shown int) string {
	community := m.game.CommunityCards()
	revealed := revealedCount(community)
	faceUp := revealed
	if faceUp > shown {
		faceUp = shown
	}

	head := areaLabelStyle.Render("Community")
	// The reveal-status note is hidden during the clear sweep, so the cards leave
	// against a plain label.
	if m.animState != animClearing {
		var status string
		switch {
		case revealed == 0:
			status = dimStyle.Render("all face down")
		case revealed < numCommunity:
			status = dimStyle.Render(fmt.Sprintf("%d of %d revealed", revealed, numCommunity))
		default:
			status = dimStyle.Render("all revealed")
		}
		head += "  " + status
	}

	cards := lipgloss.NewStyle().Height(tui.CardHeightFor(short)).
		Render(tui.RenderSlots(communityFaces(community), faceUp, shown, m.compact(), short))
	return head + "\n" + cards
}

// renderLedger is the running-total display — the heart of Mississippi Stud. It
// breaks out the Ante and every street bet placed so far, then the total at risk,
// so the player always sees exactly how much is on the line. Streets not yet
// raised show a dim placeholder.
func (m Model) renderLedger() string {
	cells := []string{fmt.Sprintf("Ante $%d", m.game.Ante())}
	labels := [numCommunity]string{"3rd", "4th", "5th"}
	for i, bet := range m.game.StreetBets() {
		if bet > 0 {
			cells = append(cells, betStyle.Render(fmt.Sprintf("%s $%d", labels[i], bet)))
		} else {
			cells = append(cells, dimStyle.Render(labels[i]+" —"))
		}
	}
	ledger := dimStyle.Render(strings.Join(cells, "  ·  "))
	risk := riskStyle.Render(fmt.Sprintf("Total at risk $%d", m.game.TotalWagered()))
	return ledger + "    " + risk
}

// madeHandHint surfaces the engine's locked-in floor as a quiet, non-nagging
// note: a win floor ("already paying") once a Jacks-or-better pair is visible, or
// a push floor ("already pushing") for a 6s–10s pair. It names the pair so the
// surprise 6s–10s PUSH is easier to anticipate. Empty when no floor is locked.
func (m Model) madeHandHint() string {
	push, win := m.game.VisibleFloor()
	if !push && !win {
		return ""
	}
	label := "pair"
	if name, ok := highestVisiblePair(m.game.HoleCards(), m.game.CommunityCards()); ok {
		label = "pair of " + name
	}
	if win {
		return dimStyle.Render(label + " — already paying")
	}
	return dimStyle.Render(label + " — already pushing")
}

// renderBottom builds the fixed bottom zone: the result banner + settlement
// breakdown (when the hand is over), the transient status line, and the phase's
// control panel + hint.
func (m Model) renderBottom() string {
	var parts []string
	if m.game.Phase() == engine.PhaseRoundOver {
		parts = append(parts, m.renderSettlement()) // the payout details…
		parts = append(parts, m.renderBanner())     // …then the win / push / loss headline
	}
	// While the felt is being swept clear the result stays up, but the action bar
	// is dropped — input is locked until the next hand's betting screen appears.
	if m.animState == animClearing {
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	if m.msg != "" {
		parts = append(parts, dimStyle.Render(m.msg))
	}
	parts = append(parts, m.renderActionBar())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ---- Result banner + settlement breakdown ----

// renderBanner builds the centered result headline for the settled hand, keyed on
// the engine's Outcome so a PUSH reads as a push (not a win or loss) — the 6s–10s
// pair that surprises new players.
func (m Model) renderBanner() string {
	s := m.game.Result()
	var text string
	var style lipgloss.Style
	switch s.Outcome {
	case engine.OutcomeWin:
		// Blink the win headline, matching the other tables' result banner.
		text, style = fmt.Sprintf("YOU WIN +$%d", s.Net), winStyle.Blink(true)
	case engine.OutcomePush:
		text, style = "PUSH", pushStyle
	default:
		text, style = fmt.Sprintf("YOU LOSE -$%d", -s.Net), loseStyle
	}
	return tui.HandIndent + style.Render(text)
}

// renderSettlement makes the payout math transparent: it names the final hand and
// shows the multiplier applied to the total wagered, broken out so the arithmetic
// is visible (e.g. "Flush · 6:1 on $250 wagered → +$1500"). A fold shows the
// forfeited total; a push and a loss are spelled out distinctly.
func (m Model) renderSettlement() string {
	s := m.game.Result()
	var line string
	switch {
	case s.Folded:
		line = dimStyle.Render(fmt.Sprintf("Folded — $%d forfeited.", s.TotalWagered))
	case s.Outcome == engine.OutcomeWin:
		line = fmt.Sprintf("%s  %s  %s  %s",
			nameStyle.Render(s.HandName),
			betStyle.Render(fmt.Sprintf("%d:1", s.Multiplier)),
			dimStyle.Render(fmt.Sprintf("on $%d wagered", s.TotalWagered)),
			winStyle.Render(fmt.Sprintf("→ +$%d", s.Profit)))
	case s.Outcome == engine.OutcomePush:
		line = fmt.Sprintf("%s  %s  %s",
			nameStyle.Render(s.HandName),
			pushStyle.Render("pushes"),
			dimStyle.Render(fmt.Sprintf("— $%d returned", s.TotalWagered)))
	default: // OutcomeLoss (not folded)
		line = fmt.Sprintf("%s  %s  %s",
			nameStyle.Render(s.HandName),
			loseStyle.Render("loses"),
			dimStyle.Render(fmt.Sprintf("— $%d forfeited", s.TotalWagered)))
	}
	return tui.HandIndent + line
}

// ---- Action bar ----

// renderActionBar renders the phase's control panel with a small dim key hint.
func (m Model) renderActionBar() string {
	var title, body, hint string
	switch m.game.Phase() {
	case engine.PhaseBetting:
		title = "Set your ante"
		body = m.renderAnteTile()
		hint = m.bettingHint()
	case engine.Phase3rdStreet, engine.Phase4thStreet, engine.Phase5thStreet:
		title = m.game.StreetLabel() + " — fold or raise"
		body = m.renderStreetMenu()
		hint = "← → choose · 1/2/3 raise · f fold · enter · ? paytable · q lobby · Q quit"
	case engine.PhaseRoundOver:
		title = "Hand settled"
		body = tui.RenderMenu([]string{"Next hand"}, 0)
		hint = "enter continue · ? paytable · q lobby · Q quit"
	case engine.PhaseGameOver:
		title = "Out of chips"
		body = tui.RenderMenu([]string{"Restart", "Quit"}, tui.ClampIdx(m.cursor, 2))
		hint = "← → choose · enter confirm · q lobby · Q quit"
	}
	return tui.TitledBox(title, body) + "\n " + dimStyle.Render(hint)
}

// renderAnteTile draws the single, always-focused Ante wager tile (◀ $N ▶) with a
// line beneath showing what remains after the Ante is escrowed on the deal.
func (m Model) renderAnteTile() string {
	value := betStyle.Render(fmt.Sprintf("◀ $%d ▶", m.game.Ante()))
	w := lipgloss.Width("Ante")
	if vw := lipgloss.Width(value); vw > w {
		w = vw
	}
	inner := tui.Center("Ante", w) + "\n" + tui.Center(value, w)
	tile := focusedTileStyle.Render(inner)
	after := m.game.Bankroll() - m.game.Ante()
	note := dimStyle.Render(fmt.Sprintf("after deal $%d", after))
	return tile + "\n" + note
}

// renderStreetMenu lays out the Fold / raise choices as an arrow menu, each raise
// carrying its dollar cost. When no raise is affordable it notes that only Fold
// is legal.
func (m Model) renderStreetMenu() string {
	actions := m.streetActions()
	labels := make([]string, len(actions))
	for i, a := range actions {
		labels[i] = streetLabel(a, m.game.Ante())
	}
	body := tui.RenderMenu(labels, tui.ClampIdx(m.cursor, len(actions)))
	if len(m.game.LegalRaises()) == 0 {
		body += "\n" + dimStyle.Render("Not enough bankroll to raise — Fold only.")
	}
	return body
}

// bettingHint builds the contextual key hint for the betting screen.
func (m Model) bettingHint() string {
	parts := []string{
		fmt.Sprintf("← → ±$%d", engine.MinBet),
		fmt.Sprintf("↑ ↓ ±$%d", coarseBetStep),
		"enter deal",
		"? paytable",
		"q lobby · Q quit",
	}
	return strings.Join(parts, " · ")
}
