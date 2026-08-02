package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/32blackjack/internal/engine"
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

	// msg is a transient status line (e.g. rejected bet, reshuffle notice).
	msg string
}

// New builds a Model over an already-constructed game. newGame is invoked to get
// a fresh, freshly-seeded game when the player restarts after game over; keeping
// the time seed in the caller preserves the engine's injected-RNG contract.
func New(game *engine.Game, newGame func() *engine.Game) Model {
	return Model{game: game, newGame: newGame}
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
	return m, nil
}

func (m Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()

	// Global quit.
	if key == "q" || key == "ctrl+c" {
		return m, tea.Quit
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
	switch key {
	case "right", "up":
		m.adjustBet(engine.BetIncrement)
	case "left", "down":
		m.adjustBet(-engine.BetIncrement)
	case "shift+right", "shift+up", "pgup":
		m.adjustBet(coarseBetStep)
	case "shift+left", "shift+down", "pgdown":
		m.adjustBet(-coarseBetStep)
	case "enter", " ":
		if err := m.game.Deal(); err != nil {
			m.msg = "cannot deal: " + err.Error()
		} else {
			m.msg = ""
			if m.game.DidReshuffle() {
				m.msg = "Shoe reshuffled."
			}
		}
	}
	return m, nil
}

// adjustBet moves the pending bet by delta (a BetIncrement step), clamped to
// [MinBet, bankroll] and snapped to a BetIncrement multiple, then pushes it
// through the engine.
func (m *Model) adjustBet(delta int) {
	target := m.game.Bet() + delta
	if target < engine.MinBet {
		target = engine.MinBet
	}
	if bank := m.game.Bankroll(); target > bank {
		target = (bank / engine.BetIncrement) * engine.BetIncrement
	}
	if target%engine.BetIncrement != 0 {
		target = (target / engine.BetIncrement) * engine.BetIncrement
	}
	if err := m.game.SetBet(target); err != nil {
		m.msg = "bet: " + err.Error()
		return
	}
	m.msg = ""
}

func (m Model) handleInsurance(key string) (tea.Model, tea.Cmd) {
	// Two-option yes/no menu (0 = Yes, 1 = No). Not sourced from LegalActions.
	m = m.syncCursor("insurance", 2)
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
		_ = m.game.Insurance(m.cursor == 0)
		m.msg = ""
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
		} else {
			m.msg = ""
		}
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
	if len(m.game.Player()) > 2 {
		return true
	}
	return m.width > 0 && m.width < compactWidthThreshold
}

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("3:2 Blackjack"))
	b.WriteString("\n\n")
	b.WriteString(m.renderDealerArea())
	b.WriteString("\n\n")
	b.WriteString(m.renderPlayerArea())
	b.WriteString("\n\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")
	b.WriteString(m.renderActionBar())
	b.WriteString("\n")

	out := b.String()
	if m.width > 0 {
		out = lipgloss.NewStyle().Width(m.width).Render(out)
	}
	return out
}

func (m Model) renderDealerArea() string {
	dv := m.game.Dealer()
	label := areaLabelStyle.Render("Dealer")

	var status string
	switch {
	case len(dv.Cards) == 0:
		status = dimStyle.Render("waiting for deal")
	case !dv.Revealed:
		status = dimStyle.Render(fmt.Sprintf("showing %d", dv.Value))
	default:
		status = describeHandValue(dv.Value, dv.Soft, dv.Blackjack)
	}

	if len(dv.Cards) == 0 {
		return label + "  " + status
	}
	cards := renderDealerHand(dv, m.compact())
	return label + "  " + status + "\n" + cards
}

func (m Model) renderPlayerArea() string {
	label := areaLabelStyle.Render("You")
	hands := m.game.Player()
	if len(hands) == 0 {
		return label + "  " + dimStyle.Render("place your bet")
	}

	roundOver := m.game.Phase() == engine.PhaseRoundOver
	blocks := make([]string, len(hands))
	for i, h := range hands {
		blocks[i] = m.renderHandBlock(i, h, len(hands) > 1, roundOver)
	}
	handsRow := lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
	return label + "\n" + handsRow
}

