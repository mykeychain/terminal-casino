package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/internal/blackjack32/engine"
)

// compactWidthThreshold: below this terminal width, or with more than two hands,
// cards are packed edge-to-edge so up to four split hands still fit.
const compactWidthThreshold = 80

// coarseBetStep is the larger bet increment used by Shift+arrow / PgUp-PgDn, for
// moving the wager quickly; plain arrows still step by engine.BetIncrement ($1).
const coarseBetStep = 25

// Model is the Bubble Tea model wrapping a frozen engine.Game. It renders engine
// state and forwards player intent as engine actions; it computes no game logic.
// Navigation is arrow-driven: the currently-legal choices are shown as a
// horizontal menu and cursor selects one, which is then dispatched to the
// matching engine method.
type Model struct {
	game    *engine.Game
	newGame func() *engine.Game // produces a fresh game for restart (seeded by main)

	width  int
	height int

	// cursor indexes the current phase's horizontal menu. It is clamped into the
	// live menu range every frame and reset to 0 whenever the menu's signature
	// changes (new phase, new active hand, a split, or a changed legal-action set).
	cursor int
	// menuSig is the signature of the menu the cursor currently indexes; when the
	// recomputed signature differs, the cursor resets to 0.
	menuSig string

	// focusedSpot indexes the bet tile currently being adjusted during
	// PhaseBetting. It is reset to 0 on entering betting (initial zero value and
	// after NextHand / restart) and clamped into [0, NumSpots()) on every betting
	// key and on add/remove.
	focusedSpot int

	// msg is a transient status line (e.g. rejected bet, reshuffle notice).
	msg string

	// ---- Feel-tier animation sub-state (independent of the engine Phase) ----
	// While animState != animIdle an animation is playing: input is gated and the
	// View renders from the reveal counters below instead of straight from the
	// engine. The engine remains the source of truth for the final state; these
	// fields only control when each piece of it becomes visible.
	animState animState

	// dealShown counts revealed positions during the initial deal cascade
	// (see dealPositions / frame). dealerShown counts dealer cards drawn face up
	// during the dealer reveal; holeFlipped records that the hole card is face up.
	dealShown   int
	dealerShown int
	holeFlipped bool

	// displayBankroll is the bankroll shown in the header. It is frozen at the
	// pre-deal value for the duration of a round (masking the escrow) and ticks
	// to the settled bankroll during the result effect, so its delta equals the
	// round's net. bankFrom / bankStep drive that count-up.
	displayBankroll int
	bankFrom        int
	bankStep        int
}

// New builds a Model over an already-constructed game. newGame is invoked to get
// a fresh, freshly-seeded game when the player restarts after game over; keeping
// the time seed in the caller preserves the engine's injected-RNG contract.
func New(game *engine.Game, newGame func() *engine.Game) Model {
	return Model{game: game, newGame: newGame, displayBankroll: game.Bankroll()}
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

	// Global quit.
	if key == "q" || key == "ctrl+c" {
		return m, tea.Quit
	}

	// Input is locked while an animation runs (locked decision 3). Keys other
	// than quit are ignored, not queued.
	if m.animState != animIdle {
		return m, nil
	}

	switch m.game.Phase() {
	case engine.PhaseBetting:
		return m.handleBetting(key)
	case engine.PhaseInsurance:
		return m.handleInsurance(key)
	case engine.PhasePlayerTurn:
		return m.handlePlayerTurn(key)
	case engine.PhaseRoundOver:
		return m.handleRoundOver(key)
	case engine.PhaseGameOver:
		return m.handleGameOver(key)
	}
	return m, nil
}

// syncCursor resets the cursor to 0 when the menu signature changed since the
// last frame, then clamps it into [0, n). Model is a value type, so callers
// reassign: m = m.syncCursor(...).
func (m Model) syncCursor(sig string, n int) Model {
	if sig != m.menuSig {
		m.menuSig = sig
		m.cursor = 0
	}
	m.cursor = clampIdx(m.cursor, n)
	return m
}

