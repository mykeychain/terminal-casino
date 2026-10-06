package engine

import (
	"errors"
	"math/rand/v2"
)

// Engine-wide constants.
const (
	// StartingBankroll is the player's starting money.
	StartingBankroll = 1000
	// MinBet is the table minimum: the smallest legal Ante, and the smallest
	// non-zero side bet.
	MinBet = 3
	// HandSize is the number of cards the player and the dealer each receive.
	HandSize = 7
)

// pcgStream is a fixed second PCG parameter so a single int64 seed fully
// determines the RNG stream (owned, injected RNG; no wall-clock dependence).
const pcgStream = 0x9E3779B97F4A7C15

// Phase is the current state of the round's state machine.
type Phase int

const (
	// PhaseBetting: the player sets the Ante and the optional Flush Rush and
	// Super Flush Rush side bets, then calls Deal.
	PhaseBetting Phase = iota
	// PhaseDecision: seven cards each are dealt; the player Folds or Plays 1×,
	// 2× or 3× the Ante (the ceiling set by the length of their flush).
	PhaseDecision
	// PhaseRoundOver: the hand is settled and the dealer is revealed; results
	// are available via Result. Call NextHand to continue.
	PhaseRoundOver
	// PhaseGameOver: the bankroll is below the minimum bet; play cannot continue.
	PhaseGameOver
)

// String returns the phase name.
func (p Phase) String() string {
	switch p {
	case PhaseBetting:
		return "betting"
	case PhaseDecision:
		return "decision"
	case PhaseRoundOver:
		return "round-over"
	case PhaseGameOver:
		return "game-over"
	default:
		return "unknown"
	}
}

// Outcome is the settled result of a single wager component.
type Outcome int

const (
	OutcomeNone Outcome = iota // the component was not in play
	OutcomeWin                 // the component won (Net is the profit)
	OutcomePush                // the component pushed (stake returned, Net 0)
	OutcomeLoss                // the component lost (Net is the negative stake)
)

// String returns the outcome name.
func (o Outcome) String() string {
	switch o {
	case OutcomeWin:
		return "win"
	case OutcomePush:
		return "push"
	case OutcomeLoss:
		return "loss"
	default:
		return "none"
	}
}

// Common errors.
var (
	ErrWrongPhase    = errors.New("action not allowed in current phase")
	ErrIllegalAction = errors.New("action is not currently legal")
	ErrInvalidBet    = errors.New("invalid bet amount")
)

// ComponentResult is the settled result of one wager.
type ComponentResult struct {
	Bet     int     // amount staked on this component (0 if not placed)
	Outcome Outcome // win / push / loss / none
	Net     int     // net bankroll change: profit, 0 on push, or -Bet on loss
}

// SideResult is a settled side bet together with the hand feature it paid on.
type SideResult struct {
	ComponentResult
	Length     int // flush length (Flush Rush) or straight-flush run (Super Flush Rush)
	Multiplier int // profit-per-unit paid (0 unless a win)
}

// Settlement is the full, component-by-component breakdown of a settled hand.
// Net is the total bankroll change and equals the sum of every component's Net.
type Settlement struct {
	DealerQualified bool // dealer held a 3-card 9-high flush or better
	Played          bool // the player made a Play wager
	Folded          bool // the player folded (forfeiting the Ante)
	PlayMultiple    int  // the Play wager as a multiple of the Ante (0 on a fold)
	Comparison      int  // >0 player's flush higher, <0 dealer's, 0 tie (valid when Played)

	Ante           ComponentResult
	Play           ComponentResult
	FlushRush      SideResult
	SuperFlushRush SideResult

	Net int
}

// Game holds all mutable state for one I Luv Suits game. Nothing is stored at
// package scope, so many Games can run concurrently in one process.
type Game struct {
	rng  *rand.Rand
	deck *Deck

	bankroll int
	phase    Phase

	// Pending wagers for the current/next hand.
	ante           int
	flushRush      int
	superFlushRush int
	playBet        int // posted when the player Plays (1×–3× the Ante)

	// Dealt round state.
	playerCards [HandSize]Card
	dealerCards [HandSize]Card
	playerFlush FlushHand
	dealerFlush FlushHand
	dealt       bool

	dealerRevealed bool
	settlement     Settlement
}

// NewGame constructs a game with a time-independent, injected RNG seed. The
// Ante defaults to MinBet with no side bets.
func NewGame(seed int64) *Game {
	rng := rand.New(rand.NewPCG(uint64(seed), pcgStream))
	return &Game{
		rng:      rng,
		deck:     NewDeck(rng),
		bankroll: StartingBankroll,
		phase:    PhaseBetting,
		ante:     MinBet,
	}
}

// NewGameWithDeck constructs a game with a pre-stacked deck and a chosen
// starting bankroll, for deterministic tests. The deck is never shuffled.
//
// Deal order: the first seven cards go to the player, the next seven to the
// dealer. Provide at least fourteen cards.
func NewGameWithDeck(bankroll int, cards []Card) *Game {
	return &Game{
		deck:     NewStackedDeck(cards),
		bankroll: bankroll,
		phase:    PhaseBetting,
		ante:     MinBet,
	}
}

