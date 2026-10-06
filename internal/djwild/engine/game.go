package engine

import (
	"errors"
	"math/rand/v2"
)

// Engine-wide constants.
const (
	// StartingBankroll is the player's starting money.
	StartingBankroll = 1000
	// MinBet is the table minimum: the smallest legal Ante (the Blind always
	// equals the Ante), and the smallest non-zero Trips bet.
	MinBet = 3
	// HandSize is the number of cards the player and the dealer each receive.
	HandSize = 5
	// PlayMultiple is the Play wager as a multiple of the Ante.
	PlayMultiple = 2
)

// pcgStream is a fixed second PCG parameter so a single int64 seed fully
// determines the RNG stream (owned, injected RNG; no wall-clock dependence).
const pcgStream = 0x9E3779B97F4A7C15

// Phase is the current state of the round's state machine.
type Phase int

const (
	// PhaseBetting: the player sets the Ante (the Blind matches it) and the
	// optional Trips side bet, then calls Deal.
	PhaseBetting Phase = iota
	// PhaseDecision: five cards each are dealt; the player Folds or Plays 2×
	// the Ante.
	PhaseDecision
	// PhaseRoundOver: the hand is settled and the dealer is revealed; results
	// are available via Result. Call NextHand to continue.
	PhaseRoundOver
	// PhaseGameOver: the bankroll cannot cover the minimum Ante plus Blind.
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
	Bet        int     // amount staked on this component (0 if not placed)
	Outcome    Outcome // win / push / loss / none
	Net        int     // net bankroll change: profit, 0 on push, or -Bet on loss
	Multiplier int     // profit-per-unit paid on a pay-table win (Blind, Trips)
}

// Settlement is the full, component-by-component breakdown of a settled hand.
// Net is the total bankroll change and equals the sum of every component's Net.
type Settlement struct {
	Played     bool // the player made the Play wager
	Folded     bool // the player folded (forfeiting the Ante and Blind)
	Comparison int  // >0 player's hand higher, <0 dealer's, 0 tie (valid when Played)

	Ante  ComponentResult
	Blind ComponentResult
	Play  ComponentResult
	Trips ComponentResult

	Net int
}

// Game holds all mutable state for one DJ Wild game. Nothing is stored at
// package scope, so many Games can run concurrently in one process.
type Game struct {
	rng  *rand.Rand
	deck *Deck

	bankroll int
	phase    Phase

	// Pending wagers for the current/next hand. The Blind always equals ante.
	ante    int
	trips   int
	playBet int // posted when the player Plays (2× the Ante)

	// Dealt round state.
	playerCards [HandSize]Card
	dealerCards [HandSize]Card
	playerValue HandValue
	dealerValue HandValue
	dealt       bool

	dealerRevealed bool
	settlement     Settlement
}

// NewGame constructs a game with a time-independent, injected RNG seed. The
// Ante (and Blind) default to MinBet with no Trips.
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
// Deal order: the first five cards go to the player, the next five to the
// dealer. Provide at least ten cards.
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

// Blind returns the pending Blind wager, which always equals the Ante.
func (g *Game) Blind() int { return g.ante }

// Trips returns the pending Trips side bet.
func (g *Game) Trips() int { return g.trips }

// PlayBet returns the posted Play wager (0 until the player Plays).
func (g *Game) PlayBet() int { return g.playBet }

// CardsRemaining returns how many cards are left in the deck.
func (g *Game) CardsRemaining() int { return g.deck.Remaining() }

// HandView is a read-only snapshot of a five-card hand for the UI.
type HandView struct {
	Cards []Card
	Value HandValue
	Name  string // e.g. "Full House", "Five Aces", "Pair of Kings"
}

// Player returns a snapshot of the player's hand. Empty before the first Deal.
func (g *Game) Player() HandView {
	if !g.dealt {
		return HandView{}
	}
	return HandView{
		Cards: append([]Card(nil), g.playerCards[:]...),
		Value: g.playerValue,
		Name:  g.playerValue.Name(),
	}
}

// DealerView is a read-only snapshot of the dealer hand. Cards and the
// evaluated fields are only meaningful once Revealed is true.
type DealerView struct {
	HandView
	Revealed bool
}

// Dealer returns a snapshot of the dealer hand. The cards stay hidden until the
// hand is revealed at showdown.
func (g *Game) Dealer() DealerView {
	if !g.dealt || !g.dealerRevealed {
		return DealerView{}
	}
	return DealerView{
		HandView: HandView{
			Cards: append([]Card(nil), g.dealerCards[:]...),
			Value: g.dealerValue,
			Name:  g.dealerValue.Name(),
		},
		Revealed: true,
	}
}

// DealerRevealed reports whether the dealer's hand is face up.
func (g *Game) DealerRevealed() bool { return g.dealerRevealed }

// Result returns the settlement for the most recently settled hand.
func (g *Game) Result() Settlement { return g.settlement }

// ---- Betting ----

// SetAnte sets the pending Ante; the Blind is posted alongside it for the same
// amount. The Ante must be at least MinBet, and Ante + Blind + Trips must fit
// the bankroll.
func (g *Game) SetAnte(amount int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if amount < MinBet || 2*amount+g.trips > g.bankroll {
		return ErrInvalidBet
	}
	g.ante = amount
	return nil
}

