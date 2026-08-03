package engine

import "testing"

// stack builds a stacked shoe from a rank sequence (suit is irrelevant to the
// engine's logic). Deal order is: player, dealer-up, player, dealer-hole, then
// subsequent draws in order.
func stack(rs ...Rank) []Card {
	return cards(rs...)
}

func mustDeal(t *testing.T, g *Game) {
	t.Helper()
	if err := g.Deal(); err != nil {
		t.Fatalf("Deal: %v", err)
	}
}

func hasAction(g *Game, a Action) bool { return g.CanAct(a) }

// ---- H17 dealer behavior ----

func TestDealerHitsSoft17(t *testing.T) {
	// Player 10,9=19 stands. Dealer up 6, hole A => soft 17 must hit.
	// Next dealer card 10 => 6+A+10 = hard 17 => stands with 3 cards.
	g := NewGameWithShoe(1000, stack(Ten, Six, Nine, Ace, Ten))
	mustDeal(t, g)
	if g.Phase() != PhasePlayerTurn {
		t.Fatalf("phase = %v, want player-turn", g.Phase())
	}
	if err := g.Stand(); err != nil {
		t.Fatal(err)
	}
	d := g.Dealer()
	if len(d.Cards) != 3 {
		t.Fatalf("dealer should have hit soft 17: %d cards, want 3", len(d.Cards))
	}
	if d.Value != 17 {
		t.Fatalf("dealer value = %d, want 17", d.Value)
	}
}

func TestDealerStandsHard17(t *testing.T) {
	// Dealer 7,10 = hard 17 stands (2 cards).
	g := NewGameWithShoe(1000, stack(Ten, Seven, Nine, Ten))
	mustDeal(t, g)
	if err := g.Stand(); err != nil {
		t.Fatal(err)
	}
	if d := g.Dealer(); len(d.Cards) != 2 || d.Value != 17 {
		t.Fatalf("dealer = %d cards value %d, want 2 cards value 17", len(d.Cards), d.Value)
	}
}

func TestDealerStandsSoft18(t *testing.T) {
	// Dealer 7,A = soft 18 stands.
	g := NewGameWithShoe(1000, stack(Ten, Seven, Nine, Ace))
	mustDeal(t, g)
	if err := g.Stand(); err != nil {
		t.Fatal(err)
	}
	if d := g.Dealer(); len(d.Cards) != 2 || d.Value != 18 {
		t.Fatalf("dealer = %d cards value %d, want 2 cards value 18", len(d.Cards), d.Value)
	}
}

// ---- Legality & affordability gating ----

func TestSplitAndDoubleLegalWhenFunded(t *testing.T) {
	g := NewGameWithShoe(1000, stack(Eight, Six, Eight, Ten))
	mustDeal(t, g)
	if !hasAction(g, ActionSplit) {
		t.Error("split should be legal for 8,8 when funded")
	}
	if !hasAction(g, ActionDouble) {
		t.Error("double should be legal on first two cards when funded")
	}
}

func TestSplitDoubleIllegalWhenUnderfunded(t *testing.T) {
	// Bankroll 5: after escrowing the $3 main bet only $2 remains, which
	// cannot fund the extra $3 for split or double.
	g := NewGameWithShoe(5, stack(Eight, Six, Eight, Ten))
	mustDeal(t, g)
	if g.Bankroll() != 2 {
		t.Fatalf("post-escrow bankroll = %d, want 2", g.Bankroll())
	}
	if hasAction(g, ActionSplit) {
		t.Error("split must be illegal when unaffordable")
	}
	if hasAction(g, ActionDouble) {
		t.Error("double must be illegal when unaffordable")
	}
	if !hasAction(g, ActionHit) || !hasAction(g, ActionStand) {
		t.Error("hit and stand must always be legal on a live hand")
	}
}

func TestSplitIllegalOnUnequalRank(t *testing.T) {
	// K and Q are both ten-value but different ranks: no split.
	g := NewGameWithShoe(1000, stack(King, Six, Queen, Ten))
	mustDeal(t, g)
	if hasAction(g, ActionSplit) {
		t.Error("split must require equal rank, not equal value")
	}
	if !hasAction(g, ActionDouble) {
		t.Error("double should still be legal")
	}
}

func TestInsuranceLegalActionSet(t *testing.T) {
	// Dealer up Ace, hole not ten => insurance offered.
	g := NewGameWithShoe(1000, stack(Nine, Ace, Seven, Six))
	mustDeal(t, g)
	if g.Phase() != PhaseInsurance {
		t.Fatalf("phase = %v, want insurance", g.Phase())
	}
	acts := g.LegalActions()
	if len(acts) != 1 || acts[0] != ActionInsurance {
		t.Fatalf("legal actions = %v, want [insurance]", acts)
	}
}

