package engine

import (
	"errors"
	"math/rand/v2"
)

// Engine-wide constants.
const (
	// StartingBankroll is the player's starting money.
	StartingBankroll = 1000
	// MinBet is the table minimum. Any placed wager must be at least this.
	MinBet = 3
)

// pcgStream is a fixed second PCG parameter so a single int64 seed fully
// determines the RNG stream (owned, injected RNG; no wall-clock dependence).
const pcgStream = 0x9E3779B97F4A7C15

// Phase is the current state of the round's state machine.
type Phase int

const (
	// PhaseBetting: player sets the Ante and/or Pair Plus wagers and calls Deal.
	PhaseBetting Phase = iota
	// PhaseDecision: cards are dealt; the player chooses Play or Fold. Reached
	// only when an Ante was posted. A Pair Plus-only hand skips this phase.
	PhaseDecision
	// PhaseRoundOver: the hand is settled and the dealer is revealed; results are
	// available via Result. Call NextHand to continue.
	PhaseRoundOver
	// PhaseGameOver: bankroll is below the minimum bet; the game cannot continue.
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

// ComponentResult is the settled result of one wager (Ante, Play, or Pair Plus).
type ComponentResult struct {
	Bet     int     // amount staked on this component (0 if not placed)
	Outcome Outcome // win / push / loss / none
	Net     int     // net bankroll change: profit, 0 on push, or -Bet on loss
}

// BonusResult is the Ante Bonus paid on a strong hand (paid on the Ante only
// when the player Played). It is a pure bonus with no separate stake.
type BonusResult struct {
	Applies  bool         // whether any Ante Bonus was earned
	Category HandCategory // the hand category that earned it (valid when Applies)
	Bet      int          // the Ante the bonus was computed from
	Net      int          // profit paid (0 when Applies is false)
}

// Settlement is the full, component-by-component breakdown of a settled hand.
// The UI reads it to message each wager separately. Net is the total bankroll
// change for the hand and equals Ante.Net + Play.Net + AnteBonus.Net +
// PairPlus.Net.
type Settlement struct {
	DealerQualified bool // dealer had Queen-high or better
	Played          bool // the player chose Play
	Folded          bool // the player folded (forfeiting the Ante)

	Ante      ComponentResult
	Play      ComponentResult
	AnteBonus BonusResult
	PairPlus  ComponentResult

	Net int // total net bankroll change for the hand
}

// Game holds all mutable state for one Three-Card Poker game. Nothing is stored
// at package scope, so many Games can run concurrently in one process.
type Game struct {
	rng  *rand.Rand
	deck *Deck

	bankroll int
	phase    Phase

	// Pending wagers for the current/next hand.
	ante     int
	pairPlus int
	playBet  int // posted when the player Plays (equals ante)

	// Dealt round state.
	playerCards [3]Card
	dealerCards [3]Card
	playerValue HandValue
	dealerValue HandValue
	dealt       bool

	dealerRevealed bool
	settlement     Settlement
}

// NewGame constructs a game with a time-independent, injected RNG seed.
// Production supplies a seed at the call site; tests may supply any fixed seed
// for reproducibility. The Ante defaults to MinBet with no Pair Plus.
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
// Deal order: the first three cards go to the player, the next three to the
// dealer. Provide at least six cards.
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

// PairPlus returns the pending Pair Plus wager.
func (g *Game) PairPlus() int { return g.pairPlus }

// PlayBet returns the posted Play wager (0 until the player Plays).
func (g *Game) PlayBet() int { return g.playBet }

// CardsRemaining returns how many cards are left in the deck.
func (g *Game) CardsRemaining() int { return g.deck.Remaining() }

// HandView is a read-only snapshot of a three-card hand for the UI.
type HandView struct {
	Cards []Card
	Value HandValue
	Name  string // e.g. "Pair of Kings", "Straight", "Ace-high"
}

// Player returns a snapshot of the player's dealt hand. Cards are empty before
// the first Deal.
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

// DealerView is a read-only snapshot of the dealer hand for the UI. Cards and
// the evaluated fields are only meaningful once Revealed is true.
type DealerView struct {
	Cards     []Card
	Value     HandValue
	Name      string
	Revealed  bool
	Qualified bool // dealer qualifies (Queen-high or better); valid when Revealed
}

// Dealer returns a snapshot of the dealer hand. The cards stay hidden until the
// hand is revealed at showdown.
func (g *Game) Dealer() DealerView {
	if !g.dealt || !g.dealerRevealed {
		return DealerView{Revealed: false}
	}
	return DealerView{
		Cards:     append([]Card(nil), g.dealerCards[:]...),
		Value:     g.dealerValue,
		Name:      g.dealerValue.Name(),
		Revealed:  true,
		Qualified: Qualifies(g.dealerValue),
	}
}

// DealerRevealed reports whether the dealer's hand is face-up.
func (g *Game) DealerRevealed() bool { return g.dealerRevealed }

// Result returns the settlement breakdown for the most recently settled hand.
// It is meaningful once the phase reaches PhaseRoundOver.
func (g *Game) Result() Settlement { return g.settlement }

// ---- Betting ----

// SetAnte sets the pending Ante. Pass 0 to place no Ante (a Pair Plus-only
// hand). A non-zero amount must be at least MinBet, and the combined Ante and
// Pair Plus escrow must not exceed the bankroll.
func (g *Game) SetAnte(amount int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if amount < 0 || (amount != 0 && amount < MinBet) {
		return ErrInvalidBet
	}
	if amount+g.pairPlus > g.bankroll {
		return ErrInvalidBet
	}
	g.ante = amount
	return nil
}

// SetPairPlus sets the pending Pair Plus wager. Pass 0 to place none. A non-zero
// amount must be at least MinBet, and the combined Ante and Pair Plus escrow
// must not exceed the bankroll.
func (g *Game) SetPairPlus(amount int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if amount < 0 || (amount != 0 && amount < MinBet) {
		return ErrInvalidBet
	}
	if g.ante+amount > g.bankroll {
		return ErrInvalidBet
	}
	g.pairPlus = amount
	return nil
}

// ---- Deal ----

// Deal starts a hand: it validates the wagers, escrows them, shuffles a fresh
// deck, and deals three cards to the player and three to the dealer. At least
// one of Ante or Pair Plus must be posted. When only Pair Plus is posted the
// decision is skipped and the hand settles immediately on the dealt cards.
func (g *Game) Deal() error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if g.ante != 0 && g.ante < MinBet {
		return ErrInvalidBet
	}
	if g.pairPlus != 0 && g.pairPlus < MinBet {
		return ErrInvalidBet
	}
	if g.ante == 0 && g.pairPlus == 0 {
		return ErrInvalidBet
	}
	escrow := g.ante + g.pairPlus
	if escrow > g.bankroll {
		return ErrInvalidBet
	}

	// Reset round state.
	g.playBet = 0
	g.settlement = Settlement{}
	g.dealerRevealed = false

	g.deck.Shuffle()
	g.bankroll -= escrow

	for i := 0; i < 3; i++ {
		g.playerCards[i] = g.deck.draw()
	}
	for i := 0; i < 3; i++ {
		g.dealerCards[i] = g.deck.draw()
	}
	g.playerValue = Evaluate(g.playerCards)
	g.dealerValue = Evaluate(g.dealerCards)
	g.dealt = true

	// Pair Plus-only hands have no Play/Fold decision; settle at once.
	if g.ante == 0 {
		g.settle(false, false)
		return nil
	}
	g.phase = PhaseDecision
	return nil
}

