package engine

import (
	"errors"
	"math/rand/v2"
)

// Engine-wide constants.
const (
	// StartingBankroll is the player's starting money.
	StartingBankroll = 1000
	// MinBet is the table minimum (and the $-increment floor).
	MinBet = 3
	// BetIncrement is the smallest chip denomination; bets must be multiples.
	BetIncrement = 1
	// NumDecks is the shoe size.
	NumDecks = 6
	// MaxHands is the maximum number of hands a single spot may reach after
	// splits. Enforced per spot (a split counts only hands sharing that spot).
	MaxHands = 4
	// MaxSpots is the maximum number of opened hands (betting positions) a
	// player may set up before the deal.
	MaxSpots = 3
	// DealerPushValue is the dealer total that pushes in Free Bet Blackjack: a
	// dealer 22 does NOT bust — it ties every non-busted player hand instead
	// (the "Push 22" rule that pays for all the free doubles and splits). Dealer
	// totals of 23+ still bust normally.
	DealerPushValue = 22
)

// pcgStream is a fixed second PCG parameter so a single int64 seed fully
// determines the RNG stream (owned, injected RNG).
const pcgStream = 0x9E3779B97F4A7C15

// Phase is the current state of the round's state machine.
type Phase int

const (
	// PhaseBetting: player sets bets (one per opened spot) and calls Deal.
	PhaseBetting Phase = iota
	// PhaseInsurance: dealer shows an Ace and insurance is affordable for at
	// least one spot; the player answers Insurance(bool) for each offered spot.
	PhaseInsurance
	// PhasePlayerTurn: player acts on the active hand.
	PhasePlayerTurn
	// PhaseRoundOver: the round is settled; results are available. Call
	// NextHand to continue.
	PhaseRoundOver
	// PhaseGameOver: bankroll is below the minimum bet; the game cannot continue.
	PhaseGameOver
)

// String returns the phase name.
func (p Phase) String() string {
	switch p {
	case PhaseBetting:
		return "betting"
	case PhaseInsurance:
		return "insurance"
	case PhasePlayerTurn:
		return "player-turn"
	case PhaseRoundOver:
		return "round-over"
	case PhaseGameOver:
		return "game-over"
	default:
		return "unknown"
	}
}

// Action is a player action the engine may consider legal.
type Action int

const (
	ActionHit Action = iota
	ActionStand
	ActionDouble
	ActionSplit
	// ActionInsurance is legal during PhaseInsurance; the player answers via
	// Insurance(true) to take it or Insurance(false) to decline.
	ActionInsurance
)

// String returns the action name.
func (a Action) String() string {
	switch a {
	case ActionHit:
		return "hit"
	case ActionStand:
		return "stand"
	case ActionDouble:
		return "double"
	case ActionSplit:
		return "split"
	case ActionInsurance:
		return "insurance"
	default:
		return "unknown"
	}
}

// Outcome is the settled result of a single player hand.
type Outcome int

const (
	OutcomePending   Outcome = iota
	OutcomeWin               // ordinary win, paid 1:1 on the hand's wager
	OutcomeLose              // loss (only the real stake is lost; free bets cost nothing)
	OutcomePush              // tie (incl. dealer 22), real stake returned
	OutcomeBlackjack         // player natural, paid 3:2
)

// String returns the outcome name.
func (o Outcome) String() string {
	switch o {
	case OutcomeWin:
		return "win"
	case OutcomeLose:
		return "lose"
	case OutcomePush:
		return "push"
	case OutcomeBlackjack:
		return "blackjack"
	default:
		return "pending"
	}
}

// Common errors.
var (
	ErrWrongPhase    = errors.New("action not allowed in current phase")
	ErrIllegalAction = errors.New("action is not currently legal")
	ErrInvalidBet    = errors.New("invalid bet amount")
)