// clampIdx pins i into [0, n) (returns 0 for an empty menu).
func clampIdx(i, n int) int {
	if n <= 0 || i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func (m Model) handleBetting(key string) (tea.Model, tea.Cmd) {
	m.focusedSpot = clampIdx(m.focusedSpot, m.game.NumSpots())
	switch key {
	case "right", "up":
		m.adjustBet(engine.BetIncrement)
	case "left", "down":
		m.adjustBet(-engine.BetIncrement)
	case "shift+right", "shift+up", "pgup":
		m.adjustBet(coarseBetStep)
	case "shift+left", "shift+down", "pgdown":
		m.adjustBet(-coarseBetStep)
	case "tab":
		if m.focusedSpot < m.game.NumSpots()-1 {
			m.focusedSpot++
		}
	case "shift+tab":
		if m.focusedSpot > 0 {
			m.focusedSpot--
		}
	case "a":
		if err := m.game.AddSpot(); err != nil {
			m.msg = "cannot add hand: " + err.Error()
		} else {
			m.focusedSpot = m.game.NumSpots() - 1
			m.msg = ""
		}
	case "x", "backspace":
		if err := m.game.RemoveSpot(m.focusedSpot); err != nil {
			m.msg = "cannot remove hand: " + err.Error()
		} else {
			m.focusedSpot = clampIdx(m.focusedSpot, m.game.NumSpots())
			m.msg = ""
		}
	case "enter", " ":
		// Snapshot the bankroll before Deal escrows the stake; the deal reveal
		// holds this value in the header so it does not visibly drop, and it
		// becomes the start of the result count-up.
		pre := m.game.Bankroll()
		if err := m.game.Deal(); err != nil {
			m.msg = "cannot deal: " + err.Error()
			return m, nil
		}
		m.msg = ""
		if m.game.DidReshuffle() {
			m.msg = "Shoe reshuffled."
		}
		return m.startDealReveal(pre)
	}
	return m, nil
}

// adjustBet moves the focused spot's bet by delta (a BetIncrement step), clamped
// to [MinBet, bankroll − other spots' stakes] and snapped to a BetIncrement
// multiple, then pushes it through the engine. If the focused spot cannot even
// afford MinBet given the other stakes, the bet is left unchanged.
func (m *Model) adjustBet(delta int) {
	i := m.focusedSpot
	target := m.game.SpotBet(i) + delta
	if target < engine.MinBet {
		target = engine.MinBet
	}
	othersTotal := 0
	for j := 0; j < m.game.NumSpots(); j++ {
		if j != i {
			othersTotal += m.game.SpotBet(j)
		}
	}
	if maxForSpot := m.game.Bankroll() - othersTotal; target > maxForSpot {
		target = (maxForSpot / engine.BetIncrement) * engine.BetIncrement
	}
	if target%engine.BetIncrement != 0 {
		target = (target / engine.BetIncrement) * engine.BetIncrement
	}
	if target < engine.MinBet {
		return
	}
	if err := m.game.SetSpotBet(i, target); err != nil {
		m.msg = "bet: " + err.Error()
		return
	}
	m.msg = ""
}

func (m Model) handleInsurance(key string) (tea.Model, tea.Cmd) {
	// Two-option yes/no menu (0 = Yes, 1 = No). Not sourced from LegalActions.
	// Keying the signature on the offered spot re-presents the menu (defaulting to
	// Yes) for each hand as the engine advances through them.
	m = m.syncCursor(fmt.Sprintf("insurance|%d", m.game.InsuranceSpot()), 2)
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
		// Answering insurance can settle the round (dealer blackjack on peek);
		// displayBankroll already holds the pre-deal snapshot, so afterAction
		// can start the dealer reveal + result if the round is now over.
		_ = m.game.Insurance(m.cursor == 0)
		m.msg = ""
		return m.afterAction()
	}
	return m, nil
}