// renderHandBlock renders one player hand: a caption, the cards, and a footer
// with value/bet (and outcome once the round is over). The active hand gets a
// highlighted border; others get an equal-size invisible border so nothing jumps.
func (m Model) renderHandBlock(idx int, h engine.HandView, multi, roundOver bool) string {
	var caption string
	if multi {
		caption = fmt.Sprintf("Hand %d", idx+1)
		if h.Active {
			caption += " " + keyHintStyle.Render("◀ active")
		}
	} else if h.Active {
		caption = keyHintStyle.Render("◀ active")
	}

	cards := renderHand(h.Cards, m.compact())

	footer := m.renderHandFooter(h, roundOver)

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

func (m Model) renderHandFooter(h engine.HandView, roundOver bool) string {
	// The value column shows the hand total; the outcome column owns the verdict
	// word. Suppress the "Blackjack!" label here once the round is settled so it
	// isn't printed twice (value slot + outcome slot).
	parts := []string{describeHandValue(h.Value, h.Soft, h.Blackjack && !roundOver)}
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

func (m Model) renderStatusBar() string {
	g := m.game
	segs := []string{
		"Bankroll " + moneyStyle.Render(fmt.Sprintf("$%d", g.Bankroll())),
		"Bet " + betStyle.Render(fmt.Sprintf("$%d", g.Bet())),
	}

	if g.Phase() == engine.PhasePlayerTurn {
		hands := g.Player()
		if i := g.ActiveHandIndex(); i >= 0 && i < len(hands) {
			h := hands[i]
			segs = append(segs, "Hand "+describeHandValue(h.Value, h.Soft, h.Blackjack))
		}
	}

	if g.TookInsurance() {
		segs = append(segs, dimStyle.Render(fmt.Sprintf("insurance $%d", g.InsuranceBet())))
	}

	line := statusBarStyle.Render(strings.Join(segs, "   "))
	if m.msg != "" {
		line += "\n" + dimStyle.Render(m.msg)
	}
	return line
}

// renderActionBar renders the arrow-navigation menu for the current phase with
// the selected item highlighted, plus a small dim navigation hint. During the
// player's turn the menu is driven directly off engine.LegalActions(), so the UI
// can never advertise an action the engine would reject.
func (m Model) renderActionBar() string {
	var menu, hint string
	switch m.game.Phase() {
	case engine.PhaseBetting:
		menu = betStyle.Render(fmt.Sprintf("◀ $%d ▶", m.game.Bet()))
		hint = fmt.Sprintf("← → $%d · shift+← → $%d · enter deal · q quit", engine.BetIncrement, coarseBetStep)
	case engine.PhaseInsurance:
		half := m.game.Bet() / 2
		prompt := dimStyle.Render(fmt.Sprintf("Dealer shows an Ace. Insurance $%d?  ", half))
		menu = prompt + renderMenu([]string{"Yes", "No"}, clampIdx(m.cursor, 2))
		hint = "← → choose · enter confirm · q quit"
	case engine.PhasePlayerTurn:
		actions := m.playerActions()
		labels := make([]string, len(actions))
		for i, a := range actions {
			labels[i] = actionLabel(a)
		}
		menu = renderMenu(labels, clampIdx(m.cursor, len(actions)))
		hint = "← → choose · enter confirm · q quit"
	case engine.PhaseRoundOver:
		menu = renderMenu([]string{"Next hand"}, 0)
		hint = "enter continue · q quit"
	case engine.PhaseGameOver:
		menu = renderMenu([]string{"Restart", "Quit"}, clampIdx(m.cursor, 2))
		hint = "← → choose · enter confirm · q quit"
	}
	return actionBarStyle.Render(menu + "\n" + dimStyle.Render(hint))
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

// outcomeText renders a settled per-hand outcome with its money delta.
func outcomeText(o engine.Outcome, net int) string {
	switch o {
	case engine.OutcomeBlackjack:
		return blackjackStyle.Render(fmt.Sprintf("Blackjack! +$%d", net))
	case engine.OutcomeWin:
		return winStyle.Render(fmt.Sprintf("Win +$%d", net))
	case engine.OutcomePush:
		return pushStyle.Render("Push")
	case engine.OutcomeLose:
		return loseStyle.Render(fmt.Sprintf("Lose -$%d", -net))
	default:
		return ""
	}
}