// playerHand is one of the player's hands plus its per-hand state. This is the
// engine's internal representation; the UI reads the exported HandView instead.
//
// A hand carries two wager components. bet is real money at risk, escrowed from
// the bankroll: it is returned on a push and lost on a loss. free is a
// house-funded "free bet" (from a free double or free split): it is never
// escrowed, pays 1:1 alongside bet on a win, and simply vanishes on a loss or
// push — the player risks nothing on it.
type playerHand struct {
	hand      Hand
	spot      int     // originating opened-spot index (splits inherit the parent's)
	bet       int     // real money escrowed for this hand (own-money doubles/splits included)
	free      int     // house-funded free-bet amount (free doubles/splits); win-only, never escrowed
	doubled   bool    // hand was doubled (free or own-money; drew exactly one extra card)
	stood     bool    // hand is finished (stood, doubled, bust, or 21)
	splitAce  bool    // hand came from splitting aces (one-and-done)
	fromSplit bool    // hand came from any split (a two-card 21 is not a natural)
	natural   bool    // this spot's initial two-card hand was a natural
	outcome   Outcome // settled outcome
	net       int     // net change to bankroll from this hand (profit or -stake)

	// Per-spot insurance side bet (tracked on the spot's initial hand).
	insuranceTaken bool
	insuranceBet   int
	insuranceNet   int
}

// Game holds all mutable state for one blackjack game. Nothing is stored at
// package scope, so many Games can run concurrently in one process.
type Game struct {
	rng  *rand.Rand
	shoe *Shoe

	bankroll int
	spotBets []int // pending per-spot bets during PhaseBetting (length 1..MaxSpots)
	phase    Phase

	// Round state.
	hands  []*playerHand
	dealer Hand
	active int // index into hands of the hand currently being played

	dealerRevealed bool // dealer hole card is visible
	dealerBJ       bool // dealer had a natural blackjack (peek)

	insuranceSpot int // spot currently offered insurance (-1 when none)

	reshuffled bool // the most recent Deal triggered a reshuffle
}

// NewGame constructs a game with a time-independent, injected RNG seed.
// Production supplies a time-based seed at the call site; tests may supply any
// fixed seed for reproducibility.
func NewGame(seed int64) *Game {
	rng := rand.New(rand.NewPCG(uint64(seed), pcgStream))
	g := &Game{
		rng:           rng,
		bankroll:      StartingBankroll,
		spotBets:      []int{MinBet},
		phase:         PhaseBetting,
		insuranceSpot: -1,
	}
	g.shoe = newShoe(NumDecks, rng)
	return g
}

// NewGameWithShoe constructs a game with a pre-stacked shoe and a chosen
// starting bankroll, for deterministic tests. Cards are dealt from index 0.
//
// Single-spot deal order (one opened hand): player card 1, dealer up-card,
// player card 2, dealer hole card, then subsequent draws in order.
//
// Multi-spot deal order (N opened hands, L->R): one card to each spot, dealer
// up-card, a second card to each spot, dealer hole card, then subsequent draws
// in order. See Deal for details. Keep fewer than CutCardPosition draws so no
// reshuffle is attempted (the stacked shoe owns no RNG).
func NewGameWithShoe(bankroll int, cards []Card) *Game {
	g := &Game{
		bankroll:      bankroll,
		spotBets:      []int{MinBet},
		phase:         PhaseBetting,
		insuranceSpot: -1,
	}
	g.shoe = newStackedShoe(cards)
	return g
}

// ---- Accessors (read-only views for the UI) ----

// Phase returns the current phase.
func (g *Game) Phase() Phase { return g.phase }

// Bankroll returns the current (post-escrow) bankroll.
func (g *Game) Bankroll() int { return g.bankroll }

// NumSpots returns the number of opened hands (betting positions).
func (g *Game) NumSpots() int { return len(g.spotBets) }

// SpotBet returns the pending bet for opened spot i (0 if i is out of range).
// The value is the spot's original stake and is unaffected by later doubles or
// splits, so it remains valid for insurance sizing during play.
func (g *Game) SpotBet(i int) int {
	if i < 0 || i >= len(g.spotBets) {
		return 0
	}
	return g.spotBets[i]
}

// Bet returns the pending main bet (opened spot 0), for single-hand back-compat.
func (g *Game) Bet() int { return g.SpotBet(0) }

