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
	MinBet = 25
	// BetIncrement is the smallest chip denomination; bets must be multiples.
	BetIncrement = 5
	// NumDecks is the shoe size.
	NumDecks = 6
	// MaxHands is the maximum number of hands a player may have after splits.
	MaxHands = 4
)

// pcgStream is a fixed second PCG parameter so a single int64 seed fully
// determines the RNG stream (locked decision 5: owned, injected RNG).
const pcgStream = 0x9E3779B97F4A7C15

// Phase is the current state of the round's state machine.
type Phase int

const (
	// PhaseBetting: player sets a bet and calls Deal.
	PhaseBetting Phase = iota
	// PhaseInsurance: dealer shows an Ace and insurance is affordable; player
	// must answer Insurance(bool).
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
	OutcomePending Outcome = iota
	OutcomeWin              // ordinary win, paid 1:1
	OutcomeLose             // loss
	OutcomePush             // tie, stake returned
	OutcomeBlackjack        // player natural, paid 3:2
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
type playerHand struct {
	hand      Hand
	bet       int     // amount escrowed for this hand (doubles/splits included)
	doubled   bool    // hand was doubled (drew exactly one extra card)
	stood     bool    // hand is finished (stood, doubled, bust, or 21)
	splitAce  bool    // hand came from splitting aces (one-and-done)
	fromSplit bool    // hand came from any split (a two-card 21 is not a natural)
	outcome   Outcome // settled outcome
	net       int     // net change to bankroll from this hand (profit or -stake)
}

// Game holds all mutable state for one blackjack game. Nothing is stored at
// package scope, so many Games can run concurrently in one process.
type Game struct {
	rng  *rand.Rand
	shoe *Shoe

	bankroll int
	bet      int // the pending bet set during PhaseBetting
	phase    Phase

	// Round state.
	hands  []*playerHand
	dealer Hand
	active int // index into hands of the hand currently being played

	dealerRevealed bool // dealer hole card is visible
	dealerBJ       bool // dealer had a natural blackjack (peek)
	playerNatural  bool // the initial player hand was a natural

	tookInsurance bool
	insuranceBet  int
	insuranceNet  int // net change from the insurance side bet, after settlement

	reshuffled bool // the most recent Deal triggered a reshuffle
}

// NewGame constructs a game with a time-independent, injected RNG seed
// (locked decision 5). Production supplies a time-based seed at the call site;
// tests may supply any fixed seed for reproducibility.
func NewGame(seed int64) *Game {
	rng := rand.New(rand.NewPCG(uint64(seed), pcgStream))
	g := &Game{
		rng:      rng,
		bankroll: StartingBankroll,
		bet:      MinBet,
		phase:    PhaseBetting,
	}
	g.shoe = newShoe(NumDecks, rng)
	return g
}

// NewGameWithShoe constructs a game with a pre-stacked shoe and a chosen
// starting bankroll, for deterministic tests. Cards are dealt from index 0.
// The deal order is: player card 1, dealer up-card, player card 2, dealer hole
// card, then subsequent draws in order. Keep fewer than CutCardPosition draws
// so no reshuffle is attempted (the stacked shoe owns no RNG).
func NewGameWithShoe(bankroll int, cards []Card) *Game {
	g := &Game{
		bankroll: bankroll,
		bet:      MinBet,
		phase:    PhaseBetting,
	}
	g.shoe = newStackedShoe(cards)
	return g
}

// ---- Accessors (read-only views for the UI) ----

// Phase returns the current phase.
func (g *Game) Phase() Phase { return g.phase }

// Bankroll returns the current (post-escrow) bankroll.
func (g *Game) Bankroll() int { return g.bankroll }

// Bet returns the pending main bet.
func (g *Game) Bet() int { return g.bet }

// ActiveHandIndex returns the index of the hand currently being played.
func (g *Game) ActiveHandIndex() int { return g.active }

// TookInsurance reports whether the player took insurance this round.
func (g *Game) TookInsurance() bool { return g.tookInsurance }

// InsuranceBet returns the insurance side-bet amount (0 if none).
func (g *Game) InsuranceBet() int { return g.insuranceBet }

// InsuranceNet returns the net bankroll change from insurance after settlement.
func (g *Game) InsuranceNet() int { return g.insuranceNet }

// DealerHadBlackjack reports whether the dealer had a natural (known after peek).
func (g *Game) DealerHadBlackjack() bool { return g.dealerBJ }

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
	Bet       int
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
			Bet:       h.bet,
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
	} else if len(g.dealer.Cards) > 0 {
		up := Hand{Cards: []Card{g.dealer.Cards[0]}}
		dv.Value, dv.Soft = up.eval()
	}
	return dv
}