// ---- Re-split cap and ace rules ----

func TestResplitToFourHandCap(t *testing.T) {
	// Player keeps drawing 8s: split three times to reach the 4-hand cap.
	g := NewGameWithShoe(1000, stack(
		Eight, Seven, Eight, Ten, // deal: player 8,8 ; dealer up 7 hole 10
		Eight, Eight, Eight, // each split's left hand redraws an 8
		Nine, Nine, Nine, // filler tail
	))
	mustDeal(t, g)
	for i := 0; i < 3; i++ {
		if !hasAction(g, ActionSplit) {
			t.Fatalf("split #%d should be legal", i+1)
		}
		if err := g.Split(); err != nil {
			t.Fatalf("split #%d: %v", i+1, err)
		}
	}
	if n := len(g.Player()); n != MaxHands {
		t.Fatalf("hands = %d, want %d", n, MaxHands)
	}
	if hasAction(g, ActionSplit) {
		t.Error("split must be illegal at the 4-hand cap")
	}
	if !hasAction(g, ActionHit) {
		t.Error("hit should still be legal")
	}
}

func TestSplitAcesOneCardEachNoResplit(t *testing.T) {
	// Player A,A ; dealer up 6 (no peek). Split aces => each gets exactly one
	// card and the round auto-resolves with no player turn. The left ace even
	// draws another ace but cannot re-split.
	g := NewGameWithShoe(1000, stack(
		Ace, Six, Ace, Ten, // player A,A ; dealer 6, hole 10
		Ace, Nine, // left ace draws A (A,A=12), right ace draws 9 (A,9=20)
		King, // dealer draws to bust: 6,10,K = 26
	))
	mustDeal(t, g)
	if !hasAction(g, ActionSplit) {
		t.Fatal("aces should be splittable")
	}
	if err := g.Split(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over (split aces take no turn)", g.Phase())
	}
	hands := g.Player()
	if len(hands) != 2 {
		t.Fatalf("hands = %d, want 2", len(hands))
	}
	for i, h := range hands {
		if len(h.Cards) != 2 {
			t.Errorf("hand %d has %d cards, want exactly 2 (one-and-done)", i, len(h.Cards))
		}
		if h.Blackjack {
			t.Errorf("hand %d must not count as blackjack (from split)", i)
		}
		if !h.SplitAce {
			t.Errorf("hand %d should be flagged split-ace", i)
		}
	}
	// The A,A hand is a soft 12 that was NOT re-split.
	if hands[0].Value != 12 {
		t.Errorf("left hand value = %d, want 12", hands[0].Value)
	}
}

func TestTenAceFromSplitIs21NotBlackjackPushesDealer21(t *testing.T) {
	// Player 10,10 split; each draws an Ace => 21 (not blackjack). Dealer makes
	// a non-natural 21 (7,7,7) => both hands push, not a 3:2 win.
	g := NewGameWithShoe(1000, stack(
		Ten, Seven, Ten, Seven, // player 10,10 ; dealer 7, hole 7
		Ace, Ace, // each ten hand draws an ace => 21
		Seven, // dealer hits 14 -> 21
	))
	mustDeal(t, g)
	if err := g.Split(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	hands := g.Player()
	if len(hands) != 2 {
		t.Fatalf("hands = %d, want 2", len(hands))
	}
	for i, h := range hands {
		if h.Value != 21 {
			t.Errorf("hand %d value = %d, want 21", i, h.Value)
		}
		if h.Blackjack {
			t.Errorf("hand %d must not be blackjack (from split)", i)
		}
		if h.Outcome != OutcomePush {
			t.Errorf("hand %d outcome = %v, want push", i, h.Outcome)
		}
		if h.Net != 0 {
			t.Errorf("hand %d net = %d, want 0", i, h.Net)
		}
	}
	if g.Bankroll() != 1000 {
		t.Errorf("bankroll = %d, want 1000 (two pushes)", g.Bankroll())
	}
}

// ---- Player natural auto-resolution ----

func TestPlayerNaturalPaid3To2NoTurn(t *testing.T) {
	// Player A,K = natural; dealer up 6 (no peek, no blackjack). Auto paid 3:2.
	g := NewGameWithShoe(1000, stack(Ace, Six, King, Nine))
	if err := g.SetBet(100); err != nil {
		t.Fatal(err)
	}
	mustDeal(t, g)
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over (natural auto-resolves)", g.Phase())
	}
	if acts := g.LegalActions(); len(acts) != 0 {
		t.Fatalf("legal actions = %v, want none (no player turn)", acts)
	}
	h := g.Player()[0]
	if h.Outcome != OutcomeBlackjack {
		t.Fatalf("outcome = %v, want blackjack", h.Outcome)
	}
	if h.Net != 150 {
		t.Fatalf("net = %d, want 150 (3:2 on 100)", h.Net)
	}
	if g.Bankroll() != 1150 {
		t.Fatalf("bankroll = %d, want 1150", g.Bankroll())
	}
}

