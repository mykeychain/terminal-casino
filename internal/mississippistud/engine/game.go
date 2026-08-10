package engine

import (
	"errors"
	"math/rand/v2"
)

// Engine-wide constants.
const (
	// StartingBankroll is the player's starting money.
	StartingBankroll = 1000
	// MinBet is the table minimum, i.e. the smallest legal Ante.
	MinBet = 3
)

// pcgStream is a fixed second PCG parameter so a single int64 seed fully
// determines the RNG stream (owned, injected RNG; no wall-clock dependence).
const pcgStream = 0x9E3779B97F4A7C15

// Phase is the current state of the round's state machine.
type Phase int

const (
	// PhaseBetting: the player sets the Ante and calls Deal.
	PhaseBetting Phase = iota
	// Phase3rdStreet: 2 hole cards are up; the player Folds or Raises. This is
	// the first betting decision (the community cards are still face down).
	Phase3rdStreet
	// Phase4thStreet: the first community card is up (3 cards visible); the
	// player Folds or Raises.
	Phase4thStreet
	// Phase5thStreet: two community cards are up (4 cards visible); the player
	// Folds or Raises. A Raise reveals the final card and settles.
	Phase5thStreet
	// PhaseRoundOver: the hand is settled; results are available via Result. Call
	// NextHand to continue.
	PhaseRoundOver
	// PhaseGameOver: the bankroll is below the minimum bet; play cannot continue.
	PhaseGameOver
)

// String returns the phase name.
func (p Phase) String() string {
	switch p {
	case PhaseBetting:
		return "betting"
	case Phase3rdStreet:
		return "3rd-street"
	case Phase4thStreet:
		return "4th-street"
	case Phase5thStreet:
		return "5th-street"
	case PhaseRoundOver:
		return "round-over"
	case PhaseGameOver:
		return "game-over"
	default:
		return "unknown"
	}
}

// numStreets is the number of betting streets (3rd, 4th, 5th).
const numStreets = 3

// streetLabels are the human-readable names for each street index.
var streetLabels = [numStreets]string{"3rd Street", "4th Street", "5th Street"}

// Outcome is the settled result of the hand's single wager.
type Outcome int