// ---- Accessors (read-only views for the UI) ----

// Phase returns the current phase.
func (g *Game) Phase() Phase { return g.phase }

// Bankroll returns the current (post-escrow) bankroll.
func (g *Game) Bankroll() int { return g.bankroll }

// Ante returns the pending Ante wager.
func (g *Game) Ante() int { return g.ante }

// FlushRush returns the pending Flush Rush side bet.
func (g *Game) FlushRush() int { return g.flushRush }

// SuperFlushRush returns the pending Super Flush Rush side bet.
func (g *Game) SuperFlushRush() int { return g.superFlushRush }

// PlayBet returns the posted Play wager (0 until the player Plays).
func (g *Game) PlayBet() int { return g.playBet }

// CardsRemaining returns how many cards are left in the deck.
func (g *Game) CardsRemaining() int { return g.deck.Remaining() }

// HandView is a read-only snapshot of a seven-card hand for the UI. Cards are
// grouped by suit with the best flush first (see SortForDisplay).
type HandView struct {
	Cards []Card
	Flush FlushHand
	Name  string // e.g. "5-card ♥ flush, King-high"
}

// Player returns a snapshot of the player's hand. Empty before the first Deal.
func (g *Game) Player() HandView {
	if !g.dealt {
		return HandView{}
	}
	return HandView{
		Cards: SortForDisplay(g.playerCards[:]),
		Flush: g.playerFlush,
		Name:  g.playerFlush.Name(),
	}
}

// DealerView is a read-only snapshot of the dealer hand. Cards and the
// evaluated fields are only meaningful once Revealed is true.
type DealerView struct {
	HandView
	Revealed  bool
	Qualified bool // dealer qualifies; valid when Revealed
}

// Dealer returns a snapshot of the dealer hand. The cards stay hidden until the
// hand is revealed at showdown.
func (g *Game) Dealer() DealerView {
	if !g.dealt || !g.dealerRevealed {
		return DealerView{}
	}
	return DealerView{
		HandView: HandView{
			Cards: SortForDisplay(g.dealerCards[:]),
			Flush: g.dealerFlush,
			Name:  g.dealerFlush.Name(),
		},
		Revealed:  true,
		Qualified: Qualifies(g.dealerFlush),
	}
}

// DealerRevealed reports whether the dealer's hand is face up.
func (g *Game) DealerRevealed() bool { return g.dealerRevealed }

// Result returns the settlement for the most recently settled hand.
func (g *Game) Result() Settlement { return g.settlement }

// ---- Betting ----

// SetAnte sets the pending Ante. The Ante is mandatory: it must be at least
// MinBet, and the Ante plus both side bets must not exceed the bankroll.
func (g *Game) SetAnte(amount int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if amount < MinBet || amount+g.flushRush+g.superFlushRush > g.bankroll {
		return ErrInvalidBet
	}
	g.ante = amount
	return nil
}

// SetFlushRush sets the pending Flush Rush side bet (0 for none).
func (g *Game) SetFlushRush(amount int) error {
	if err := g.checkSide(amount, g.ante+g.superFlushRush); err != nil {
		return err
	}
	g.flushRush = amount
	return nil
}

// SetSuperFlushRush sets the pending Super Flush Rush side bet (0 for none).
func (g *Game) SetSuperFlushRush(amount int) error {
	if err := g.checkSide(amount, g.ante+g.flushRush); err != nil {
		return err
	}
	g.superFlushRush = amount
	return nil
}

// checkSide validates a side bet amount given the other wagers' total.
func (g *Game) checkSide(amount, others int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if amount < 0 || (amount != 0 && amount < MinBet) || amount+others > g.bankroll {
		return ErrInvalidBet
	}
	return nil
}

// ---- Deal ----

// Deal starts a hand: it validates and escrows the wagers, shuffles a fresh
// deck, and deals seven cards to the player and seven to the dealer.
func (g *Game) Deal() error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if g.ante < MinBet {
		return ErrInvalidBet
	}
	escrow := g.ante + g.flushRush + g.superFlushRush
	if escrow > g.bankroll {
		return ErrInvalidBet
	}

	g.playBet = 0
	g.settlement = Settlement{}
	g.dealerRevealed = false

	g.deck.Shuffle()
	g.bankroll -= escrow

	for i := range g.playerCards {
		g.playerCards[i] = g.deck.draw()
	}
	for i := range g.dealerCards {
		g.dealerCards[i] = g.deck.draw()
	}
	g.playerFlush = Evaluate(g.playerCards[:])
	g.dealerFlush = Evaluate(g.dealerCards[:])
	g.dealt = true
	g.phase = PhaseDecision
	return nil
}

// ---- Decision ----

// MaxPlay returns the largest Play multiple the player's flush permits (1–3),
// before affordability. It is 0 outside the decision phase.
func (g *Game) MaxPlay() int {
	if g.phase != PhaseDecision {
		return 0
	}
	return MaxPlayMultiple(g.playerFlush.Len())
}