func (m Model) handlePlayerTurn(key string) (tea.Model, tea.Cmd) {
	actions := m.playerActions()
	m = m.syncCursor(playerTurnSig(m.game, actions), len(actions))

	switch key {
	case "left", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "right", "down":
		if m.cursor < len(actions)-1 {
			m.cursor++
		}
	case "enter", " ":
		if len(actions) == 0 {
			return m, nil
		}
		a := actions[m.cursor]
		// Defense in depth: re-check legality against the engine before calling.
		if !m.game.CanAct(a) {
			return m, nil
		}
		var err error
		switch a {
		case engine.ActionHit:
			err = m.game.Hit()
		case engine.ActionStand:
			err = m.game.Stand()
		case engine.ActionDouble:
			err = m.game.Double()
		case engine.ActionSplit:
			err = m.game.Split()
		}
		if err != nil {
			m.msg = a.String() + ": " + err.Error()
			return m, nil
		}
		m.msg = ""
		// Hit/Double reveal their card instantly (no per-card animation), but if
		// the action settled the round the dealer reveal + result play out.
		// displayBankroll still holds the pre-deal value, so it is the correct
		// pre-settlement start for the count-up.
		return m.afterAction()
	}
	return m, nil
}

// playerActions returns the currently-legal player-turn actions, in menu order,
// sourced entirely from the engine's legal-action set.
func (m Model) playerActions() []engine.Action {
	var out []engine.Action
	for _, a := range m.game.LegalActions() {
		switch a {
		case engine.ActionHit, engine.ActionStand, engine.ActionDouble, engine.ActionSplit:
			out = append(out, a)
		}
	}
	return out
}

// playerTurnSig identifies the current decision point. It changes on a new
// active hand, after a split (hand count changes), and whenever the legal-action
// set changes — any of which resets the menu cursor to 0.
func playerTurnSig(g *engine.Game, actions []engine.Action) string {
	return fmt.Sprintf("pt|%d|%d|%v", len(g.Player()), g.ActiveHandIndex(), actions)
}

func (m Model) handleRoundOver(key string) (tea.Model, tea.Cmd) {
	m = m.syncCursor("round-over", 1)
	if key == "enter" || key == " " {
		if err := m.game.NextHand(); err != nil {
			m.msg = err.Error()
		} else {
			m.msg = ""
			m.focusedSpot = 0
			m.displayBankroll = m.game.Bankroll()
		}
	}
	return m, nil
}

func (m Model) handleGameOver(key string) (tea.Model, tea.Cmd) {
	// Menu: 0 = Restart, 1 = Quit.
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
			m.focusedSpot = 0
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
	if len(m.game.Player()) > 2 {
		return true
	}
	return m.width > 0 && m.width < compactWidthThreshold
}

// View implements tea.Model.
func (m Model) View() string {
	f := m.frame()
	var b strings.Builder
	b.WriteString(titleStyle.Render("3:2 Blackjack"))
	b.WriteString("\n")
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")
	b.WriteString(m.renderDealerArea(f))
	b.WriteString("\n\n")
	b.WriteString(m.renderPlayerArea(f))
	b.WriteString("\n\n")
	if f.showBanner {
		b.WriteString(m.renderBanner())
		b.WriteString("\n\n")
	}
	// The transient status line and the action bar/prompt are hidden mid-animation
	// (controls unlock only when idle).
	if f.showControls {
		if m.msg != "" {
			b.WriteString(dimStyle.Render(m.msg))
			b.WriteString("\n\n")
		}
		b.WriteString(m.renderActionBar())
		b.WriteString("\n")
	}

	out := b.String()
	if m.width > 0 {
		out = lipgloss.NewStyle().Width(m.width).Render(out)
	}
	return out
}

func (m Model) renderDealerArea(f revealFrame) string {
	dv := m.game.Dealer()
	label := areaLabelStyle.Render("Dealer")

	var status string
	switch {
	case f.dealerCards == 0:
		status = dimStyle.Render("waiting for deal")
	case f.dealerFaceUp <= 1:
		// Only the up-card is face up (hole still hidden): report the up-card.
		status = dimStyle.Render(fmt.Sprintf("showing %d", dv.Upcard.Value()))
	case f.showOutcomes:
		// Whole dealer hand revealed and the round is settled: show its verdict.
		status = describeHandValue(dv.Value, dv.Soft, dv.Blackjack)
	default:
		// Mid-reveal (hole flipped, dealer still drawing): show the running total
		// of the revealed cards so it climbs — and busts — as each draw lands.
		rv := engine.Hand{Cards: dv.Cards[:f.dealerFaceUp]}
		status = describeHandValue(rv.Value(), rv.IsSoft(), false)
	}

	if len(m.game.Player()) == 0 {
		// No round in progress (betting): no dealer row to reserve.
		return label + "  " + status
	}
	// Reserve a fixed-height card row so the layout doesn't jump as the dealer's
	// cards cascade in — the row is blank space until the up-card lands.
	cards := lipgloss.NewStyle().Height(cardHeight).Render(renderDealerHandFrame(dv, f, m.compact()))
	return label + "  " + status + "\n" + cards
}