const (
	OutcomeNone Outcome = iota // not yet settled
	OutcomeWin                 // the hand won (Net is the profit)
	OutcomePush                // the hand pushed (total wager returned, Net 0)
	OutcomeLoss                // the hand lost (Net is the negative total wager)
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

// Settlement is the settled result of a hand. It is meaningful once the phase
// reaches PhaseRoundOver. Net is the total bankroll change for the hand and, by
// the money invariant, equals the bankroll delta over the whole round.
type Settlement struct {
	Folded       bool         // the player folded, forfeiting the total wager
	Category     HandCategory // the final hand's category (valid when not folded)
	HandName     string       // human-readable final hand ("Flush", "Pair of Jacks", ...)
	Outcome      Outcome      // win / push / loss
	Multiplier   int          // profit-per-unit applied (0 on push/loss/fold)
	TotalWagered int          // ante + every street bet placed
	Profit       int          // profit paid (0 unless a win)
	Net          int          // total net bankroll change: +Profit, 0, or -TotalWagered
}

// CommunityCard is a community card together with whether it has been revealed.
// The UI flips backs one at a time as each street's Raise reveals a card.
type CommunityCard struct {
	Card     Card
	Revealed bool
}

// Game holds all mutable state for one Mississippi Stud game. Nothing is stored
// at package scope, so many Games can run concurrently in one process.
type Game struct {
	rng  *rand.Rand
	deck *Deck

	bankroll int
	phase    Phase

	ante       int              // pending / active Ante wager
	streetBets [numStreets]int  // amount raised on each street (0 until raised)
	holeCards  [2]Card          // face up from the deal
	community  [numStreets]Card // dealt face down; revealed one per street
	revealed   [numStreets]bool // per-community-card reveal flag
	dealt      bool

	settlement Settlement
}

// NewGame constructs a game with a time-independent, injected RNG seed.
// Production supplies a seed at the call site; tests may supply any fixed seed
// for reproducibility. The Ante defaults to MinBet.
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
// Deal order: cards[0], cards[1] are the two hole cards (face up); cards[2],
// cards[3], cards[4] are the three community cards in reveal order (index 2 is
// revealed first, at 3rd Street). Provide at least five cards.
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

// Ante returns the pending/active Ante wager.
func (g *Game) Ante() int { return g.ante }

// CardsRemaining returns how many cards are left in the deck.
func (g *Game) CardsRemaining() int { return g.deck.Remaining() }

// StreetIndex returns the current street's 0-based index (0 = 3rd, 1 = 4th,
// 2 = 5th), or -1 when the game is not awaiting a street decision.
func (g *Game) StreetIndex() int {
	switch g.phase {
	case Phase3rdStreet:
		return 0
	case Phase4thStreet:
		return 1
	case Phase5thStreet:
		return 2
	default:
		return -1
	}
}

// StreetLabel returns the current street's label (e.g. "3rd Street"), or "" when
// the game is not awaiting a street decision.
func (g *Game) StreetLabel() string {
	if i := g.StreetIndex(); i >= 0 {
		return streetLabels[i]
	}
	return ""
}

// StreetBets returns a copy of the amount raised on each street (0 where the
// player has not yet raised).
func (g *Game) StreetBets() []int {
	return append([]int(nil), g.streetBets[:]...)
}

// TotalWagered returns the total staked so far this hand: the Ante plus every
// street bet placed. It equals Result().TotalWagered once the hand is settled.
func (g *Game) TotalWagered() int {
	if !g.dealt || g.phase == PhaseBetting {
		return 0
	}
	total := g.ante
	for _, b := range g.streetBets {
		total += b
	}
	return total
}

// HoleCards returns a copy of the two face-up hole cards, or an empty slice
// while no hand is in progress (during betting, before a hand is dealt).
func (g *Game) HoleCards() []Card {
	if !g.dealt {
		return nil
	}
	return append([]Card(nil), g.holeCards[:]...)
}

// CommunityCards returns a copy of the three community cards, each tagged with
// whether it has been revealed. Empty while no hand is in progress (during
// betting, before a hand is dealt).
func (g *Game) CommunityCards() []CommunityCard {
	if !g.dealt {
		return nil
	}
	out := make([]CommunityCard, numStreets)
	for i := range out {
		out[i] = CommunityCard{Card: g.community[i], Revealed: g.revealed[i]}
	}
	return out
}

// Result returns the settlement for the most recently settled hand. It is
// meaningful once the phase reaches PhaseRoundOver.
func (g *Game) Result() Settlement { return g.settlement }

// ---- Betting ----

// SetAnte sets the pending Ante. It must be at least MinBet and no more than the
// bankroll. The Ante is mandatory in Mississippi Stud (there is no zero bet).
func (g *Game) SetAnte(amount int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if amount < MinBet {
		return ErrInvalidBet
	}
	if amount > g.bankroll {
		return ErrInvalidBet
	}
	g.ante = amount
	return nil
}

// ---- Deal ----

// Deal starts a hand: it validates and escrows the Ante, shuffles a fresh deck,
// deals two hole cards face up and three community cards face down, and advances
// to 3rd Street.
func (g *Game) Deal() error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if g.ante < MinBet {
		return ErrInvalidBet
	}
	if g.ante > g.bankroll {
		return ErrInvalidBet
	}

	// Reset round state.
	g.streetBets = [numStreets]int{}
	g.revealed = [numStreets]bool{}
	g.settlement = Settlement{}

	g.deck.Shuffle()
	g.bankroll -= g.ante // escrow the Ante

	g.holeCards[0] = g.deck.draw()
	g.holeCards[1] = g.deck.draw()
	for i := 0; i < numStreets; i++ {
		g.community[i] = g.deck.draw()
	}
	g.dealt = true
	g.phase = Phase3rdStreet
	return nil
}

// ---- Street decisions ----

// CanRaise reports whether raising by the given multiple (1, 2, or 3) is legal
// right now. Under the bankroll policy a multiple is affordable only when
// mult*Ante fits the current bankroll.
func (g *Game) CanRaise(mult int) bool {
	if g.StreetIndex() < 0 {
		return false
	}
	if mult < 1 || mult > 3 {
		return false
	}
	return mult*g.ante <= g.bankroll
}

// LegalRaises returns the affordable raise multiples (a subset of {1,2,3}) for
// the current street, in ascending order.
//
// Edge case: when the bankroll cannot afford even 1x the Ante this returns an
// empty slice, meaning only Fold is legal. The engine never offers a partial or
// all-in raise; a raise is always exactly 1x, 2x, or 3x the Ante.
func (g *Game) LegalRaises() []int {
	var out []int
	for m := 1; m <= 3; m++ {
		if g.CanRaise(m) {
			out = append(out, m)
		}
	}
	return out
}

