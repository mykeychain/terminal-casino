package engine

import "testing"

// Multi-spot deal order (N opened hands, left to right):
//   1. one card to each spot (spot 0 first),
//   2. dealer up-card,
//   3. a second card to each spot (spot 0 first),
//   4. dealer hole card,
//   5. all subsequent draws (player hits, then dealer) in order.
// Stacked shoes below are built in exactly that order.

// setupSpots opens n spots with the given bets. The first bet reuses spot 0;
// each additional bet opens a new spot. It fails the test on any engine error.
func setupSpots(t *testing.T, g *Game, bets ...int) {
	t.Helper()
	for i, b := range bets {
		if i > 0 {
			if err := g.AddSpot(); err != nil {
				t.Fatalf("AddSpot #%d: %v", i, err)
			}
		}
		if err := g.SetSpotBet(i, b); err != nil {
			t.Fatalf("SetSpotBet(%d, %d): %v", i, b, err)
		}
	}
}

// ---- Deal order & summed escrow ----

func TestMultiSpotDealOrderAndEscrow(t *testing.T) {
	// 3 spots, bets 10/20/30 (sum 60). Deal order interleaves L->R.
	//   s0c1, s1c1, s2c1, Dup, s0c2, s1c2, s2c2, Dhole
	g := NewGameWithShoe(1000, stack(
		Ten, Ten, Ten, // first card to each spot
		Six,                // dealer up
		Nine, Eight, Seven, // second card to each spot
		Ten, // dealer hole
	))
	setupSpots(t, g, 10, 20, 30)
	if g.NumSpots() != 3 {
		t.Fatalf("NumSpots = %d, want 3", g.NumSpots())
	}
	mustDeal(t, g)

	if g.Bankroll() != 1000-60 {
		t.Fatalf("post-escrow bankroll = %d, want %d (summed stake)", g.Bankroll(), 1000-60)
	}
	hands := g.Player()
	if len(hands) != 3 {
		t.Fatalf("hands = %d, want 3", len(hands))
	}
	wantValues := []int{19, 18, 17}
	wantBets := []int{10, 20, 30}
	for i, h := range hands {
		if h.Spot != i {
			t.Errorf("hand %d spot = %d, want %d", i, h.Spot, i)
		}
		if h.Value != wantValues[i] {
			t.Errorf("hand %d value = %d, want %d", i, h.Value, wantValues[i])
		}
		if h.Bet != wantBets[i] {
			t.Errorf("hand %d bet = %d, want %d", i, h.Bet, wantBets[i])
		}
		if len(h.Cards) != 2 {
			t.Errorf("hand %d has %d cards, want 2", i, len(h.Cards))
		}
	}
	if up := g.Dealer().Upcard; up.Rank != Six {
		t.Errorf("dealer up = %v, want 6", up)
	}
}

// ---- Mixed per-hand settlement across separate spots ----