// ActiveHandIndex returns the index of the hand currently being played.
func (g *Game) ActiveHandIndex() int { return g.active }

// InsuranceSpot returns the spot index currently being offered insurance during
// PhaseInsurance, or -1 when no offer is active. The offered amount is
// SpotBet(InsuranceSpot())/2.
func (g *Game) InsuranceSpot() int { return g.insuranceSpot }

// TookInsurance reports whether the player took insurance on any spot this round.
func (g *Game) TookInsurance() bool {
	for _, h := range g.hands {
		if h.insuranceTaken {
			return true
		}
	}
	return false
}

// InsuranceBet returns the total insurance side-bet posted across all spots.
func (g *Game) InsuranceBet() int {
	total := 0
	for _, h := range g.hands {
		total += h.insuranceBet
	}
	return total
}

// InsuranceNet returns the total net bankroll change from insurance across all
// spots after settlement.
func (g *Game) InsuranceNet() int {
	total := 0
	for _, h := range g.hands {
		total += h.insuranceNet
	}
	return total
}

// DealerHadBlackjack reports whether the dealer had a natural (known after peek).
func (g *Game) DealerHadBlackjack() bool { return g.dealerBJ }

// DealerPush22 reports whether the dealer's revealed hand is exactly 22 — the
// Push 22 rule, under which every non-busted player hand pushes. Valid once the
// dealer hand is revealed.
func (g *Game) DealerPush22() bool {
	return g.dealerRevealed && g.dealer.Value() == DealerPushValue
}

// CardsRemaining returns how many cards are left in the shoe.
func (g *Game) CardsRemaining() int { return g.shoe.Remaining() }

// CutCardReached reports whether the shoe has passed its cut card.
func (g *Game) CutCardReached() bool { return g.shoe.CutCardReached() }

// DidReshuffle reports whether the most recent Deal reshuffled the shoe.
func (g *Game) DidReshuffle() bool { return g.reshuffled }

// HandView is a read-only snapshot of a player hand for the UI.
type HandView struct {
	Cards     []Card
	Value     int
	Soft      bool
	Bust      bool
	Blackjack bool // a scoring natural (two-card 21, not from a split)
	Spot      int  // originating opened-spot index
	Bet       int  // real money at risk on this hand
	Free      int  // house-funded free-bet amount riding on this hand (win-only)
	Doubled   bool
	SplitAce  bool
	Active    bool
	Outcome   Outcome
	Net       int
}

// Player returns snapshots of all player hands, in play order.
func (g *Game) Player() []HandView {
	views := make([]HandView, len(g.hands))
	for i, h := range g.hands {
		v, soft := h.hand.eval()
		views[i] = HandView{
			Cards:     append([]Card(nil), h.hand.Cards...),
			Value:     v,
			Soft:      soft,
			Bust:      h.hand.IsBust(),
			Blackjack: h.hand.IsBlackjack() && !h.fromSplit,
			Spot:      h.spot,
			Bet:       h.bet,
			Free:      h.free,
			Doubled:   h.doubled,
			SplitAce:  h.splitAce,
			Active:    g.phase == PhasePlayerTurn && i == g.active,
			Outcome:   h.outcome,
			Net:       h.net,
		}
	}
	return views
}

// DealerView is a read-only snapshot of the dealer hand for the UI.
type DealerView struct {
	Cards     []Card // full hand when Revealed; otherwise up-card + hidden slots
	Upcard    Card
	Value     int  // full value when Revealed; up-card value otherwise
	Soft      bool // soft flag for the reported value
	Revealed  bool // hole card visible
	Blackjack bool // dealer natural (valid once revealed / after peek)
	Push22    bool // dealer stands on 22 (pushes all non-busted hands)
}