// SetTrips sets the pending Trips side bet (0 for none).
func (g *Game) SetTrips(amount int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if amount < 0 || (amount != 0 && amount < MinBet) || 2*g.ante+amount > g.bankroll {
		return ErrInvalidBet
	}
	g.trips = amount
	return nil
}

// ---- Deal ----

// Deal starts a hand: it validates and escrows the Ante, Blind and Trips,
// shuffles a fresh deck, and deals five cards to the player and five to the
// dealer.
func (g *Game) Deal() error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if g.ante < MinBet {
		return ErrInvalidBet
	}
	escrow := 2*g.ante + g.trips
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
	g.playerValue = Evaluate(g.playerCards)
	g.dealerValue = Evaluate(g.dealerCards)
	g.dealt = true
	g.phase = PhaseDecision
	return nil
}

// ---- Decision ----

// CanPlay reports whether the player may Play: the phase is Decision and the
// bankroll covers the Play wager (twice the Ante).
func (g *Game) CanPlay() bool {
	return g.phase == PhaseDecision && g.bankroll >= PlayMultiple*g.ante
}

// Play posts the Play wager (2× the Ante) and settles the hand against the
// dealer. The dealer always plays — there is no qualifying hand.
func (g *Game) Play() error {
	if g.phase != PhaseDecision {
		return ErrWrongPhase
	}
	if !g.CanPlay() {
		return ErrIllegalAction
	}
	g.playBet = PlayMultiple * g.ante
	g.bankroll -= g.playBet
	g.settle(true)
	return nil
}

// Fold forfeits the Ante and Blind and ends the hand. Trips still resolves on
// the player's cards.
func (g *Game) Fold() error {
	if g.phase != PhaseDecision {
		return ErrWrongPhase
	}
	g.settle(false)
	return nil
}

// ---- Settlement ----

// settle resolves every wager, applies all returns to the bankroll atomically,
// reveals the dealer, and moves to PhaseRoundOver.
func (g *Game) settle(played bool) {
	s := Settlement{Played: played, Folded: !played}
	returns := 0

	s.Ante.Bet, s.Blind.Bet = g.ante, g.ante
	if played {
		s.Play.Bet = g.playBet
		s.Comparison = Compare(g.playerValue, g.dealerValue)
	}
	switch {
	case !played:
		s.Ante.Outcome, s.Ante.Net = OutcomeLoss, -g.ante
		s.Blind.Outcome, s.Blind.Net = OutcomeLoss, -g.ante
	case s.Comparison > 0:
		// Player wins: Ante and Play pay even money; the Blind pays its table on
		// a Straight or better and pushes otherwise.
		s.Ante.Outcome, s.Ante.Net = OutcomeWin, g.ante
		s.Play.Outcome, s.Play.Net = OutcomeWin, g.playBet
		returns += 2*g.ante + 2*g.playBet
		if m, ok := BlindPayout(g.playerValue.Category); ok {
			s.Blind.Outcome, s.Blind.Net, s.Blind.Multiplier = OutcomeWin, g.ante*m, m
			returns += g.ante + g.ante*m
		} else {
			s.Blind.Outcome = OutcomePush
			returns += g.ante
		}
	case s.Comparison < 0:
		s.Ante.Outcome, s.Ante.Net = OutcomeLoss, -g.ante
		s.Blind.Outcome, s.Blind.Net = OutcomeLoss, -g.ante
		s.Play.Outcome, s.Play.Net = OutcomeLoss, -g.playBet
	default:
		s.Ante.Outcome, s.Blind.Outcome, s.Play.Outcome = OutcomePush, OutcomePush, OutcomePush
		returns += 2*g.ante + g.playBet
	}

	if g.trips > 0 {
		s.Trips.Bet = g.trips
		if m, ok := TripsPayout(g.playerValue); ok {
			s.Trips.Outcome, s.Trips.Net, s.Trips.Multiplier = OutcomeWin, g.trips*m, m
			returns += g.trips + g.trips*m
		} else {
			s.Trips.Outcome, s.Trips.Net = OutcomeLoss, -g.trips
		}
	}

	s.Net = s.Ante.Net + s.Blind.Net + s.Play.Net + s.Trips.Net

	g.bankroll += returns
	g.settlement = s
	g.dealerRevealed = true
	g.phase = PhaseRoundOver
}

// ---- Next hand ----

// NextHand advances from a settled hand to the next bet, or to game over when
// the bankroll cannot cover the minimum Ante plus Blind. The previous wagers
// carry over, clamped to what the bankroll still affords (Trips reduced first).
func (g *Game) NextHand() error {
	if g.phase != PhaseRoundOver {
		return ErrWrongPhase
	}
	if g.bankroll < 2*MinBet {
		g.phase = PhaseGameOver
		return nil
	}

	ante := g.ante
	if ante < MinBet {
		ante = MinBet
	}
	if 2*ante > g.bankroll {
		ante = g.bankroll / 2
	}
	trips := g.trips
	if room := g.bankroll - 2*ante; trips > room {
		trips = room
	}
	if trips < MinBet {
		trips = 0
	}

	g.ante, g.trips = ante, trips
	g.playBet = 0
	g.dealt = false
	g.dealerRevealed = false
	g.phase = PhaseBetting
	return nil
}