func TestMultiSpotMixedSettlement(t *testing.T) {
	// 3 spots vs dealer 10,7 = 17.
	//   s0 = 10,9 = 19 => win
	//   s1 = 10,7 = 17 => push
	//   s2 = 10,6 = 16 => lose
	g := NewGameWithShoe(1000, stack(
		Ten, Ten, Ten, // first card each
		Ten,               // dealer up
		Nine, Seven, Six,  // second card each
		Seven, // dealer hole => 10,7 = 17 stands
	))
	setupSpots(t, g, 10, 10, 10)
	mustDeal(t, g)
	if g.Phase() != PhasePlayerTurn {
		t.Fatalf("phase = %v, want player-turn", g.Phase())
	}
	// Stand all three spots left to right.
	for i := 0; i < 3; i++ {
		if err := g.Stand(); err != nil {
			t.Fatalf("stand spot %d: %v", i, err)
		}
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	hands := g.Player()
	wantOutcome := []Outcome{OutcomeWin, OutcomePush, OutcomeLose}
	wantNet := []int{10, 0, -10}
	for i := range hands {
		if hands[i].Outcome != wantOutcome[i] {
			t.Errorf("hand %d outcome = %v, want %v", i, hands[i].Outcome, wantOutcome[i])
		}
		if hands[i].Net != wantNet[i] {
			t.Errorf("hand %d net = %d, want %d", i, hands[i].Net, wantNet[i])
		}
	}
	if g.Bankroll() != 1000 {
		t.Errorf("bankroll = %d, want 1000 (win + push + lose nets 0)", g.Bankroll())
	}
}

// ---- Per-hand natural paid 3:2 while another spot still plays ----

func TestMultiSpotNaturalPaidWhileOtherPlays(t *testing.T) {
	// 2 spots, dealer up 6 (no peek).
	//   s0 = A,K = natural => auto-paid 3:2 at peek, no turn
	//   s1 = 10,6 = 16 => plays; dealer 6,10=16 hits 10 => 26 bust => s1 wins
	g := NewGameWithShoe(1000, stack(
		Ace, Ten, // first card each
		Six,      // dealer up
		King, Six, // second card each
		Ten, // dealer hole => 6,10 = 16
		Ten, // dealer hit => bust
	))
	setupSpots(t, g, 100, 10)
	mustDeal(t, g)

	// s0's natural is resolved immediately; s1 is still in play.
	if g.Phase() != PhasePlayerTurn {
		t.Fatalf("phase = %v, want player-turn (s1 still plays)", g.Phase())
	}
	if g.ActiveHandIndex() != 1 {
		t.Fatalf("active = %d, want 1 (s0 natural resolved)", g.ActiveHandIndex())
	}
	hands := g.Player()
	if hands[0].Outcome != OutcomeBlackjack {
		t.Fatalf("s0 outcome = %v, want blackjack", hands[0].Outcome)
	}
	if hands[0].Net != 150 {
		t.Fatalf("s0 net = %d, want 150 (3:2 on 100)", hands[0].Net)
	}
	if hands[1].Outcome != OutcomePending {
		t.Fatalf("s1 outcome = %v, want pending", hands[1].Outcome)
	}

	if err := g.Stand(); err != nil { // s1 stands 16, dealer busts
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	hands = g.Player()
	if hands[0].Outcome != OutcomeBlackjack {
		t.Errorf("s0 outcome changed to %v, want blackjack (not re-settled)", hands[0].Outcome)
	}
	if hands[1].Outcome != OutcomeWin {
		t.Errorf("s1 outcome = %v, want win", hands[1].Outcome)
	}
	// +150 (s0 natural) + 10 (s1 win) = +160.
	if g.Bankroll() != 1160 {
		t.Errorf("bankroll = %d, want 1160", g.Bankroll())
	}
}

// ---- Per-spot split cap ----

func TestPerSpotSplitCap(t *testing.T) {
	// 2 spots both dealt 8,8. Split spot 0 to the 4-hand cap; spot 1 (still one
	// hand) must remain splittable, proving the cap is per spot, not global.
	//   Deal: s0c1=8, s1c1=8, Dup=7, s0c2=8, s1c2=8, Dhole=10
	//   spot 0's parent redraws 8,8,8 across three splits (idx 6,7,8)
	//   the three new spot-0 hands take a 2 each (idx 9,10,11)
	g := NewGameWithShoe(1000, stack(
		Eight, Eight, // first card each
		Seven,        // dealer up
		Eight, Eight, // second card each
		Ten,                 // dealer hole
		Eight, Eight, Eight, // spot-0 parent redraws across 3 splits
		Two, Two, Two, // the 3 new spot-0 hands' second cards
	))
	setupSpots(t, g, 3, 3)
	mustDeal(t, g)

	// Split spot 0 three times to reach MaxHands for that spot.
	for i := 0; i < 3; i++ {
		if !hasAction(g, ActionSplit) {
			t.Fatalf("spot-0 split #%d should be legal", i+1)
		}
		if err := g.Split(); err != nil {
			t.Fatalf("spot-0 split #%d: %v", i+1, err)
		}
	}
	spot0Hands := 0
	for _, h := range g.Player() {
		if h.Spot == 0 {
			spot0Hands++
		}
	}
	if spot0Hands != MaxHands {
		t.Fatalf("spot 0 hand count = %d, want %d", spot0Hands, MaxHands)
	}
	if hasAction(g, ActionSplit) {
		t.Error("spot 0 at cap must not allow another split")
	}
	if n := len(g.Player()); n != MaxHands+1 {
		t.Fatalf("total hands = %d, want %d (4 on spot 0 + 1 on spot 1)", n, MaxHands+1)
	}

	// Stand through spot 0's four hands to reach spot 1.
	for i := 0; i < MaxHands; i++ {
		if g.Player()[g.ActiveHandIndex()].Spot != 0 {
			t.Fatalf("expected to still be on spot 0 at step %d", i)
		}
		if err := g.Stand(); err != nil {
			t.Fatalf("stand spot-0 hand %d: %v", i, err)
		}
	}
	// Now active is spot 1 (one hand); split must be legal despite 5 total hands.
	if g.Player()[g.ActiveHandIndex()].Spot != 1 {
		t.Fatalf("active spot = %d, want 1", g.Player()[g.ActiveHandIndex()].Spot)
	}
	if !hasAction(g, ActionSplit) {
		t.Error("spot 1 (single hand) must be splittable even though total hands exceed MaxHands")
	}
}

// ---- Per-hand insurance ----

func TestPerHandInsuranceDealerBlackjackPays2To1(t *testing.T) {
	// Dealer A up, 10 hole => blackjack. 3 spots (all 9,7=16, no naturals).
	// Take insurance on spots 0 and 2, decline spot 1.
	g := NewGameWithShoe(1000, stack(
		Nine, Nine, Nine, // first card each
		Ace,                 // dealer up
		Seven, Seven, Seven, // second card each
		Ten, // dealer hole => blackjack
	))
	setupSpots(t, g, 100, 100, 100)
	mustDeal(t, g)

	if g.Phase() != PhaseInsurance {
		t.Fatalf("phase = %v, want insurance", g.Phase())
	}
	if g.InsuranceSpot() != 0 {
		t.Fatalf("InsuranceSpot = %d, want 0", g.InsuranceSpot())
	}
	if err := g.Insurance(true); err != nil { // spot 0 takes
		t.Fatal(err)
	}
	if g.InsuranceSpot() != 1 {
		t.Fatalf("InsuranceSpot = %d, want 1", g.InsuranceSpot())
	}
	if err := g.Insurance(false); err != nil { // spot 1 declines
		t.Fatal(err)
	}
	if g.InsuranceSpot() != 2 {
		t.Fatalf("InsuranceSpot = %d, want 2", g.InsuranceSpot())
	}
	if err := g.Insurance(true); err != nil { // spot 2 takes => peek
		t.Fatal(err)
	}

	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	if g.InsuranceSpot() != -1 {
		t.Fatalf("InsuranceSpot = %d, want -1 after offers", g.InsuranceSpot())
	}
	if !g.DealerHadBlackjack() {
		t.Fatal("dealer should have blackjack")
	}
	for i, h := range g.Player() {
		if h.Outcome != OutcomeLose {
			t.Errorf("hand %d outcome = %v, want lose", i, h.Outcome)
		}
	}
	// Two 50-stake insurances taken; each pays 2:1 (+100), total +200.
	if g.InsuranceBet() != 100 {
		t.Errorf("insurance bet total = %d, want 100 (two $50)", g.InsuranceBet())
	}
	if g.InsuranceNet() != 200 {
		t.Errorf("insurance net = %d, want +200 (2 x 2:1 on 50)", g.InsuranceNet())
	}
	// escrow 300; -50 -50 insurance; +150 +150 insurance returns => 900.
	if g.Bankroll() != 900 {
		t.Errorf("bankroll = %d, want 900", g.Bankroll())
	}
}

func TestPerHandInsuranceDealerNoBlackjackLoses(t *testing.T) {
	// Dealer A up, 6 hole => no blackjack. 2 spots both take insurance and lose
	// it; the hands then play on.
	g := NewGameWithShoe(1000, stack(
		Ten, Ten, // first card each
		Ace,        // dealer up
		Nine, Eight, // second card each
		Six, // dealer hole => A,6 soft 17
		Ten, // dealer hits soft 17 => 17 hard, stands
	))
	setupSpots(t, g, 100, 100)
	mustDeal(t, g)

	if err := g.Insurance(true); err != nil { // spot 0
		t.Fatal(err)
	}
	if err := g.Insurance(true); err != nil { // spot 1 => peek
		t.Fatal(err)
	}
	if g.DealerHadBlackjack() {
		t.Fatal("dealer should not have blackjack")
	}
	if g.Phase() != PhasePlayerTurn {
		t.Fatalf("phase = %v, want player-turn", g.Phase())
	}
	if g.InsuranceNet() != -100 {
		t.Fatalf("insurance net = %d, want -100 (two $50 lost)", g.InsuranceNet())
	}
	// escrow 200 + two $50 insurance = 700.
	if g.Bankroll() != 700 {
		t.Fatalf("bankroll = %d, want 700", g.Bankroll())
	}

	if err := g.Stand(); err != nil { // s0 = 19
		t.Fatal(err)
	}
	if err := g.Stand(); err != nil { // s1 = 18
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	hands := g.Player()
	if hands[0].Outcome != OutcomeWin || hands[1].Outcome != OutcomeWin {
		t.Fatalf("outcomes = %v/%v, want win/win", hands[0].Outcome, hands[1].Outcome)
	}
	// +100 +100 hand wins, -50 -50 insurance => +100.
	if g.Bankroll() != 1100 {
		t.Fatalf("bankroll = %d, want 1100", g.Bankroll())
	}
}

// ---- Betting affordability across spots ----

func TestAddSpotAffordabilityAndCap(t *testing.T) {
	// Bankroll 10. One spot raised to $10 leaves nothing for a second spot.
	g := NewGameWithShoe(10, stack(Ten, Ten, Ten, Ten))
	if err := g.SetSpotBet(0, 10); err != nil {
		t.Fatalf("SetSpotBet(0,10): %v", err)
	}
	if err := g.AddSpot(); err == nil {
		t.Fatal("AddSpot should fail: $10 + $3 > $10 bankroll")
	}

	// Bankroll 10 supports three $3 spots but not a fourth (cap) or a raise.
	g = NewGameWithShoe(10, stack(Ten, Ten, Ten, Ten))
	if err := g.AddSpot(); err != nil { // total 6
		t.Fatalf("AddSpot #2: %v", err)
	}
	if err := g.AddSpot(); err != nil { // total 9
		t.Fatalf("AddSpot #3: %v", err)
	}
	if g.NumSpots() != MaxSpots {
		t.Fatalf("NumSpots = %d, want %d", g.NumSpots(), MaxSpots)
	}
	if err := g.AddSpot(); err == nil {
		t.Fatal("AddSpot past MaxSpots should fail")
	}
	// Raising spot 2 to $5 would make the total 11 > 10.
	if err := g.SetSpotBet(2, 5); err == nil {
		t.Fatal("SetSpotBet should fail when total would exceed bankroll")
	}
	// Raising to $4 makes the total exactly 10, which is allowed.
	if err := g.SetSpotBet(2, 4); err != nil {
		t.Fatalf("SetSpotBet(2,4) should succeed (total 10 == bankroll): %v", err)
	}
	// Below the table minimum is rejected.
	if err := g.SetSpotBet(0, MinBet-1); err == nil {
		t.Fatal("SetSpotBet below MinBet should fail")
	}
}

func TestRemoveSpotRules(t *testing.T) {
	g := NewGameWithShoe(1000, stack(Ten, Ten, Ten, Ten))
	// Cannot remove the only spot.
	if err := g.RemoveSpot(0); err == nil {
		t.Fatal("RemoveSpot at one spot should fail")
	}
	if err := g.AddSpot(); err != nil {
		t.Fatal(err)
	}
	if err := g.SetSpotBet(0, 10); err != nil {
		t.Fatal(err)
	}
	if err := g.SetSpotBet(1, 20); err != nil {
		t.Fatal(err)
	}
	// Out-of-range index is illegal.
	if err := g.RemoveSpot(5); err == nil {
		t.Fatal("RemoveSpot out of range should fail")
	}
	// Removing spot 0 leaves spot 1's bet at index 0.
	if err := g.RemoveSpot(0); err != nil {
		t.Fatalf("RemoveSpot(0): %v", err)
	}
	if g.NumSpots() != 1 {
		t.Fatalf("NumSpots = %d, want 1", g.NumSpots())
	}
	if g.SpotBet(0) != 20 {
		t.Fatalf("SpotBet(0) = %d, want 20 (spot 1 shifted down)", g.SpotBet(0))
	}
	if g.Bet() != 20 {
		t.Fatalf("Bet() = %d, want 20 (back-compat maps to spot 0)", g.Bet())
	}
	// Down to one spot again: removal illegal.
	if err := g.RemoveSpot(0); err == nil {
		t.Fatal("RemoveSpot at one spot should fail")
	}
}

// ---- Dealer plays iff a hand is still live ----

func TestDealerSkipsWhenAllHandsBust(t *testing.T) {
	// 2 spots, both bust. Dealer (10,6=16) must NOT draw.
	g := NewGameWithShoe(1000, stack(
		Ten, Ten, // first card each
		Ten,      // dealer up
		Six, Six, // second card each
		Six,      // dealer hole => 10,6 = 16
		Ten, Ten, // player hits => both bust
	))
	setupSpots(t, g, 10, 10)
	mustDeal(t, g)
	if err := g.Hit(); err != nil { // s0 16 -> 26 bust
		t.Fatal(err)
	}
	if err := g.Hit(); err != nil { // s1 16 -> 26 bust
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	d := g.Dealer()
	if len(d.Cards) != 2 {
		t.Fatalf("dealer drew %d cards, want 2 (should skip when all bust)", len(d.Cards))
	}
	for i, h := range g.Player() {
		if h.Outcome != OutcomeLose {
			t.Errorf("hand %d outcome = %v, want lose", i, h.Outcome)
		}
	}
	if g.Bankroll() != 980 {
		t.Errorf("bankroll = %d, want 980", g.Bankroll())
	}
}

func TestDealerPlaysWhenSomeHandStands(t *testing.T) {
	// 2 spots: s0 busts, s1 stands 20. Dealer (10,6=16) must play and bust.
	g := NewGameWithShoe(1000, stack(
		Ten, Ten, // first card each
		Ten,      // dealer up
		Six, Ten, // second card each => s0=16, s1=20
		Six, // dealer hole => 10,6 = 16
		Ten, // s0 hit => bust
		Ten, // dealer hit => 26 bust
	))
	setupSpots(t, g, 10, 10)
	mustDeal(t, g)
	if err := g.Hit(); err != nil { // s0 -> bust
		t.Fatal(err)
	}
	if err := g.Stand(); err != nil { // s1 stands 20
		t.Fatal(err)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	d := g.Dealer()
	if len(d.Cards) != 3 {
		t.Fatalf("dealer drew %d cards, want 3 (must play for the live hand)", len(d.Cards))
	}
	hands := g.Player()
	if hands[0].Outcome != OutcomeLose {
		t.Errorf("s0 outcome = %v, want lose", hands[0].Outcome)
	}
	if hands[1].Outcome != OutcomeWin {
		t.Errorf("s1 outcome = %v, want win (dealer bust)", hands[1].Outcome)
	}
	if g.Bankroll() != 1000 {
		t.Errorf("bankroll = %d, want 1000", g.Bankroll())
	}
}