// Dealer returns a snapshot of the dealer hand. While the hole card is hidden,
// only the up-card and its value are reported.
func (g *Game) Dealer() DealerView {
	dv := DealerView{
		Cards:    append([]Card(nil), g.dealer.Cards...),
		Revealed: g.dealerRevealed,
	}
	if len(g.dealer.Cards) > 0 {
		dv.Upcard = g.dealer.Cards[0]
	}
	if g.dealerRevealed {
		v, soft := g.dealer.eval()
		dv.Value = v
		dv.Soft = soft
		dv.Blackjack = g.dealerBJ
		dv.Push22 = v == DealerPushValue
	} else if len(g.dealer.Cards) > 0 {
		up := Hand{Cards: []Card{g.dealer.Cards[0]}}
		dv.Value, dv.Soft = up.eval()
	}
	return dv
}

// ---- Legal action set ----

// LegalActions returns the set of actions currently legal, already accounting
// for bankroll affordability. In Free Bet Blackjack a double on a two-card hard
// 9/10/11 and a split of any non-ten pair are FREE — they cost no additional
// real money and so are always legal regardless of bankroll. Own-money doubles
// (any other two-card total) and own-money splits (ten-value pairs) appear only
// when the player can fund the extra real bet.
func (g *Game) LegalActions() []Action {
	switch g.phase {
	case PhaseInsurance:
		return []Action{ActionInsurance}
	case PhasePlayerTurn:
		h := g.hands[g.active]
		acts := []Action{ActionHit, ActionStand}
		twoCards := len(h.hand.Cards) == 2
		wager := h.bet + h.free
		if twoCards && !h.splitAce {
			if g.doubleIsFree(h) || g.canAfford(wager) {
				acts = append(acts, ActionDouble)
			}
		}
		if twoCards && !h.splitAce &&
			h.hand.Cards[0].Rank == h.hand.Cards[1].Rank &&
			g.spotHandCount(h.spot) < MaxHands {
			if g.splitIsFree(h) || g.canAfford(wager) {
				acts = append(acts, ActionSplit)
			}
		}
		return acts
	default:
		return nil
	}
}

// doubleIsFree reports whether doubling the given (two-card) hand is a free bet:
// the Free Bet rule grants a free double on any two-card hard 9, 10, or 11.
func (g *Game) doubleIsFree(h *playerHand) bool {
	v, soft := h.hand.eval()
	return !soft && v >= 9 && v <= 11
}

// splitIsFree reports whether splitting the given pair is a free bet: every pair
// splits free except ten-value pairs, which cost the player's own money.
func (g *Game) splitIsFree(h *playerHand) bool {
	return !h.hand.Cards[0].isTenValue()
}

// DoubleIsFree reports whether a double on the active hand would be a free bet
// (two-card hard 9/10/11). Used by the UI to label the action; false outside the
// player turn or when a double is not applicable.
func (g *Game) DoubleIsFree() bool {
	if g.phase != PhasePlayerTurn {
		return false
	}
	h := g.hands[g.active]
	if len(h.hand.Cards) != 2 {
		return false
	}
	return g.doubleIsFree(h)
}

// SplitIsFree reports whether a split of the active hand would be a free bet
// (any non-ten pair). Used by the UI to label the action; false outside the
// player turn or when a split is not applicable.
func (g *Game) SplitIsFree() bool {
	if g.phase != PhasePlayerTurn {
		return false
	}
	h := g.hands[g.active]
	if len(h.hand.Cards) != 2 {
		return false
	}
	return g.splitIsFree(h)
}

// spotHandCount returns how many current hands originate from the given spot.
// The split cap (MaxHands) is enforced per spot, so opening or splitting one
// spot never affects another's legality.
func (g *Game) spotHandCount(spot int) int {
	n := 0
	for _, h := range g.hands {
		if h.spot == spot {
			n++
		}
	}
	return n
}

// CanAct reports whether the given action is currently legal.
func (g *Game) CanAct(a Action) bool {
	for _, x := range g.LegalActions() {
		if x == a {
			return true
		}
	}
	return false
}

func (g *Game) canAfford(amount int) bool { return g.bankroll >= amount }

// ---- Betting ----