// ---- Decision ----

// CanPlay reports whether the player may Play: the phase is Decision and the
// bankroll can cover the Play wager (equal to the Ante).
func (g *Game) CanPlay() bool {
	return g.phase == PhaseDecision && g.bankroll >= g.ante
}

// Play posts the Play wager (equal to the Ante) and settles the hand at
// showdown against the dealer. It is illegal when the bankroll cannot cover the
// Play wager.
func (g *Game) Play() error {
	if g.phase != PhaseDecision {
		return ErrWrongPhase
	}
	if g.bankroll < g.ante {
		return ErrIllegalAction
	}
	g.bankroll -= g.ante
	g.playBet = g.ante
	g.settle(true, false)
	return nil
}

// Fold forfeits the Ante and ends the hand. Any Pair Plus wager is still
// resolved on the dealt cards.
func (g *Game) Fold() error {
	if g.phase != PhaseDecision {
		return ErrWrongPhase
	}
	g.settle(false, true)
	return nil
}

// ---- Settlement ----

// Qualifies reports whether a dealer hand qualifies: any Pair-or-better, or a
// High Card hand whose top card is Queen or Ace (Queen-high qualifies, Jack-high
// does not).
func Qualifies(v HandValue) bool {
	if v.Category > HighCard {
		return true
	}
	return v.highRank >= Queen
}