func TestNaturalVsNaturalPush(t *testing.T) {
	// Player A,K natural; dealer 10,A natural (up 10 => peek). Push.
	g := NewGameWithShoe(1000, stack(Ace, Ten, King, Ace))
	if err := g.SetBet(100); err != nil {
		t.Fatal(err)
	}
	mustDeal(t, g)
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	if !g.DealerHadBlackjack() {
		t.Fatal("dealer should have blackjack")
	}
	h := g.Player()[0]
	if h.Outcome != OutcomePush {
		t.Fatalf("outcome = %v, want push", h.Outcome)
	}
	if g.Bankroll() != 1000 {
		t.Fatalf("bankroll = %d, want 1000 (push returns stake)", g.Bankroll())
	}
}

// ---- Payout math ----

func TestPayoutEvenMoneyWin(t *testing.T) {
	// Player 10,K=20 vs dealer 7,10=17.
	g := NewGameWithShoe(1000, stack(Ten, Seven, King, Ten))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if err := g.Stand(); err != nil {
		t.Fatal(err)
	}
	h := g.Player()[0]
	if h.Outcome != OutcomeWin || h.Net != 100 {
		t.Fatalf("outcome=%v net=%d, want win +100", h.Outcome, h.Net)
	}
	if g.Bankroll() != 1100 {
		t.Fatalf("bankroll = %d, want 1100", g.Bankroll())
	}
}

func TestPayoutPush(t *testing.T) {
	// Player 20 vs dealer 20 (up 10 => peek, no BJ).
	g := NewGameWithShoe(1000, stack(Ten, Ten, King, King))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if err := g.Stand(); err != nil {
		t.Fatal(err)
	}
	h := g.Player()[0]
	if h.Outcome != OutcomePush || h.Net != 0 {
		t.Fatalf("outcome=%v net=%d, want push 0", h.Outcome, h.Net)
	}
	if g.Bankroll() != 1000 {
		t.Fatalf("bankroll = %d, want 1000", g.Bankroll())
	}
}

func TestPayoutDouble(t *testing.T) {
	// Player 5,6=11 doubles, draws 10 => 21 vs dealer 7,10=17.
	g := NewGameWithShoe(1000, stack(Five, Seven, Six, Ten, Ten))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if !hasAction(g, ActionDouble) {
		t.Fatal("double should be legal")
	}
	if err := g.Double(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over after double", g.Phase())
	}
	h := g.Player()[0]
	if !h.Doubled {
		t.Error("hand should be flagged doubled")
	}
	if h.Bet != 200 {
		t.Errorf("hand bet = %d, want 200", h.Bet)
	}
	if h.Outcome != OutcomeWin || h.Net != 200 {
		t.Fatalf("outcome=%v net=%d, want win +200", h.Outcome, h.Net)
	}
	if g.Bankroll() != 1200 {
		t.Fatalf("bankroll = %d, want 1200", g.Bankroll())
	}
}

func TestPayoutInsurancePaid2To1OnDealerBlackjack(t *testing.T) {
	// Dealer A up, 10 hole => natural. Player 9,7=16 takes insurance.
	g := NewGameWithShoe(1000, stack(Nine, Ace, Seven, Ten))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if g.Phase() != PhaseInsurance {
		t.Fatalf("phase = %v, want insurance", g.Phase())
	}
	if g.Bankroll() != 900 {
		t.Fatalf("post-escrow bankroll = %d, want 900", g.Bankroll())
	}
	if err := g.Insurance(true); err != nil {
		t.Fatal(err)
	}
	if !g.DealerHadBlackjack() {
		t.Fatal("dealer should have blackjack")
	}
	if g.InsuranceNet() != 100 {
		t.Fatalf("insurance net = %d, want +100 (2:1 on 50)", g.InsuranceNet())
	}
	h := g.Player()[0]
	if h.Outcome != OutcomeLose {
		t.Fatalf("main hand outcome = %v, want lose", h.Outcome)
	}
	// Main -100 offset by insurance +100 => bankroll back to start.
	if g.Bankroll() != 1000 {
		t.Fatalf("bankroll = %d, want 1000", g.Bankroll())
	}
}