// Raise places a raise of exactly mult times the Ante (mult in {1,2,3}) on the
// current street, escrows it, reveals that street's community card, and advances
// to the next street (settling the hand after 5th Street). It returns
// ErrInvalidBet for a multiple outside {1,2,3} and ErrIllegalAction when the
// bankroll cannot afford the raise.
func (g *Game) Raise(mult int) error {
	idx := g.StreetIndex()
	if idx < 0 {
		return ErrWrongPhase
	}
	if mult < 1 || mult > 3 {
		return ErrInvalidBet
	}
	amount := mult * g.ante
	if amount > g.bankroll {
		return ErrIllegalAction
	}

	g.bankroll -= amount // escrow the raise
	g.streetBets[idx] = amount
	g.revealed[idx] = true // reveal this street's community card

	switch g.phase {
	case Phase3rdStreet:
		g.phase = Phase4thStreet
	case Phase4thStreet:
		g.phase = Phase5thStreet
	case Phase5thStreet:
		g.settle(false)
	}
	return nil
}

// Fold forfeits everything wagered so far (the escrowed Ante plus any street
// bets already placed) and ends the hand as a total loss. It is legal on any
// street.
func (g *Game) Fold() error {
	if g.StreetIndex() < 0 {
		return ErrWrongPhase
	}
	g.settle(true)
	return nil
}

// ---- Settlement ----

// settle resolves the hand against the pay table, applies the return to the
// bankroll atomically, and moves to PhaseRoundOver. folded is true when the
// player folded; a fold forfeits the whole total wager regardless of the cards.
func (g *Game) settle(folded bool) {
	total := g.TotalWagered()
	s := Settlement{Folded: folded, TotalWagered: total}

	returns := 0
	switch {
	case folded:
		s.Outcome = OutcomeLoss
		s.HandName = "Folded"
		s.Net = -total
	default:
		v := Evaluate([5]Card{
			g.holeCards[0], g.holeCards[1],
			g.community[0], g.community[1], g.community[2],
		})
		mult, outcome := payout(v)
		s.Category = v.Category
		s.HandName = v.Name()
		s.Outcome = outcome
		switch outcome {
		case OutcomeWin:
			s.Multiplier = mult
			s.Profit = mult * total
			s.Net = s.Profit
			returns = total + s.Profit // stake back plus profit
		case OutcomePush:
			s.Net = 0
			returns = total // stake back only
		default: // OutcomeLoss
			s.Net = -total
			returns = 0
		}
	}

	g.bankroll += returns
	g.settlement = s
	g.phase = PhaseRoundOver
}

// ---- Made-hand hint ----

// VisibleFloor inspects the currently visible cards (the two hole cards plus any
// revealed community cards) and reports the locked-in floor from an already-made
// pair. A pair of 6s or better guarantees the final hand is at least a PUSH; a
// pair of Jacks or better guarantees at least a WIN. It is a read-only hint; the
// UI decides whether and how to surface it. Both false means no such floor.
func (g *Game) VisibleFloor() (guaranteesPush, guaranteesWin bool) {
	r, ok := highestPairRank(g.visibleCards())
	if !ok {
		return false, false
	}
	switch {
	case r >= PairWinFloor:
		return true, true
	case r >= PairPushFloor:
		return true, false
	default:
		return false, false
	}
}

// visibleCards returns the cards the player can currently see: both hole cards
// and every revealed community card. It is empty before the first Deal.
func (g *Game) visibleCards() []Card {
	if !g.dealt {
		return nil
	}
	cards := []Card{g.holeCards[0], g.holeCards[1]}
	for i, c := range g.community {
		if g.revealed[i] {
			cards = append(cards, c)
		}
	}
	return cards
}

// highestPairRank returns the highest rank that appears at least twice among the
// cards, and whether any such pair exists.
func highestPairRank(cards []Card) (Rank, bool) {
	counts := map[Rank]int{}
	for _, c := range cards {
		counts[c.Rank]++
	}
	best, found := Rank(0), false
	for r, n := range counts {
		if n >= 2 && r > best {
			best, found = r, true
		}
	}
	return best, found
}

// ---- Next hand ----

// NextHand advances from a settled hand to the next bet, or to game over when
// the bankroll can no longer cover the minimum bet. The previous Ante carries
// over, clamped down to what the bankroll still affords. The last hand's cards
// are cleared so the betting screen starts with a clean board, identical to the
// state before the very first deal.
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
	// Clamp the Ante down to the bankroll (bankroll >= MinBet guaranteed above).
	if ante > g.bankroll {
		ante = g.bankroll
	}

	g.ante = ante
	g.streetBets = [numStreets]int{}
	g.revealed = [numStreets]bool{}
	g.holeCards = [2]Card{}
	g.community = [numStreets]Card{}
	g.dealt = false
	g.phase = PhaseBetting
	return nil
}