// settle computes the full component breakdown, applies all returns to the
// bankroll atomically, reveals the dealer, and moves to PhaseRoundOver. played
// is true when the player posted the Play wager; folded records a fold for the
// result (a Pair Plus-only hand is neither played nor folded).
func (g *Game) settle(played, folded bool) {
	pv, dv := g.playerValue, g.dealerValue
	qualified := Qualifies(dv)

	s := Settlement{DealerQualified: qualified, Played: played, Folded: folded}
	returns := 0

	// Ante.
	if g.ante > 0 {
		s.Ante.Bet = g.ante
		switch {
		case !played:
			// Folded: Ante is forfeited.
			s.Ante.Outcome = OutcomeLoss
			s.Ante.Net = -g.ante
		case !qualified:
			// Dealer does not qualify: Ante pays 1:1.
			s.Ante.Outcome = OutcomeWin
			s.Ante.Net = g.ante
			returns += 2 * g.ante
		default:
			switch cmp := Compare(pv, dv); {
			case cmp > 0:
				s.Ante.Outcome = OutcomeWin
				s.Ante.Net = g.ante
				returns += 2 * g.ante
			case cmp < 0:
				s.Ante.Outcome = OutcomeLoss
				s.Ante.Net = -g.ante
			default:
				s.Ante.Outcome = OutcomePush
				returns += g.ante
			}
		}
	}

	// Play (only when the player Played).
	if played {
		s.Play.Bet = g.playBet
		if !qualified {
			// Dealer does not qualify: the Play wager pushes.
			s.Play.Outcome = OutcomePush
			returns += g.playBet
		} else {
			switch cmp := Compare(pv, dv); {
			case cmp > 0:
				s.Play.Outcome = OutcomeWin
				s.Play.Net = g.playBet
				returns += 2 * g.playBet
			case cmp < 0:
				s.Play.Outcome = OutcomeLoss
				s.Play.Net = -g.playBet
			default:
				s.Play.Outcome = OutcomePush
				returns += g.playBet
			}
		}
	}

	// Ante Bonus (only when Played), independent of dealer qualification/outcome.
	if played {
		if m := anteBonusMultiplier(pv.Category); m > 0 {
			profit := g.ante * m
			s.AnteBonus = BonusResult{Applies: true, Category: pv.Category, Bet: g.ante, Net: profit}
			returns += profit
		}
	}

	// Pair Plus, independent of the dealer and of fold/play.
	if g.pairPlus > 0 {
		s.PairPlus.Bet = g.pairPlus
		if m, ok := pairPlusMultiplier(pv.Category); ok {
			profit := g.pairPlus * m
			s.PairPlus.Outcome = OutcomeWin
			s.PairPlus.Net = profit
			returns += g.pairPlus + profit
		} else {
			s.PairPlus.Outcome = OutcomeLoss
			s.PairPlus.Net = -g.pairPlus
		}
	}

	s.Net = s.Ante.Net + s.Play.Net + s.AnteBonus.Net + s.PairPlus.Net

	g.bankroll += returns
	g.settlement = s
	g.dealerRevealed = true
	g.phase = PhaseRoundOver
}

// ---- Next hand ----

// NextHand advances from a settled hand to the next bet, or to game over if the
// bankroll can no longer cover the minimum bet. The previous Ante and Pair Plus
// carry over, clamped down to what the bankroll still affords (Pair Plus reduced
// first), always keeping at least one affordable wager.
func (g *Game) NextHand() error {
	if g.phase != PhaseRoundOver {
		return ErrWrongPhase
	}
	if g.bankroll < MinBet {
		g.phase = PhaseGameOver
		return nil
	}

	ante, pp := g.ante, g.pairPlus
	if ante == 0 && pp == 0 {
		ante = MinBet
	}
	// Clamp the Ante down to the bankroll (bankroll >= MinBet guaranteed above).
	if ante > g.bankroll {
		ante = g.bankroll
	}
	// Reduce Pair Plus to fit the remaining bankroll; drop it if it can't meet
	// the minimum.
	if ante+pp > g.bankroll {
		pp = g.bankroll - ante
		if pp < MinBet {
			pp = 0
		}
	}
	// Guarantee at least one placed wager.
	if ante == 0 && pp == 0 {
		ante = MinBet
	}

	g.ante = ante
	g.pairPlus = pp
	g.playBet = 0
	g.phase = PhaseBetting
	return nil
}