func (m Model) renderPlayerArea(f revealFrame) string {
	label := areaLabelStyle.Render("You")
	hands := m.game.Player()
	if len(hands) == 0 {
		return label + "  " + dimStyle.Render("place your bet")
	}

	roundOver := f.showOutcomes
	blocks := make([]string, len(hands))
	for i, h := range hands {
		count := len(h.Cards)
		if i < len(f.playerCards) {
			count = f.playerCards[i]
		}
		blocks[i] = m.renderHandBlock(i, h, len(hands) > 1, roundOver, count)
	}
	handsRow := lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
	// Overflow: if the horizontal row of hand blocks would exceed the terminal
	// width (even after compact cards), stack the blocks vertically so up to three
	// hands with their splits stay readable.
	if m.width > 0 && lipgloss.Width(handsRow) > m.width {
		handsRow = lipgloss.JoinVertical(lipgloss.Left, blocks...)
	}

	out := label + "\n" + handsRow
	if roundOver && len(hands) > 1 {
		net := 0
		for _, h := range hands {
			net += h.Net
		}
		out += "\n" + renderRoundNet(net)
	}
	return out
}

// renderRoundNet formats the round's summed per-hand net: green when non-negative
// (winStyle), red when negative (loseStyle).
func renderRoundNet(net int) string {
	if net >= 0 {
		return winStyle.Render(fmt.Sprintf("Round net +$%d", net))
	}
	return loseStyle.Render(fmt.Sprintf("Round net -$%d", -net))
}

// renderHandBlock renders one player hand: a caption, the cards, and a footer
// with value/bet (and outcome once the round is over). The active hand gets a
// highlighted border; others get an equal-size invisible border so nothing jumps.
func (m Model) renderHandBlock(idx int, h engine.HandView, multi, roundOver bool, count int) string {
	// No "active" caption: the highlighted (gold) border already marks the active
	// hand. Multi-hand rounds keep a "Hand N" label to identify the spots.
	var caption string
	if multi {
		caption = fmt.Sprintf("Hand %d", idx+1)
	}

	// Reserve a fixed-height card row so a hand block keeps its height as its cards
	// cascade in (blank until the first card lands), preventing vertical jumping.
	cards := lipgloss.NewStyle().Height(cardHeight).Render(renderHandCount(h.Cards, count, m.compact()))

	// Show the hand total only once every card in this hand is on the table, so
	// the deal cascade does not spoil a not-yet-complete total.
	showValue := count > 0 && count >= len(h.Cards)
	footer := m.renderHandFooter(h, roundOver, showValue)

	inner := cards
	if caption != "" {
		inner = caption + "\n" + cards
	}
	if footer != "" {
		inner = inner + "\n" + footer
	}

	if h.Active {
		return activeHandStyle.Render(inner)
	}
	return inactiveHandStyle.Render(inner)
}

func (m Model) renderHandFooter(h engine.HandView, roundOver, showValue bool) string {
	// The value column shows the hand total; the outcome column owns the verdict
	// word. Suppress the "Blackjack!" label here once the round is settled so it
	// isn't printed twice (value slot + outcome slot). During the deal cascade the
	// value is hidden entirely until the hand is complete.
	var parts []string
	if showValue {
		parts = append(parts, describeHandValue(h.Value, h.Soft, h.Blackjack && !roundOver))
	}
	parts = append(parts, betStyle.Render(fmt.Sprintf("$%d", h.Bet)))
	if h.Doubled {
		parts = append(parts, dimStyle.Render("doubled"))
	}
	if h.SplitAce {
		parts = append(parts, dimStyle.Render("split ace"))
	}
	if roundOver {
		parts = append(parts, outcomeText(h.Outcome, h.Net))
	}
	return strings.Join(parts, "  ")
}

