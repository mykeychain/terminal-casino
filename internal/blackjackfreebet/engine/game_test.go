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

func TestOwnMoneyDoubleIllegalWhenUnderfunded(t *testing.T) {
	// Player 8,8 = hard 16: a double here is an OWN-MONEY double (not a free
	// 9/10/11). Bankroll 5 leaves only $2 after the $3 escrow, so the extra real
	// bet cannot be afforded and the double is illegal. The 8,8 split, however,
	// is FREE and stays legal even when broke (see TestFreeSplitLegalWhenBroke).
	g := NewGameWithShoe(5, stack(Eight, Six, Eight, Ten))
	mustDeal(t, g)
	if g.Bankroll() != 2 {
		t.Fatalf("post-escrow bankroll = %d, want 2", g.Bankroll())
	}
	if hasAction(g, ActionDouble) {
		t.Error("own-money double on hard 16 must be illegal when unaffordable")
	}
	if !hasAction(g, ActionSplit) {
		t.Error("free split of 8,8 must stay legal even when underfunded")
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

func TestFreeDoubleOnElevenPaysDoubleAtNoExtraCost(t *testing.T) {
	// Player 5,6 = hard 11 => a FREE double. Draws 10 => 21 vs dealer 7,10=17.
	// The house posts a $100 free bet: no extra real money is escrowed (the real
	// bet stays $100), yet the win pays as though $200 were wagered.
	g := NewGameWithShoe(1000, stack(Five, Seven, Six, Ten, Ten))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if !g.DoubleIsFree() {
		t.Fatal("double on hard 11 should be free")
	}
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
	if h.Bet != 100 {
		t.Errorf("real bet = %d, want 100 (a free double adds no real money)", h.Bet)
	}
	if h.Free != 100 {
		t.Errorf("free bet = %d, want 100", h.Free)
	}
	if h.Outcome != OutcomeWin || h.Net != 200 {
		t.Fatalf("outcome=%v net=%d, want win +200 (paid on bet+free)", h.Outcome, h.Net)
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

func TestMixedMultiHandFreeSplitSettlement(t *testing.T) {
	// Player 8,8 FREE-split into three hands vs dealer 7,10=17.
	//   H0: 8,3,10 = 21   => win   (the original REAL bet)
	//   H2: 8,9    = 17   => push  (a free bet)
	//   H1: 8,10,10= bust => lose  (a free bet — costs nothing)
	// Only the original hand carries real money; both split siblings ride free
	// bets, so the busted hand loses $0 and the whole round nets +$3.
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
	if err := g.Split(); err != nil { // first split (free)
		t.Fatal(err)
	}
	if err := g.Split(); err != nil { // second split (H0 redrew an 8; free)
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
	wantOutcome := []Outcome{OutcomeWin, OutcomePush, OutcomeLose}
	wantNet := []int{3, 0, 0} // the free-bet loss (H1) costs nothing
	for i := range wantOutcome {
		if hands[i].Outcome != wantOutcome[i] {
			t.Errorf("hand %d outcome = %v, want %v", i, hands[i].Outcome, wantOutcome[i])
		}
		if hands[i].Net != wantNet[i] {
			t.Errorf("hand %d net = %d, want %d", i, hands[i].Net, wantNet[i])
		}
	}
	// The original hand is the only real money; both split siblings are free bets.
	if hands[0].Bet != 3 || hands[0].Free != 0 {
		t.Errorf("H0 bet/free = %d/%d, want 3/0 (original real hand)", hands[0].Bet, hands[0].Free)
	}
	if hands[1].Bet != 0 || hands[1].Free != 3 {
		t.Errorf("H2 bet/free = %d/%d, want 0/3 (free split)", hands[1].Bet, hands[1].Free)
	}
	if hands[2].Bet != 0 || hands[2].Free != 3 {
		t.Errorf("H1 bet/free = %d/%d, want 0/3 (free split)", hands[2].Bet, hands[2].Free)
	}
	// escrow 3 -> 997; only H0 wins (+6: $3 stake back + $3 profit); the free
	// hands net 0 => 1003.
	if g.Bankroll() != 1003 {
		t.Errorf("bankroll = %d, want 1003 (only the real hand wins; the free loss is free)", g.Bankroll())
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

// ---- Free Bet: free vs own-money doubles ----

func TestDoubleIsFreeClassification(t *testing.T) {
	// Hard 11 (5,6) => free double.
	g := NewGameWithShoe(1000, stack(Five, Nine, Six, Ten))
	mustDeal(t, g)
	if !g.DoubleIsFree() {
		t.Error("hard 11 should be a free double")
	}
	// Hard 8 (3,5) => own-money double (offered when funded, but not free).
	g = NewGameWithShoe(1000, stack(Three, Nine, Five, Ten))
	mustDeal(t, g)
	if g.DoubleIsFree() {
		t.Error("hard 8 must NOT be a free double")
	}
	if !hasAction(g, ActionDouble) {
		t.Error("own-money double on hard 8 should be legal when funded")
	}
	// Soft 17 (A,6) => own-money double, not free.
	g = NewGameWithShoe(1000, stack(Ace, Nine, Six, Ten))
	mustDeal(t, g)
	if g.DoubleIsFree() {
		t.Error("soft 17 must NOT be a free double")
	}
}

func TestFreeDoubleLossOnlyLosesRealStake(t *testing.T) {
	// Player 6,4 = hard 10 => free double; draws a 2 => 12, loses to dealer 19.
	// The free bet vanishes at no cost: only the $100 real stake is lost.
	g := NewGameWithShoe(1000, stack(Six, Ten, Four, Nine, Two))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if !g.DoubleIsFree() {
		t.Fatal("hard 10 should be a free double")
	}
	if err := g.Double(); err != nil {
		t.Fatal(err)
	}
	h := g.Player()[0]
	if h.Bet != 100 || h.Free != 100 {
		t.Errorf("bet/free = %d/%d, want 100/100", h.Bet, h.Free)
	}
	if h.Outcome != OutcomeLose || h.Net != -100 {
		t.Fatalf("outcome=%v net=%d, want lose -100 (free bet costs nothing)", h.Outcome, h.Net)
	}
	if g.Bankroll() != 900 {
		t.Fatalf("bankroll = %d, want 900", g.Bankroll())
	}
}

func TestOwnMoneyDoubleOnHard8(t *testing.T) {
	// Player 3,5 = hard 8 => own-money double: an extra real $100 is escrowed and
	// the bet becomes $200. Draws 10 => 18 beats dealer 17.
	g := NewGameWithShoe(1000, stack(Three, Ten, Five, Seven, Ten))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if g.DoubleIsFree() {
		t.Fatal("hard 8 must not be a free double")
	}
	if err := g.Double(); err != nil {
		t.Fatal(err)
	}
	h := g.Player()[0]
	if h.Bet != 200 || h.Free != 0 {
		t.Errorf("bet/free = %d/%d, want 200/0 (own-money double)", h.Bet, h.Free)
	}
	if h.Outcome != OutcomeWin || h.Net != 200 {
		t.Fatalf("outcome=%v net=%d, want win +200", h.Outcome, h.Net)
	}
	if g.Bankroll() != 1200 {
		t.Fatalf("bankroll = %d, want 1200", g.Bankroll())
	}
}

// ---- Free Bet: free vs own-money splits ----

func TestFreeSplitLegalWhenBroke(t *testing.T) {
	// Bankroll 3: after the $3 escrow, nothing remains. An 8,8 split is FREE, so
	// it is legal anyway; the new hand rides a $3 free bet. Both hands then lose,
	// but only the original real bet is at risk.
	g := NewGameWithShoe(3, stack(Seven, Nine, Seven, Ten, Five, Four))
	mustDeal(t, g)
	if g.Bankroll() != 0 {
		t.Fatalf("post-escrow bankroll = %d, want 0", g.Bankroll())
	}
	if !g.SplitIsFree() {
		t.Error("splitting 7,7 should be free")
	}
	if !hasAction(g, ActionSplit) {
		t.Fatal("a free split must be legal even with no money left")
	}
	if err := g.Split(); err != nil {
		t.Fatal(err)
	}
	if g.Bankroll() != 0 {
		t.Errorf("bankroll = %d, want 0 (a free split escrows nothing)", g.Bankroll())
	}
	if err := g.Stand(); err != nil { // H0 = 7,5 = 12 stands
		t.Fatal(err)
	}
	if err := g.Stand(); err != nil { // H1 = 7,4 = 11 stands
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	hands := g.Player()
	if hands[0].Bet != 3 || hands[0].Free != 0 {
		t.Errorf("H0 bet/free = %d/%d, want 3/0 (original real hand)", hands[0].Bet, hands[0].Free)
	}
	if hands[1].Bet != 0 || hands[1].Free != 3 {
		t.Errorf("H1 bet/free = %d/%d, want 0/3 (free split)", hands[1].Bet, hands[1].Free)
	}
	// Both lose to dealer 19, but the free hand's loss costs nothing.
	if hands[0].Net != -3 || hands[1].Net != 0 {
		t.Errorf("nets = %d/%d, want -3/0", hands[0].Net, hands[1].Net)
	}
	if g.Bankroll() != 0 {
		t.Errorf("bankroll = %d, want 0", g.Bankroll())
	}
}

func TestOwnMoneySplitTensRequiresFunds(t *testing.T) {
	// A ten-value pair (K,K) is an OWN-MONEY split, so it is gated by bankroll.
	// Broke: illegal.
	g := NewGameWithShoe(3, stack(King, Nine, King, Ten))
	mustDeal(t, g)
	if g.Bankroll() != 0 {
		t.Fatalf("post-escrow bankroll = %d, want 0", g.Bankroll())
	}
	if g.SplitIsFree() {
		t.Error("splitting tens must NOT be free")
	}
	if hasAction(g, ActionSplit) {
		t.Error("own-money split of tens must be illegal when unaffordable")
	}
	// Funded: legal, and it escrows a real extra bet (both hands stay real money).
	g = NewGameWithShoe(1000, stack(King, Nine, King, Ten, Five, Five))
	mustDeal(t, g)
	if g.SplitIsFree() {
		t.Error("splitting tens must NOT be free")
	}
	if !hasAction(g, ActionSplit) {
		t.Fatal("own-money split of tens should be legal when funded")
	}
	before := g.Bankroll() // 997 after the $3 escrow
	if err := g.Split(); err != nil {
		t.Fatal(err)
	}
	if g.Bankroll() != before-MinBet {
		t.Errorf("bankroll = %d, want %d (own-money split escrows a real bet)", g.Bankroll(), before-MinBet)
	}
	for i, h := range g.Player() {
		if h.Bet != MinBet || h.Free != 0 {
			t.Errorf("hand %d bet/free = %d/%d, want %d/0 (all real money)", i, h.Bet, h.Free, MinBet)
		}
	}
}

func TestSplitAcesFreeEvenWhenBroke(t *testing.T) {
	// Aces are a non-ten pair, so they FREE-split even with no money left. Each
	// ace takes exactly one card and the round auto-resolves (one-and-done).
	g := NewGameWithShoe(3, stack(Ace, Nine, Ace, Ten, Five, Four))
	mustDeal(t, g)
	if g.Bankroll() != 0 {
		t.Fatalf("post-escrow bankroll = %d, want 0", g.Bankroll())
	}
	if !g.SplitIsFree() {
		t.Error("splitting aces should be free")
	}
	if !hasAction(g, ActionSplit) {
		t.Fatal("free ace split must be legal even when broke")
	}
	if err := g.Split(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over (split aces are one-and-done)", g.Phase())
	}
	hands := g.Player()
	if len(hands) != 2 {
		t.Fatalf("hands = %d, want 2", len(hands))
	}
	for i, h := range hands {
		if !h.SplitAce {
			t.Errorf("hand %d should be flagged split-ace", i)
		}
		if len(h.Cards) != 2 {
			t.Errorf("hand %d has %d cards, want 2 (one-and-done)", i, len(h.Cards))
		}
	}
	if hands[0].Bet != 3 || hands[0].Free != 0 {
		t.Errorf("H0 bet/free = %d/%d, want 3/0 (original real hand)", hands[0].Bet, hands[0].Free)
	}
	if hands[1].Bet != 0 || hands[1].Free != 3 {
		t.Errorf("H1 bet/free = %d/%d, want 0/3 (free split)", hands[1].Bet, hands[1].Free)
	}
}

func TestFreeDoubleAfterFreeSplit(t *testing.T) {
	// Player 4,4 free-splits; each 4 draws a 7 to make hard 11 and FREE-doubles
	// into a 21. Both beat dealer 19. On a $10 bet, no extra real money is ever
	// escrowed yet the round pays +$40 — the free-bet dream.
	g := NewGameWithShoe(1000, stack(
		Four, Nine, Four, Ten, // player 4,4 ; dealer 9, hole 10 = 19
		Seven, Ten, // H0: 4 -> 4,7=11, free double -> +10 => 21
		Seven, Ten, // H1: 4 -> 4,7=11, free double -> +10 => 21
	))
	_ = g.SetBet(10)
	mustDeal(t, g)
	if err := g.Split(); err != nil { // free split of 4,4
		t.Fatal(err)
	}
	if !g.DoubleIsFree() {
		t.Fatal("H0 = 4,7 = 11 should offer a free double")
	}
	if err := g.Double(); err != nil { // free double H0
		t.Fatal(err)
	}
	if !g.DoubleIsFree() {
		t.Fatal("H1 = 4,7 = 11 should offer a free double")
	}
	if err := g.Double(); err != nil { // free double H1
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	hands := g.Player()
	// H0 is the original real hand: $10 real + $10 free from the double.
	if hands[0].Bet != 10 || hands[0].Free != 10 {
		t.Errorf("H0 bet/free = %d/%d, want 10/10", hands[0].Bet, hands[0].Free)
	}
	// H1 rode a free split ($10 free) then free-doubled (+$10 free) = $20 free.
	if hands[1].Bet != 0 || hands[1].Free != 20 {
		t.Errorf("H1 bet/free = %d/%d, want 0/20", hands[1].Bet, hands[1].Free)
	}
	for i, h := range hands {
		if h.Outcome != OutcomeWin {
			t.Errorf("hand %d outcome = %v, want win", i, h.Outcome)
		}
		if !h.Doubled {
			t.Errorf("hand %d should be flagged doubled", i)
		}
	}
	if hands[0].Net != 20 || hands[1].Net != 20 {
		t.Errorf("nets = %d/%d, want 20/20", hands[0].Net, hands[1].Net)
	}
	// escrow 10 -> 990; H0 win +30 (10 stake + 20 profit) => 1020; H1 win +20 => 1040.
	if g.Bankroll() != 1040 {
		t.Errorf("bankroll = %d, want 1040", g.Bankroll())
	}
}

// ---- Free Bet: dealer Push 22 ----

func TestDealerPush22PushesNonBustHands(t *testing.T) {
	// Two spots (20 and 18) vs a dealer that draws to 22. Under Push 22 the
	// dealer does not bust — every non-busted hand pushes.
	g := NewGameWithShoe(1000, stack(
		Ten, Ten, // first card each
		Ten,        // dealer up
		Ten, Eight, // second card each => s0=20, s1=18
		Six, // dealer hole => 10,6 = 16
		Six, // dealer hits => 22
	))
	setupSpots(t, g, 100, 100)
	mustDeal(t, g)
	if err := g.Stand(); err != nil { // s0 = 20
		t.Fatal(err)
	}
	if err := g.Stand(); err != nil { // s1 = 18
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	if !g.DealerPush22() {
		t.Fatal("DealerPush22 should be true on a dealer 22")
	}
	if d := g.Dealer(); d.Value != 22 || !d.Push22 {
		t.Fatalf("dealer view value/push22 = %d/%v, want 22/true", d.Value, d.Push22)
	}
	for i, h := range g.Player() {
		if h.Outcome != OutcomePush || h.Net != 0 {
			t.Errorf("hand %d outcome/net = %v/%d, want push/0", i, h.Outcome, h.Net)
		}
	}
	if g.Bankroll() != 1000 {
		t.Errorf("bankroll = %d, want 1000 (both push)", g.Bankroll())
	}
}

func TestDealerPush22PlayerBustStillLoses(t *testing.T) {
	// s0 busts, s1 stands 19 so the dealer plays out to 22. The busted hand still
	// LOSES against dealer 22; only the non-busted hand pushes.
	g := NewGameWithShoe(1000, stack(
		Ten, Ten, // first card each
		Ten,       // dealer up
		Six, Nine, // second card each => s0=16, s1=19
		Six, // dealer hole => 10,6 = 16
		Ten, // s0 hits => 26 bust
		Six, // dealer hits => 22
	))
	setupSpots(t, g, 100, 100)
	mustDeal(t, g)
	if err := g.Hit(); err != nil { // s0 16 -> 26 bust
		t.Fatal(err)
	}
	if err := g.Stand(); err != nil { // s1 stands 19
		t.Fatal(err)
	}
	if !g.DealerPush22() {
		t.Fatal("dealer should be 22")
	}
	hands := g.Player()
	if hands[0].Outcome != OutcomeLose || hands[0].Net != -100 {
		t.Errorf("s0 (bust) outcome/net = %v/%d, want lose/-100", hands[0].Outcome, hands[0].Net)
	}
	if hands[1].Outcome != OutcomePush || hands[1].Net != 0 {
		t.Errorf("s1 outcome/net = %v/%d, want push/0", hands[1].Outcome, hands[1].Net)
	}
	// escrow 200 -> 800; s0 loses (+0), s1 pushes (+100) => 900.
	if g.Bankroll() != 900 {
		t.Errorf("bankroll = %d, want 900", g.Bankroll())
	}
}

func TestDealerPush22PlayerBlackjackStillWins(t *testing.T) {
	// s0 is a natural (paid 3:2 at the peek, before the dealer draws); s1 = 20
	// keeps the dealer live and it draws to 22. The natural still WINS while the
	// 20 pushes — a player blackjack always beats a dealer 22.
	g := NewGameWithShoe(1000, stack(
		Ace, Ten, // first card each
		Six,       // dealer up (no peek)
		King, Ten, // second card each => s0 = A,K natural, s1 = 20
		Ten, // dealer hole => 6,10 = 16
		Six, // dealer hits => 22
	))
	setupSpots(t, g, 100, 10)
	mustDeal(t, g)
	// s0's natural is already resolved; s1 still plays.
	if g.ActiveHandIndex() != 1 {
		t.Fatalf("active = %d, want 1 (natural resolved)", g.ActiveHandIndex())
	}
	if err := g.Stand(); err != nil { // s1 stands 20
		t.Fatal(err)
	}
	if !g.DealerPush22() {
		t.Fatal("dealer should be 22")
	}
	hands := g.Player()
	if hands[0].Outcome != OutcomeBlackjack || hands[0].Net != 150 {
		t.Errorf("s0 outcome/net = %v/%d, want blackjack/+150", hands[0].Outcome, hands[0].Net)
	}
	if hands[1].Outcome != OutcomePush || hands[1].Net != 0 {
		t.Errorf("s1 outcome/net = %v/%d, want push/0", hands[1].Outcome, hands[1].Net)
	}
	// escrow 110 -> 890; natural +250 (stake 100 + 150) => 1140; s1 push +10 => 1150.
	if g.Bankroll() != 1150 {
		t.Errorf("bankroll = %d, want 1150", g.Bankroll())
	}
}

func TestDealer23BustsPlayerWins(t *testing.T) {
	// Only 22 pushes: a dealer 23 is an ordinary bust and the player wins.
	g := NewGameWithShoe(1000, stack(Ten, Ten, Eight, Six, Seven))
	_ = g.SetBet(100)
	mustDeal(t, g)
	if err := g.Stand(); err != nil { // player 18
		t.Fatal(err)
	}
	if g.DealerPush22() {
		t.Fatal("a dealer 23 must NOT be a Push 22")
	}
	if d := g.Dealer(); d.Value != 23 {
		t.Fatalf("dealer value = %d, want 23", d.Value)
	}
	h := g.Player()[0]
	if h.Outcome != OutcomeWin || h.Net != 100 {
		t.Fatalf("outcome=%v net=%d, want win +100 (dealer busts on 23)", h.Outcome, h.Net)
	}
	if g.Bankroll() != 1100 {
		t.Fatalf("bankroll = %d, want 1100", g.Bankroll())
	}
}