// ---- Legal action set ----

// LegalActions returns the set of actions currently legal, already accounting
// for bankroll affordability (locked decision 3).
func (g *Game) LegalActions() []Action {
	switch g.phase {
	case PhaseInsurance:
		return []Action{ActionInsurance}
	case PhasePlayerTurn:
		h := g.hands[g.active]
		acts := []Action{ActionHit, ActionStand}
		twoCards := len(h.hand.Cards) == 2
		if twoCards && !h.splitAce && g.canAfford(h.bet) {
			acts = append(acts, ActionDouble)
		}
		if twoCards && !h.splitAce &&
			h.hand.Cards[0].Rank == h.hand.Cards[1].Rank &&
			len(g.hands) < MaxHands && g.canAfford(h.bet) {
			acts = append(acts, ActionSplit)
		}
		return acts
	default:
		return nil
	}
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

// SetBet sets the pending main bet. It must be at least MinBet, no more than
// the current bankroll, and a multiple of BetIncrement.
func (g *Game) SetBet(amount int) error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if amount < MinBet || amount > g.bankroll || amount%BetIncrement != 0 {
		return ErrInvalidBet
	}
	g.bet = amount
	return nil
}

// ---- Deal ----

// Deal starts a round: reshuffles if the cut card was reached, escrows the main
// bet, deals two cards each, and advances to insurance, player turn, or an
// immediate resolution (dealer/player natural) as appropriate.
func (g *Game) Deal() error {
	if g.phase != PhaseBetting {
		return ErrWrongPhase
	}
	if g.bet < MinBet || g.bet > g.bankroll {
		return ErrInvalidBet
	}

	// Reshuffle at the start of the hand if the cut card was passed.
	g.reshuffled = false
	if g.shoe.CutCardReached() {
		g.shoe.reshuffle()
		g.reshuffled = true
	}

	// Reset round state.
	g.hands = nil
	g.dealer = Hand{}
	g.active = 0
	g.dealerRevealed = false
	g.dealerBJ = false
	g.playerNatural = false
	g.tookInsurance = false
	g.insuranceBet = 0
	g.insuranceNet = 0

	// Escrow the main bet (locked decision 4).
	g.bankroll -= g.bet
	main := &playerHand{bet: g.bet, outcome: OutcomePending}

	// Deal order: player, dealer up, player, dealer hole.
	main.hand.add(g.shoe.draw())
	g.dealer.add(g.shoe.draw())
	main.hand.add(g.shoe.draw())
	g.dealer.add(g.shoe.draw())

	g.hands = []*playerHand{main}
	g.playerNatural = main.hand.IsBlackjack()

	// Insurance is offered only on an Ace up-card and only if affordable.
	up := g.dealer.Cards[0]
	if up.Rank == Ace && g.canAfford(g.bet/2) {
		g.phase = PhaseInsurance
		return nil
	}
	g.resolvePeek()
	return nil
}

// Insurance answers the insurance offer. take=true posts a fixed half-bet side
// wager (locked decision 1); take=false declines. Either way the dealer then
// peeks.
func (g *Game) Insurance(take bool) error {
	if g.phase != PhaseInsurance {
		return ErrWrongPhase
	}
	if take {
		amt := g.bet / 2
		if !g.canAfford(amt) {
			return ErrIllegalAction
		}
		g.bankroll -= amt
		g.tookInsurance = true
		g.insuranceBet = amt
	}
	g.resolvePeek()
	return nil
}

// resolvePeek performs the dealer peek (on a ten or ace up-card) and routes to
// the correct next state: dealer blackjack resolution, player-natural payout,
// or the player's turn.
func (g *Game) resolvePeek() {
	up := g.dealer.Cards[0]
	peeks := up.Rank == Ace || up.isTenValue()
	if peeks && g.dealer.IsBlackjack() {
		g.resolveDealerBlackjack()
		return
	}

	// Dealer does not have blackjack. Insurance (if taken) is lost.
	if g.tookInsurance {
		g.insuranceNet = -g.insuranceBet
	}

	// Player natural auto-resolves, paid 3:2, with no turn (locked decision 2).
	if g.playerNatural {
		g.payPlayerNatural()
		return
	}

	g.phase = PhasePlayerTurn
	g.resolvePlayerTurn()
}