// SetSpotBet sets the pending bet for opened spot i. The amount must be at least
// MinBet, a multiple of BetIncrement, and leave the new total across all spots
// no greater than the current bankroll.
func (g *Game) SetSpotBet(i, amount int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if i < 0 || i >= len(g.spotBets) {
		return ErrIllegalAction
	}
	if amount < MinBet || amount%BetIncrement != 0 {
		return ErrInvalidBet
	}
	if g.totalStaked()-g.spotBets[i]+amount > g.bankroll {
		return ErrInvalidBet
	}
	g.spotBets[i] = amount
	return nil
}

// SetBet sets the pending main bet (opened spot 0), for single-hand back-compat.
func (g *Game) SetBet(amount int) error { return g.SetSpotBet(0, amount) }

// AddSpot opens an additional betting position, defaulting its bet to the
// previous (last) hand's bet — clamped down to what the bankroll can still cover,
// but at least MinBet. It fails if the cap (MaxSpots) is reached or another hand
// cannot be afforded even at MinBet.
func (g *Game) AddSpot() error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if len(g.spotBets) >= MaxSpots {
		return ErrIllegalAction
	}
	avail := g.bankroll - g.totalStaked()
	if avail < MinBet {
		return ErrInvalidBet
	}
	bet := g.spotBets[len(g.spotBets)-1] // default to the previous hand's bet
	if bet > avail {
		bet = (avail / BetIncrement) * BetIncrement
	}
	if bet < MinBet {
		bet = MinBet
	}
	g.spotBets = append(g.spotBets, bet)
	return nil
}

// RemoveSpot closes opened spot i. It is illegal to remove the last remaining
// spot (there must always be at least one).
func (g *Game) RemoveSpot(i int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if len(g.spotBets) <= 1 {
		return ErrIllegalAction
	}
	if i < 0 || i >= len(g.spotBets) {
		return ErrIllegalAction
	}
	g.spotBets = append(g.spotBets[:i], g.spotBets[i+1:]...)
	return nil
}

// totalStaked returns the sum of all pending per-spot bets.
func (g *Game) totalStaked() int {
	total := 0
	for _, b := range g.spotBets {
		total += b
	}
	return total
}

// ---- Deal ----

// Deal starts a round: reshuffles if the cut card was reached, escrows the sum
// of the pending per-spot bets, deals one hand per spot, and advances to
// insurance, the player turn, or an immediate resolution (dealer/player
// naturals) as appropriate.
//
// Deal order for N opened spots (left to right): one card to each spot, the
// dealer up-card, a second card to each spot, the dealer hole card, then all
// subsequent draws (player hits, then dealer draws) in order. With one spot this
// reduces to the classic player, dealer-up, player, dealer-hole sequence.
func (g *Game) Deal() error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	for _, b := range g.spotBets {
		if b < MinBet || b%BetIncrement != 0 {
			return ErrInvalidBet
		}
	}
	total := g.totalStaked()
	if total > g.bankroll {
		return ErrInvalidBet
	}

	// Reshuffle at the start of the hand if the cut card was passed.
	g.reshuffled = false
	if g.shoe.CutCardReached() {
		g.shoe.reshuffle()
		g.reshuffled = true
	}

	// Reset round state.
	g.dealer = Hand{}
	g.active = 0
	g.dealerRevealed = false
	g.dealerBJ = false
	g.insuranceSpot = -1

	// Escrow the summed real stake and create one hand per spot.
	g.bankroll -= total
	g.hands = make([]*playerHand, len(g.spotBets))
	for i, b := range g.spotBets {
		g.hands[i] = &playerHand{spot: i, bet: b, outcome: OutcomePending}
	}

	// Deal order: first card to each spot L->R, dealer up, second card to each
	// spot L->R, dealer hole.
	for _, h := range g.hands {
		h.hand.add(g.shoe.draw())
	}
	g.dealer.add(g.shoe.draw())
	for _, h := range g.hands {
		h.hand.add(g.shoe.draw())
	}
	g.dealer.add(g.shoe.draw())

	// Mark each spot's initial natural (per hand, not a single bool).
	for _, h := range g.hands {
		h.natural = h.hand.IsBlackjack()
	}

	// Insurance is offered only on an Ace up-card and only to spots that can
	// afford it; otherwise proceed straight to the peek.
	if g.dealer.Cards[0].Rank == Ace {
		g.advanceInsurance(0)
		return nil
	}
	g.resolvePeek()
	return nil
}