// renderHeader shows the player's persistent standing (bankroll) directly under
// the title, left-aligned. There is no status bar: the current wager lives in the
// betting control and per-hand footers, and the hand total is shown by the cards
// themselves. Insurance, when taken, is noted here since it has no other home.
func (m Model) renderHeader() string {
	g := m.game
	// The header shows displayBankroll, which the feel tier animates; when idle it
	// equals g.Bankroll().
	s := "Bankroll " + moneyStyle.Render(fmt.Sprintf("$%d", m.displayBankroll))
	if g.TookInsurance() && g.Phase() != engine.PhaseBetting {
		s += "    " + dimStyle.Render(fmt.Sprintf("insurance $%d", g.InsuranceBet()))
	}
	return s
}

// renderActionBar renders the arrow-navigation menu for the current phase with
// the selected item highlighted, plus a small dim navigation hint. During the
// player's turn the menu is driven directly off engine.LegalActions(), so the UI
// can never advertise an action the engine would reject.
func (m Model) renderActionBar() string {
	var title, body, hint string
	switch m.game.Phase() {
	case engine.PhaseBetting:
		title, body = m.renderBettingPanel()
		hint = m.bettingHint()
	case engine.PhaseInsurance:
		spot := m.game.InsuranceSpot()
		handNo := spot + 1
		half := m.game.SpotBet(spot) / 2
		title = fmt.Sprintf("Insurance · Hand %d", handNo)
		body = dimStyle.Render(fmt.Sprintf("Dealer shows an Ace — insure Hand %d for $%d?", handNo, half)) +
			"\n" + renderMenu([]string{"Yes", "No"}, clampIdx(m.cursor, 2))
		hint = "← → choose · enter confirm · q quit"
	case engine.PhasePlayerTurn:
		title = "Your move"
		actions := m.playerActions()
		labels := make([]string, len(actions))
		for i, a := range actions {
			labels[i] = actionLabel(a)
		}
		body = renderMenu(labels, clampIdx(m.cursor, len(actions)))
		hint = "← → choose · enter confirm · q quit"
	case engine.PhaseRoundOver:
		title = "Round over"
		body = renderMenu([]string{"Next hand"}, 0)
		hint = "enter continue · q quit"
	case engine.PhaseGameOver:
		title = "Game over"
		body = renderMenu([]string{"Restart", "Quit"}, clampIdx(m.cursor, 2))
		hint = "← → choose · enter confirm · q quit"
	}
	return titledBox(title, body) + "\n " + dimStyle.Render(hint)
}

// renderBettingPanel builds the betting panel's title and body. One opened hand
// keeps the classic single gold ◀ $N ▶ control; two or more render a row of
// per-hand bet tiles plus a staked/after-deal total line.
func (m Model) renderBettingPanel() (title, body string) {
	n := m.game.NumSpots()
	if n <= 1 {
		return "Place your bet", betStyle.Render(fmt.Sprintf("◀ $%d ▶", m.game.SpotBet(0)))
	}
	total := 0
	for i := 0; i < n; i++ {
		total += m.game.SpotBet(i)
	}
	after := m.game.Bankroll() - total
	totalLine := dimStyle.Render(fmt.Sprintf("Total staked $%d · after deal $%d", total, after))
	return "Place your bets", m.renderBetTiles() + "\n" + totalLine
}