// resolveDealerBlackjack settles the round when the dealer has a natural: the
// player's non-natural hand loses, a player natural pushes, and insurance (if
// taken) pays 2:1.
func (g *Game) resolveDealerBlackjack() {
	g.dealerRevealed = true
	g.dealerBJ = true

	if g.tookInsurance {
		// Pays 2:1: profit is 2x the stake; stake+profit (3x) returns to bankroll.
		g.bankroll += g.insuranceBet * 3
		g.insuranceNet = g.insuranceBet * 2
	}

	h := g.hands[0]
	if h.hand.IsBlackjack() && !h.fromSplit {
		// Natural vs natural pushes: stake returned.
		g.bankroll += h.bet
		h.outcome = OutcomePush
		h.net = 0
	} else {
		h.outcome = OutcomeLose
		h.net = -h.bet
	}
	g.endRound()
}

// payPlayerNatural pays the player's natural 3:2 and ends the round.
func (g *Game) payPlayerNatural() {
	g.dealerRevealed = true
	h := g.hands[0]
	profit := h.bet * 3 / 2
	g.bankroll += h.bet + profit
	h.outcome = OutcomeBlackjack
	h.net = profit
	g.endRound()
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

// Double posts an additional bet equal to this hand's bet, draws exactly one
// card, and finishes the hand. Legal only on a two-card, non-split-ace hand
// with sufficient funds (locked decision 3; DAS allowed on non-ace splits).
func (g *Game) Double() error {
	if g.phase != PhasePlayerTurn {
		return ErrWrongPhase
	}
	if !g.CanAct(ActionDouble) {
		return ErrIllegalAction
	}
	h := g.hands[g.active]
	g.bankroll -= h.bet
	h.bet *= 2
	h.doubled = true
	h.hand.add(g.shoe.draw())
	h.stood = true
	g.resolvePlayerTurn()
	return nil
}

// Split splits an equal-rank pair into two hands, posting an additional bet.
// Aces are one-and-done (one card each, no hit/double/re-split); non-aces may
// re-split up to MaxHands. Legal only with sufficient funds (locked decision 3).
func (g *Game) Split() error {
	if g.phase != PhasePlayerTurn {
		return ErrWrongPhase
	}
	if !g.CanAct(ActionSplit) {
		return ErrIllegalAction
	}
	h := g.hands[g.active]
	isAce := h.hand.Cards[0].Rank == Ace

	// Post the additional bet.
	g.bankroll -= h.bet

	// Move the second card into a brand-new hand inserted right after this one.
	c1 := h.hand.Cards[1]
	h.hand.Cards = h.hand.Cards[:1]
	h.fromSplit = true
	h.splitAce = isAce
	h.stood = false

	newHand := &playerHand{
		hand:      Hand{Cards: []Card{c1}},
		bet:       h.bet,
		fromSplit: true,
		splitAce:  isAce,
		outcome:   OutcomePending,
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
// soft 17), and settles every hand.
func (g *Game) dealerTurnAndSettle() {
	g.dealerRevealed = true

	anyLive := false
	for _, h := range g.hands {
		if !h.hand.IsBust() {
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
	dealerBust := dv > 21

	for _, h := range g.hands {
		switch {
		case h.hand.IsBust():
			h.outcome = OutcomeLose
			h.net = -h.bet
		case h.hand.IsBlackjack() && !h.fromSplit:
			// Player natural beating a non-blackjack dealer, paid 3:2.
			profit := h.bet * 3 / 2
			g.bankroll += h.bet + profit
			h.outcome = OutcomeBlackjack
			h.net = profit
		case dealerBust:
			g.bankroll += h.bet * 2
			h.outcome = OutcomeWin
			h.net = h.bet
		default:
			pv := h.hand.Value()
			switch {
			case pv > dv:
				g.bankroll += h.bet * 2
				h.outcome = OutcomeWin
				h.net = h.bet
			case pv < dv:
				h.outcome = OutcomeLose
				h.net = -h.bet
			default:
				g.bankroll += h.bet
				h.outcome = OutcomePush
				h.net = 0
			}
		}
	}
	g.endRound()
}

// endRound moves to the round-over phase.
func (g *Game) endRound() {
	g.phase = PhaseRoundOver
}

// NextHand advances from a settled round to the next bet, or to game over if
// the bankroll can no longer cover the minimum bet. The pending bet is clamped
// to what the bankroll allows.
func (g *Game) NextHand() error {
	if g.phase != PhaseRoundOver {
		return ErrWrongPhase
	}
	if g.bankroll < MinBet {
		g.phase = PhaseGameOver
		return nil
	}
	if g.bet > g.bankroll {
		// Clamp down to the largest affordable multiple of BetIncrement.
		g.bet = (g.bankroll / BetIncrement) * BetIncrement
		if g.bet < MinBet {
			g.bet = MinBet
		}
	}
	g.phase = PhaseBetting
	return nil
}