// advanceInsurance moves the insurance offer to the first spot at index >= from
// that can afford SpotBet(i)/2 against the current bankroll. If none remain, it
// proceeds to the dealer peek. Affordability is sequential: each taken insurance
// lowers the bankroll available to later offers.
func (g *Game) advanceInsurance(from int) {
	for i := from; i < len(g.hands); i++ {
		if g.canAfford(g.SpotBet(i) / 2) {
			g.insuranceSpot = i
			g.phase = PhaseInsurance
			return
		}
	}
	g.insuranceSpot = -1
	g.resolvePeek()
}

// Insurance answers the insurance offer for the current insurance spot. take=true
// posts that spot's fixed half-bet side wager; take=false declines. Either way
// the offer advances to the next affordable spot, and once none remain the
// dealer peeks.
func (g *Game) Insurance(take bool) error {
	if g.phase != PhaseInsurance {
		return ErrWrongPhase
	}
	i := g.insuranceSpot
	h := g.hands[i]
	if take {
		amt := g.SpotBet(i) / 2
		if !g.canAfford(amt) {
			return ErrIllegalAction
		}
		g.bankroll -= amt
		h.insuranceTaken = true
		h.insuranceBet = amt
	}
	g.advanceInsurance(i + 1)
	return nil
}

// resolvePeek performs the dealer peek (on a ten or ace up-card) and routes to
// the correct next state: dealer blackjack resolution, per-spot natural payouts,
// or the player's turn.
func (g *Game) resolvePeek() {
	up := g.dealer.Cards[0]
	peeks := up.Rank == Ace || up.isTenValue()
	if peeks && g.dealer.IsBlackjack() {
		g.resolveDealerBlackjack()
		return
	}

	// Dealer does not have blackjack. Any insurance taken is lost.
	for _, h := range g.hands {
		if h.insuranceTaken {
			h.insuranceNet = -h.insuranceBet
		}
	}

	// Each spot's initial natural auto-resolves, paid 3:2, with no turn.
	// Remaining hands go to the player turn.
	for _, h := range g.hands {
		if h.natural {
			g.payNatural(h)
		}
	}

	g.phase = PhasePlayerTurn
	g.resolvePlayerTurn()
}

// resolveDealerBlackjack settles the round when the dealer has a natural: every
// non-natural hand loses, a hand holding a natural pushes, and each taken
// insurance pays 2:1. (Peek precedes the player turn, so hands equal spots and
// no free bets exist yet.)
func (g *Game) resolveDealerBlackjack() {
	g.dealerRevealed = true
	g.dealerBJ = true

	for _, h := range g.hands {
		if h.insuranceTaken {
			// Pays 2:1: profit is 2x the stake; stake+profit (3x) returns.
			g.bankroll += h.insuranceBet * 3
			h.insuranceNet = h.insuranceBet * 2
		}
	}

	for _, h := range g.hands {
		if h.natural {
			// Natural vs natural pushes: stake returned.
			g.bankroll += h.bet
			h.outcome = OutcomePush
			h.net = 0
		} else {
			h.outcome = OutcomeLose
			h.net = -h.bet
		}
	}
	g.endRound()
}

// payNatural pays a spot's initial natural 3:2 and marks it resolved. It does
// not reveal the hole card: other spots may still have a turn to play. A natural
// is paid before the dealer plays out, so it is never caught by the dealer's
// Push 22 — a player blackjack always beats a dealer 22.
func (g *Game) payNatural(h *playerHand) {
	profit := h.bet * 3 / 2
	g.bankroll += h.bet + profit
	h.outcome = OutcomeBlackjack
	h.net = profit
	h.stood = true
}

// ---- Player actions ----