// CanPlay reports whether a Play wager of mult times the Ante is legal now: the
// multiple is within the flush-length ceiling and the bankroll covers it.
func (g *Game) CanPlay(mult int) bool {
	return mult >= 1 && mult <= g.MaxPlay() && mult*g.ante <= g.bankroll
}

// LegalPlays returns the legal Play multiples in ascending order. It is empty
// when the bankroll cannot cover even 1× the Ante (only Fold is legal).
func (g *Game) LegalPlays() []int {
	var out []int
	for m := 1; m <= 3; m++ {
		if g.CanPlay(m) {
			out = append(out, m)
		}
	}
	return out
}

// Play posts a Play wager of mult times the Ante and settles the hand against
// the dealer.
func (g *Game) Play(mult int) error {
	if g.phase != PhaseDecision {
		return ErrWrongPhase
	}
	if !g.CanPlay(mult) {
		return ErrIllegalAction
	}
	g.playBet = mult * g.ante
	g.bankroll -= g.playBet
	g.settle(mult)
	return nil
}

// Fold forfeits the Ante and ends the hand. The side bets still resolve on the
// player's cards.
func (g *Game) Fold() error {
	if g.phase != PhaseDecision {
		return ErrWrongPhase
	}
	g.settle(0)
	return nil
}

// ---- Settlement ----

// settle resolves every wager, applies all returns to the bankroll atomically,
// reveals the dealer, and moves to PhaseRoundOver. mult is the Play multiple, or
// 0 for a fold.
func (g *Game) settle(mult int) {
	played := mult > 0
	qualified := Qualifies(g.dealerFlush)
	s := Settlement{DealerQualified: qualified, Played: played, Folded: !played, PlayMultiple: mult}
	returns := 0

	s.Ante.Bet = g.ante
	if played {
		s.Play.Bet = g.playBet
		s.Comparison = Compare(g.playerFlush, g.dealerFlush)
	}
	switch {
	case !played:
		s.Ante.Outcome, s.Ante.Net = OutcomeLoss, -g.ante
	case !qualified:
		// Dealer does not qualify: the Ante pays even money, the Play pushes.
		s.Ante.Outcome, s.Ante.Net = OutcomeWin, g.ante
		s.Play.Outcome = OutcomePush
		returns += 2*g.ante + g.playBet
	case s.Comparison > 0:
		s.Ante.Outcome, s.Ante.Net = OutcomeWin, g.ante
		s.Play.Outcome, s.Play.Net = OutcomeWin, g.playBet
		returns += 2*g.ante + 2*g.playBet
	case s.Comparison < 0:
		s.Ante.Outcome, s.Ante.Net = OutcomeLoss, -g.ante
		s.Play.Outcome, s.Play.Net = OutcomeLoss, -g.playBet
	default:
		s.Ante.Outcome, s.Play.Outcome = OutcomePush, OutcomePush
		returns += g.ante + g.playBet
	}

	var r int
	s.FlushRush, r = resolveSide(g.flushRush, g.playerFlush.Len(), FlushRushPayouts)
	returns += r
	s.SuperFlushRush, r = resolveSide(g.superFlushRush, StraightFlushLen(g.playerCards[:]), SuperFlushRushPayouts)
	returns += r

	s.Net = s.Ante.Net + s.Play.Net + s.FlushRush.Net + s.SuperFlushRush.Net

	g.bankroll += returns
	g.settlement = s
	g.dealerRevealed = true
	g.phase = PhaseRoundOver
}

// resolveSide settles one side bet against its pay table, returning the result
// and the amount to credit back to the bankroll (stake plus profit on a win).
func resolveSide(bet, length int, table map[int]int) (SideResult, int) {
	res := SideResult{Length: length}
	if bet == 0 {
		return res, 0
	}
	res.Bet = bet
	if m, ok := table[length]; ok {
		res.Outcome, res.Multiplier, res.Net = OutcomeWin, m, bet*m
		return res, bet + bet*m
	}
	res.Outcome, res.Net = OutcomeLoss, -bet
	return res, 0
}

// ---- Next hand ----

// NextHand advances from a settled hand to the next bet, or to game over when
// the bankroll can no longer cover the minimum Ante. The previous wagers carry
// over, clamped to what the bankroll still affords (side bets reduced first,
// Super Flush Rush before Flush Rush).
func (g *Game) NextHand() error {
	if g.phase != PhaseRoundOver {
		return ErrWrongPhase
	}
	if g.bankroll < MinBet {
		g.phase = PhaseGameOver
		return nil
	}

	ante := g.ante
	if ante < MinBet {
		ante = MinBet
	}
	if ante > g.bankroll {
		ante = g.bankroll
	}
	fr := clampSide(g.flushRush, g.bankroll-ante)
	sfr := clampSide(g.superFlushRush, g.bankroll-ante-fr)

	g.ante, g.flushRush, g.superFlushRush = ante, fr, sfr
	g.playBet = 0
	g.dealt = false
	g.dealerRevealed = false
	g.phase = PhaseBetting
	return nil
}

// clampSide reduces a side bet to fit room, dropping it when what is left is
// below the minimum.
func clampSide(bet, room int) int {
	if bet > room {
		bet = room
	}
	if bet < MinBet {
		return 0
	}
	return bet
}