// renderBetTiles lays out one bet tile per opened hand. The focused tile is
// gold-bordered and shows ◀ $N ▶; the rest sit in a plain border showing $N.
func (m Model) renderBetTiles() string {
	n := m.game.NumSpots()
	tiles := make([]string, n)
	for i := 0; i < n; i++ {
		label := fmt.Sprintf("Hand %d", i+1)
		var value string
		if i == m.focusedSpot {
			value = betStyle.Render(fmt.Sprintf("◀ $%d ▶", m.game.SpotBet(i)))
		} else {
			value = fmt.Sprintf("$%d", m.game.SpotBet(i))
		}
		w := lipgloss.Width(label)
		if vw := lipgloss.Width(value); vw > w {
			w = vw
		}
		inner := center(label, w) + "\n" + center(value, w)
		if i == m.focusedSpot {
			tiles[i] = focusedTileStyle.Render(inner)
		} else {
			tiles[i] = plainTileStyle.Render(inner)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tiles...)
}

// bettingHint builds the contextual key hint for the betting screen: switch/remove
// appear only with 2+ hands, and add is hidden once the spot cap is reached.
func (m Model) bettingHint() string {
	n := m.game.NumSpots()
	parts := []string{
		fmt.Sprintf("← → bet ±$%d", engine.BetIncrement),
		fmt.Sprintf("shift ±$%d", coarseBetStep),
	}
	if n > 1 {
		parts = append(parts, "tab ⇄ switch")
	}
	if n < engine.MaxSpots {
		parts = append(parts, "a add hand")
	}
	if n > 1 {
		parts = append(parts, "x remove")
	}
	parts = append(parts, "enter deal", "q quit")
	return strings.Join(parts, " · ")
}

// titledBox draws a rounded panel around content with a label embedded in the top
// border. Content may contain ANSI styling and span multiple lines; widths are
// measured with lipgloss.Width so styled menu pills still align.
func titledBox(title, content string) string {
	lines := strings.Split(content, "\n")
	inner := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > inner {
			inner = w
		}
	}
	titleW := lipgloss.Width(title)
	if need := titleW + 2; need > inner { // keep the title from overflowing the top border
		inner = need
	}
	interior := inner + 2 // one space of padding on each side
	dashes := interior - (titleW + 3)
	if dashes < 0 {
		dashes = 0
	}

	var sb strings.Builder
	sb.WriteString(panelBorderStyle.Render("╭─ ") + panelTitleStyle.Render(title) +
		panelBorderStyle.Render(" "+strings.Repeat("─", dashes)+"╮"))
	sb.WriteString("\n")
	for _, l := range lines {
		pad := inner - lipgloss.Width(l)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(panelBorderStyle.Render("│ ") + l + strings.Repeat(" ", pad) +
			panelBorderStyle.Render(" │") + "\n")
	}
	sb.WriteString(panelBorderStyle.Render("╰" + strings.Repeat("─", interior) + "╯"))
	return sb.String()
}

// renderMenu lays a set of labels out horizontally, highlighting the selected one
// in gold.
func renderMenu(labels []string, selected int) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		if i == selected {
			parts[i] = menuSelectedStyle.Render(l)
		} else {
			parts[i] = menuUnselectedStyle.Render(l)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// actionLabel maps a legal engine action to its menu label.
func actionLabel(a engine.Action) string {
	switch a {
	case engine.ActionHit:
		return "Hit"
	case engine.ActionStand:
		return "Stand"
	case engine.ActionDouble:
		return "Double"
	case engine.ActionSplit:
		return "Split"
	default:
		return a.String()
	}
}

// describeHandValue formats a value with its soft/hard qualifier, or a special
// word for a natural or a bust. Purely presentational; the numbers come from
// the engine.
func describeHandValue(value int, soft, blackjack bool) string {
	switch {
	case blackjack:
		return blackjackStyle.Render("Blackjack!")
	case value > 21:
		return loseStyle.Render(fmt.Sprintf("bust (%d)", value))
	case soft:
		return fmt.Sprintf("soft %d", value)
	default:
		return fmt.Sprintf("hard %d", value)
	}
}

// outcomeText renders a settled per-hand outcome as its colored money delta. The
// win/lose wording is dropped — the +/- and color convey direction, and the result
// banner already names the outcome. Push has no delta, so it keeps its label.
func outcomeText(o engine.Outcome, net int) string {
	switch o {
	case engine.OutcomeBlackjack, engine.OutcomeWin:
		return winStyle.Render(fmt.Sprintf("+$%d", net))
	case engine.OutcomePush:
		return pushStyle.Render("Push")
	case engine.OutcomeLose:
		return loseStyle.Render(fmt.Sprintf("-$%d", -net))
	default:
		return ""
	}
}
