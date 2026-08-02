package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mykeychain/terminal-casino/32blackjack/internal/engine"
)

// betStep is how much a single +/- keypress moves the bet. $25 is the table
// minimum and a standard chip; the engine still enforces BetIncrement/bankroll
// via SetBet, so any residual clamping is safe. (Resolved ambiguity: the spec
// says "adjust by chip denomination"; we use the $25 chip as the step.)
const betStep = engine.MinBet

// compactWidthThreshold: below this terminal width, or with more than two hands,
// cards are packed edge-to-edge so up to four split hands still fit.
const compactWidthThreshold = 80

// Model is the Bubble Tea model wrapping a frozen engine.Game. It renders engine
// state and forwards player intent as engine actions; it computes no game logic.
type Model struct {
	game    *engine.Game
	newGame func() *engine.Game // produces a fresh game for restart (seeded by main)

	width  int
	height int

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

func (m Model) handleBetting(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "+", "=", "up", "right", "k", "l":
		m.adjustBet(betStep)
	case "-", "_", "down", "left", "j":
		m.adjustBet(-betStep)
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

// adjustBet moves the pending bet by delta, clamped to [MinBet, bankroll] and
// snapped to a BetIncrement multiple, then pushes it through the engine.
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
	switch key {
	case "y", "Y":
		_ = m.game.Insurance(true)
		m.msg = ""
	case "n", "N":
		_ = m.game.Insurance(false)
		m.msg = ""
	}
	return m, nil
}

func (m Model) handlePlayerTurn(key string) (tea.Model, tea.Cmd) {
	var (
		action engine.Action
		call   func() error
	)
	switch key {
	case "h":
		action, call = engine.ActionHit, m.game.Hit
	case "s":
		action, call = engine.ActionStand, m.game.Stand
	case "d":
		action, call = engine.ActionDouble, m.game.Double
	case "p":
		action, call = engine.ActionSplit, m.game.Split
	default:
		return m, nil
	}
	// Only invoke when the engine says the action is currently legal.
	if !m.game.CanAct(action) {
		return m, nil
	}
	if err := call(); err != nil {
		m.msg = action.String() + ": " + err.Error()
	} else {
		m.msg = ""
	}
	return m, nil
}

func (m Model) handleRoundOver(key string) (tea.Model, tea.Cmd) {
	if key == "enter" || key == " " || key == "n" {
		if err := m.game.NextHand(); err != nil {
			m.msg = err.Error()
		} else {
			m.msg = ""
		}
	}
	return m, nil
}

func (m Model) handleGameOver(key string) (tea.Model, tea.Cmd) {
	if key == "r" {
		m.game = m.newGame()
		m.msg = ""
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
	parts := []string{describeHandValue(h.Value, h.Soft, h.Blackjack)}
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

// renderActionBar shows only currently-legal key hints. During the player's
// turn it is driven directly off engine.LegalActions() so the UI can never
// advertise an action the engine would reject.
func (m Model) renderActionBar() string {
	var hints []string
	switch m.game.Phase() {
	case engine.PhaseBetting:
		hints = []string{
			hint("+/-", "bet"),
			hint("enter", "deal"),
			hint("q", "quit"),
		}
	case engine.PhaseInsurance:
		half := m.game.Bet() / 2
		hints = []string{
			dimStyle.Render(fmt.Sprintf("Dealer shows an Ace. Insurance $%d?", half)),
			hint("y", "yes"),
			hint("n", "no"),
		}
	case engine.PhasePlayerTurn:
		for _, a := range m.game.LegalActions() {
			if key, label, ok := actionHint(a); ok {
				hints = append(hints, hint(key, label))
			}
		}
		hints = append(hints, hint("q", "quit"))
	case engine.PhaseRoundOver:
		hints = []string{
			hint("enter", "next hand"),
			hint("q", "quit"),
		}
	case engine.PhaseGameOver:
		hints = []string{
			hint("r", "restart"),
			hint("q", "quit"),
		}
	}
	return actionBarStyle.Render(strings.Join(hints, "   "))
}

// actionHint maps a legal engine action to its key binding and label.
func actionHint(a engine.Action) (key, label string, ok bool) {
	switch a {
	case engine.ActionHit:
		return "h", "hit", true
	case engine.ActionStand:
		return "s", "stand", true
	case engine.ActionDouble:
		return "d", "double", true
	case engine.ActionSplit:
		return "p", "split", true
	default:
		return "", "", false
	}
}

func hint(key, label string) string {
	return keyHintStyle.Render("["+key+"]") + " " + label
}

// describeHandValue formats a value with its soft/hard qualifier, or a special
// word for a natural or a bust. Purely presentational; the numbers come from
// the engine.
func describeHandValue(value int, soft, blackjack bool) string {
	switch {
	case blackjack:
		return winStyle.Render("Blackjack!")
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
		return winStyle.Render(fmt.Sprintf("Blackjack! +$%d", net))
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