// Hit draws a card for the active hand. Busting or reaching 21 finishes the hand.
func (g *Game) Hit() error {
	if g.phase != PhasePlayerTurn {
		return ErrWrongPhase
	}
	if !g.CanAct(ActionHit) {
		return ErrIllegalAction
	}
	h := g.hands[g.active]
	h.hand.add(g.shoe.draw())
	if h.hand.IsBust() || h.hand.Value() == 21 {
		h.stood = true
	}
	g.resolvePlayerTurn()
	return nil
}

// Stand finishes the active hand.
func (g *Game) Stand() error {
	if g.phase != PhasePlayerTurn {
		return ErrWrongPhase
	}
	h := g.hands[g.active]
	h.stood = true
	g.resolvePlayerTurn()
	return nil
}

// Double doubles the active hand: it draws exactly one card and finishes the
// hand. A free double (two-card hard 9/10/11) posts a house-funded free bet
// equal to the hand's wager and costs the player nothing; any other two-card
// total is an own-money double that escrows an additional real bet (and so is
// legal only with sufficient funds). Never legal on a split ace.
func (g *Game) Double() error {
	if g.phase != PhasePlayerTurn {
		return ErrWrongPhase
	}
	if !g.CanAct(ActionDouble) {
		return ErrIllegalAction
	}
	h := g.hands[g.active]
	wager := h.bet + h.free
	if g.doubleIsFree(h) {
		h.free += wager // house pays for the double
	} else {
		g.bankroll -= wager // own-money double
		h.bet += wager
	}
	h.doubled = true
	h.hand.add(g.shoe.draw())
	h.stood = true
	g.resolvePlayerTurn()
	return nil
}

// Split splits an equal-rank pair into two hands. A free split (any non-ten
// pair, including aces) funds the new hand with a house free bet and costs
// nothing; a ten-value pair is an own-money split that escrows an additional
// real bet (and so is legal only with sufficient funds). The original hand keeps
// its existing wager; the new hand inherits a matching wager as free or real
// money accordingly. Aces are one-and-done (one card each, no hit/double/
// re-split); other pairs may re-split up to MaxHands hands per spot.
func (g *Game) Split() error {
	if g.phase != PhasePlayerTurn {
		return ErrWrongPhase
	}
	if !g.CanAct(ActionSplit) {
		return ErrIllegalAction
	}
	h := g.hands[g.active]
	isAce := h.hand.Cards[0].Rank == Ace
	wager := h.bet + h.free
	free := g.splitIsFree(h)
	if !free {
		g.bankroll -= wager // own-money split (ten-value pair)
	}

	// Move the second card into a brand-new hand inserted right after this one,
	// inheriting the parent's originating spot.
	c1 := h.hand.Cards[1]
	h.hand.Cards = h.hand.Cards[:1]
	h.fromSplit = true
	h.splitAce = isAce
	h.stood = false

	newHand := &playerHand{
		hand:      Hand{Cards: []Card{c1}},
		spot:      h.spot,
		fromSplit: true,
		splitAce:  isAce,
		outcome:   OutcomePending,
	}
	if free {
		newHand.free = wager
	} else {
		newHand.bet = wager
	}
	g.hands = append(g.hands, nil)
	copy(g.hands[g.active+2:], g.hands[g.active+1:])
	g.hands[g.active+1] = newHand

	g.resolvePlayerTurn()
	return nil
}

// resolvePlayerTurn advances through hands from the active index, dealing the
// second card to freshly-split hands, auto-finishing split aces and 21/bust
// hands, and stopping at the first playable hand. When no playable hand remains
// it proceeds to the dealer turn and settlement.
func (g *Game) resolvePlayerTurn() {
	for g.active < len(g.hands) {
		h := g.hands[g.active]
		if h.stood {
			g.active++
			continue
		}
		if len(h.hand.Cards) < 2 {
			h.hand.add(g.shoe.draw())
		}
		if h.splitAce {
			h.stood = true
			g.active++
			continue
		}
		if h.hand.IsBust() || h.hand.Value() == 21 {
			h.stood = true
			g.active++
			continue
		}
		return // playable hand
	}
	g.dealerTurnAndSettle()
}

// ---- Dealer turn & settlement ----