func TestInsuranceLostWhenDealerHasNoBlackjack(t *testing.T) {
	// Dealer A up, 6 hole => no blackjack. Player takes insurance, loses it,
	// and the main hand continues.
	g := NewGameWithShoe(1000, stack(Nine, Ace, Seven, Six, Ten))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if err := g.Insurance(true); err != nil {
		t.Fatal(err)
	}
	if g.DealerHadBlackjack() {
		t.Fatal("dealer should not have blackjack")
	}
	if g.Phase() != PhasePlayerTurn {
		t.Fatalf("phase = %v, want player-turn (hand continues)", g.Phase())
	}
	if g.InsuranceNet() != -50 {
		t.Fatalf("insurance net = %d, want -50 (lost)", g.InsuranceNet())
	}
	// Escrow 100 + insurance 50 removed from 1000.
	if g.Bankroll() != 850 {
		t.Fatalf("bankroll = %d, want 850", g.Bankroll())
	}
	if !hasAction(g, ActionHit) || !hasAction(g, ActionStand) {
		t.Error("player should still be able to act")
	}
}

// ---- Multi-hand mixed settlement ----

func TestMixedMultiHandSettlement(t *testing.T) {
	// Player 8,8 split into three hands vs dealer 7,10=17.
	//   H0: 8,3,10 = 21  => win
	//   H2: 8,9    = 17  => push
	//   H1: 8,10,10= bust => lose
	g := NewGameWithShoe(1000, stack(
		Eight, Seven, Eight, Ten, // player 8,8 ; dealer 7, hole 10
		Eight, // H0 redraws 8 (split again)
		Three, // H0 second card after 2nd split => 8,3
		Ten,   // H0 hit => 21
		Nine,  // H2 second card => 8,9 = 17
		Ten,   // H1 second card => 8,10 = 18
		Ten,   // H1 hit => 28 bust
	))
	mustDeal(t, g)
	if err := g.Split(); err != nil { // first split
		t.Fatal(err)
	}
	if err := g.Split(); err != nil { // second split (H0 redrew an 8)
		t.Fatal(err)
	}
	// Active hand is H0 = 8,3.
	if err := g.Hit(); err != nil { // H0 -> 21, auto-stands, advance to H2
		t.Fatal(err)
	}
	if err := g.Stand(); err != nil { // H2 = 17 stands, advance to H1
		t.Fatal(err)
	}
	if err := g.Hit(); err != nil { // H1 -> bust, ends player turn
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	hands := g.Player()
	if len(hands) != 3 {
		t.Fatalf("hands = %d, want 3", len(hands))
	}
	want := []Outcome{OutcomeWin, OutcomePush, OutcomeLose}
	for i, w := range want {
		if hands[i].Outcome != w {
			t.Errorf("hand %d outcome = %v, want %v", i, hands[i].Outcome, w)
		}
	}
	// +3 (win) + 0 (push) - 3 (lose) = net 0.
	if g.Bankroll() != 1000 {
		t.Errorf("bankroll = %d, want 1000", g.Bankroll())
	}
}

// ---- Game over ----

func TestBankrollGameOver(t *testing.T) {
	// Bankroll 5, bet 3. Player 16 loses to dealer 19; bankroll -> 2 (< min).
	g := NewGameWithShoe(5, stack(Ten, Nine, Six, King))
	mustDeal(t, g)
	if err := g.Stand(); err != nil {
		t.Fatal(err)
	}
	if g.Bankroll() != 2 {
		t.Fatalf("bankroll = %d, want 2", g.Bankroll())
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseGameOver {
		t.Fatalf("phase = %v, want game-over (bankroll below min bet)", g.Phase())
	}
}

func TestNextHandContinuesWhenSolvent(t *testing.T) {
	g := NewGameWithShoe(1000, stack(Ten, Seven, King, Ten))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if err := g.Stand(); err != nil {
		t.Fatal(err)
	}
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseBetting {
		t.Fatalf("phase = %v, want betting", g.Phase())
	}
}

// ---- Cut-card reshuffle ----

func TestCutCardReshuffleTrigger(t *testing.T) {
	g := NewGame(7)
	// Force the shoe past the cut card.
	g.shoe.pos = CutCardPosition
	if !g.CutCardReached() {
		t.Fatal("cut card should be reached")
	}
	mustDeal(t, g)
	if !g.DidReshuffle() {
		t.Fatal("Deal should have reshuffled after cut card")
	}
	if g.CutCardReached() {
		t.Fatal("shoe should be fresh after reshuffle")
	}
	// Fresh 312-card shoe minus the 4 dealt cards.
	if g.CardsRemaining() != NumDecks*52-4 {
		t.Fatalf("remaining = %d, want %d", g.CardsRemaining(), NumDecks*52-4)
	}
}

func TestNoReshuffleBeforeCutCard(t *testing.T) {
	g := NewGame(7)
	mustDeal(t, g)
	if g.DidReshuffle() {
		t.Fatal("should not reshuffle before cut card")
	}
}