// dealerTurnAndSettle reveals the hole card, plays the dealer out (H17: hits
// soft 17) when at least one hand is still live, and settles every unresolved
// hand. Hands resolved at peek (naturals paid 3:2) are skipped.
//
// Free Bet's Push 22 rule lives here: a dealer total of exactly 22 is not a
// bust — every non-busted player hand pushes against it (real stake returned,
// free bets void). Player naturals are already paid at the peek, so a player
// blackjack is never pushed by a dealer 22.
func (g *Game) dealerTurnAndSettle() {
	g.dealerRevealed = true

	// The dealer plays only if some hand is still unresolved and not bust.
	anyLive := false
	for _, h := range g.hands {
		if h.outcome == OutcomePending && !h.hand.IsBust() {
			anyLive = true
			break
		}
	}

	if anyLive {
		for {
			v, soft := g.dealer.eval()
			if v < 17 || (v == 17 && soft) {
				g.dealer.add(g.shoe.draw())
				continue
			}
			break
		}
	}

	dv := g.dealer.Value()
	dealerPush22 := dv == DealerPushValue // dealer 22 pushes, does not bust
	dealerBust := dv > 21 && !dealerPush22

	for _, h := range g.hands {
		if h.outcome != OutcomePending {
			continue // already resolved (natural paid at peek)
		}
		switch {
		case h.hand.IsBust():
			g.settleLose(h)
		case h.hand.IsBlackjack() && !h.fromSplit:
			// Player natural beating a non-blackjack dealer, paid 3:2. (In
			// practice naturals resolve at peek; kept for completeness.)
			profit := h.bet * 3 / 2
			g.bankroll += h.bet + profit
			h.outcome = OutcomeBlackjack
			h.net = profit
		case dealerPush22:
			// Push 22: dealer ties every non-busted, non-natural hand.
			g.settlePush(h)
		case dealerBust:
			g.settleWin(h)
		default:
			pv := h.hand.Value()
			switch {
			case pv > dv:
				g.settleWin(h)
			case pv < dv:
				g.settleLose(h)
			default:
				g.settlePush(h)
			}
		}
	}
	g.endRound()
}

// settleWin pays a winning hand 1:1 on its full wager (real bet + free bet) and
// returns the real stake. The free bet pays like the real bet on a win.
func (g *Game) settleWin(h *playerHand) {
	g.bankroll += h.bet + h.bet + h.free // real stake back + winnings on (bet + free)
	h.outcome = OutcomeWin
	h.net = h.bet + h.free
}

// settleLose loses only the real stake; the free bet costs nothing.
func (g *Game) settleLose(h *playerHand) {
	h.outcome = OutcomeLose
	h.net = -h.bet
}

// settlePush returns the real stake; the free bet is void.
func (g *Game) settlePush(h *playerHand) {
	g.bankroll += h.bet
	h.outcome = OutcomePush
	h.net = 0
}

// endRound moves to the round-over phase.
func (g *Game) endRound() {
	g.phase = PhaseRoundOver
}

// NextHand advances from a settled round to the next bet, or to game over if
// the bankroll can no longer cover the minimum bet. Betting carries over the
// previous round's hands (their count and bets), keeping as many as the bankroll
// still affords at those bets and always keeping at least one hand.
func (g *Game) NextHand() error {
	if g.phase != PhaseRoundOver {
		return ErrWrongPhase
	}
	if g.bankroll < MinBet {
		g.phase = PhaseGameOver
		return nil
	}
	var next []int
	total := 0
	for _, b := range g.spotBets {
		if b < MinBet {
			b = MinBet
		}
		if total+b > g.bankroll {
			break // drop this and any later hand we can no longer afford
		}
		next = append(next, b)
		total += b
	}
	if len(next) == 0 {
		// Even the first hand's previous bet is unaffordable: one hand at the
		// largest affordable bet (bankroll >= MinBet is guaranteed above).
		bet := (g.bankroll / BetIncrement) * BetIncrement
		if bet < MinBet {
			bet = MinBet
		}
		next = []int{bet}
	}
	g.spotBets = next
	g.phase = PhaseBetting
	return nil
}
