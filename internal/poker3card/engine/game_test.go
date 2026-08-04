package engine

import "testing"

// deck assembles a stacked deck; the first three cards are dealt to the player,
// the next three to the dealer.
func deck(cards ...Card) []Card { return cards }

func mustDeal(t *testing.T, g *Game) {
	t.Helper()
	if err := g.Deal(); err != nil {
		t.Fatalf("Deal: %v", err)
	}
}

// checkNet verifies the settlement's total equals the sum of its components.
func checkNet(t *testing.T, s Settlement) {
	t.Helper()
	sum := s.Ante.Net + s.Play.Net + s.AnteBonus.Net + s.PairPlus.Net
	if s.Net != sum {
		t.Errorf("Net = %d, want sum of components %d", s.Net, sum)
	}
}

// ---- Dealer qualification ----

func TestQualificationThreshold(t *testing.T) {
	qHigh := Evaluate(h(c(Queen, Spades), c(Nine, Hearts), c(Four, Clubs)))
	if !Qualifies(qHigh) {
		t.Error("Queen-high must qualify")
	}
	jHigh := Evaluate(h(c(Jack, Spades), c(Nine, Hearts), c(Four, Clubs)))
	if Qualifies(jHigh) {
		t.Error("Jack-high must NOT qualify")
	}
	pairTwos := Evaluate(h(c(Two, Spades), c(Two, Hearts), c(Five, Clubs)))
	if !Qualifies(pairTwos) {
		t.Error("a pair of 2s must qualify")
	}
}

// ---- Ante/Play resolution ----

func TestDealerDoesNotQualify(t *testing.T) {
	// Player King-high, dealer Jack-high (no qualify). Ante pays 1:1, Play pushes.
	g := NewGameWithDeck(1000, deck(
		c(King, Spades), c(Queen, Hearts), c(Nine, Clubs), // player K-high
		c(Jack, Spades), c(Seven, Hearts), c(Two, Clubs), // dealer J-high (no qualify)
	))
	if err := g.SetAnte(100); err != nil {
		t.Fatal(err)
	}
	mustDeal(t, g)
	if g.Phase() != PhaseDecision {
		t.Fatalf("phase = %v, want decision", g.Phase())
	}
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	if s.DealerQualified {
		t.Error("dealer should not qualify")
	}
	if s.Ante.Outcome != OutcomeWin || s.Ante.Net != 100 {
		t.Errorf("ante = %v net %d, want win +100", s.Ante.Outcome, s.Ante.Net)
	}
	if s.Play.Outcome != OutcomePush || s.Play.Net != 0 {
		t.Errorf("play = %v net %d, want push 0", s.Play.Outcome, s.Play.Net)
	}
	if s.Net != 100 || g.Bankroll() != 1100 {
		t.Errorf("net = %d bankroll = %d, want +100 / 1100", s.Net, g.Bankroll())
	}
	checkNet(t, s)
}

func TestDealerQualifiesPlayerWins(t *testing.T) {
	// Player Ace-high, dealer Queen-high (qualifies). Ante and Play both pay 1:1.
	g := NewGameWithDeck(1000, deck(
		c(Ace, Spades), c(King, Hearts), c(Nine, Clubs), // player A-high
		c(Queen, Spades), c(Jack, Hearts), c(Three, Clubs), // dealer Q-high (qualifies)
	))
	_ = g.SetAnte(100)
	mustDeal(t, g)
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	if !s.DealerQualified {
		t.Error("dealer should qualify")
	}
	if s.Ante.Net != 100 || s.Play.Net != 100 {
		t.Errorf("ante/play net = %d/%d, want 100/100", s.Ante.Net, s.Play.Net)
	}
	if s.Net != 200 || g.Bankroll() != 1200 {
		t.Errorf("net = %d bankroll = %d, want +200 / 1200", s.Net, g.Bankroll())
	}
	checkNet(t, s)
}

func TestDealerQualifiesDealerWins(t *testing.T) {
	// Player Queen-high, dealer Ace-high (qualifies, wins). Lose both.
	g := NewGameWithDeck(1000, deck(
		c(Queen, Spades), c(Nine, Hearts), c(Four, Clubs), // player Q-high
		c(Ace, Spades), c(King, Hearts), c(Seven, Clubs), // dealer A-high
	))
	_ = g.SetAnte(100)
	mustDeal(t, g)
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	if s.Ante.Net != -100 || s.Play.Net != -100 {
		t.Errorf("ante/play net = %d/%d, want -100/-100", s.Ante.Net, s.Play.Net)
	}
	if s.Net != -200 || g.Bankroll() != 800 {
		t.Errorf("net = %d bankroll = %d, want -200 / 800", s.Net, g.Bankroll())
	}
	checkNet(t, s)
}

func TestDealerQualifiesTiePush(t *testing.T) {
	// Identical King-high hands (qualify): both Ante and Play push.
	g := NewGameWithDeck(1000, deck(
		c(King, Spades), c(Nine, Hearts), c(Four, Clubs), // player
		c(King, Diamonds), c(Nine, Clubs), c(Four, Hearts), // dealer (same ranks)
	))
	_ = g.SetAnte(100)
	mustDeal(t, g)
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	if s.Ante.Outcome != OutcomePush || s.Play.Outcome != OutcomePush {
		t.Errorf("ante/play = %v/%v, want push/push", s.Ante.Outcome, s.Play.Outcome)
	}
	if s.Net != 0 || g.Bankroll() != 1000 {
		t.Errorf("net = %d bankroll = %d, want 0 / 1000", s.Net, g.Bankroll())
	}
	checkNet(t, s)
}

// ---- Ante Bonus ----

func TestAnteBonusAmounts(t *testing.T) {
	tests := []struct {
		name       string
		player     [3]Card
		wantCat    HandCategory
		wantBonus  int // on a 100 ante
		wantPlayed bool
	}{
		{"straight", h(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs)), Straight, 100, true},
		{"trips", h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs)), ThreeOfAKind, 400, true},
		{"straight flush", h(c(Nine, Spades), c(Eight, Spades), c(Seven, Spades)), StraightFlush, 500, true},
	}
	for _, tc := range tests {
		// Dealer is a non-qualifying 5-high; bonus is independent of that.
		g := NewGameWithDeck(1000, deck(
			tc.player[0], tc.player[1], tc.player[2],
			c(Five, Diamonds), c(Three, Hearts), c(Two, Clubs),
		))
		_ = g.SetAnte(100)
		mustDeal(t, g)
		if err := g.Play(); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		s := g.Result()
		if !s.AnteBonus.Applies || s.AnteBonus.Category != tc.wantCat {
			t.Errorf("%s: bonus applies=%v cat=%v", tc.name, s.AnteBonus.Applies, s.AnteBonus.Category)
		}
		if s.AnteBonus.Net != tc.wantBonus {
			t.Errorf("%s: bonus net = %d, want %d", tc.name, s.AnteBonus.Net, tc.wantBonus)
		}
		checkNet(t, s)
	}
}

func TestAnteBonusNotPaidOnFold(t *testing.T) {
	// Player holds a straight but folds: no Ante Bonus, Ante forfeited.
	g := NewGameWithDeck(1000, deck(
		c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs), // player straight
		c(Ace, Spades), c(King, Hearts), c(Queen, Clubs), // dealer (irrelevant)
	))
	_ = g.SetAnte(100)
	mustDeal(t, g)
	if err := g.Fold(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	if s.AnteBonus.Applies {
		t.Error("no Ante Bonus should be paid on a fold")
	}
	if s.Ante.Outcome != OutcomeLoss || s.Ante.Net != -100 {
		t.Errorf("ante = %v net %d, want loss -100", s.Ante.Outcome, s.Ante.Net)
	}
	if s.Net != -100 || g.Bankroll() != 900 {
		t.Errorf("net = %d bankroll = %d, want -100 / 900", s.Net, g.Bankroll())
	}
	checkNet(t, s)
}

// ---- Fold with Pair Plus still resolved ----

func TestFoldPairPlusWins(t *testing.T) {
	// Fold forfeits the Ante, but a winning Pair Plus (a pair) still pays 1:1.
	g := NewGameWithDeck(1000, deck(
		c(King, Spades), c(King, Hearts), c(Three, Clubs), // player pair of kings
		c(Ace, Spades), c(Queen, Hearts), c(Nine, Clubs), // dealer
	))
	_ = g.SetAnte(100)
	_ = g.SetPairPlus(100)
	mustDeal(t, g)
	if err := g.Fold(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	if !s.Folded || s.Played {
		t.Error("hand should be recorded as folded, not played")
	}
	if s.Ante.Net != -100 {
		t.Errorf("ante net = %d, want -100 (forfeited)", s.Ante.Net)
	}
	if s.PairPlus.Outcome != OutcomeWin || s.PairPlus.Net != 100 {
		t.Errorf("pairplus = %v net %d, want win +100", s.PairPlus.Outcome, s.PairPlus.Net)
	}
	// -100 ante + 100 pairplus = 0 net; escrow 200 then 200 returned.
	if s.Net != 0 || g.Bankroll() != 1000 {
		t.Errorf("net = %d bankroll = %d, want 0 / 1000", s.Net, g.Bankroll())
	}
	checkNet(t, s)
}

func TestFoldPairPlusLoses(t *testing.T) {
	// Fold with a losing Pair Plus (high card): lose both stakes.
	g := NewGameWithDeck(1000, deck(
		c(King, Spades), c(Nine, Hearts), c(Four, Clubs), // player high card
		c(Ace, Spades), c(Queen, Hearts), c(Nine, Clubs), // dealer
	))
	_ = g.SetAnte(100)
	_ = g.SetPairPlus(100)
	mustDeal(t, g)
	if err := g.Fold(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	if s.PairPlus.Outcome != OutcomeLoss || s.PairPlus.Net != -100 {
		t.Errorf("pairplus = %v net %d, want loss -100", s.PairPlus.Outcome, s.PairPlus.Net)
	}
	if s.Net != -200 || g.Bankroll() != 800 {
		t.Errorf("net = %d bankroll = %d, want -200 / 800", s.Net, g.Bankroll())
	}
	checkNet(t, s)
}

// ---- Pair Plus pay table ----

func TestPairPlusPayTable(t *testing.T) {
	tests := []struct {
		name    string
		player  [3]Card
		wantNet int // on a 100 Pair Plus
		wantWin bool
	}{
		{"straight flush", h(c(Nine, Spades), c(Eight, Spades), c(Seven, Spades)), 4000, true},
		{"trips", h(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs)), 3000, true},
		{"straight", h(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs)), 600, true},
		{"flush", h(c(King, Diamonds), c(Nine, Diamonds), c(Two, Diamonds)), 300, true},
		{"pair", h(c(King, Spades), c(King, Hearts), c(Three, Clubs)), 100, true},
		{"high card loses", h(c(King, Spades), c(Nine, Hearts), c(Two, Clubs)), -100, false},
	}
	for _, tc := range tests {
		// Pair Plus-only deal (no Ante): the decision phase is skipped.
		g := NewGameWithDeck(1000, deck(
			tc.player[0], tc.player[1], tc.player[2],
			c(Two, Clubs), c(Five, Hearts), c(Nine, Diamonds),
		))
		if err := g.SetAnte(0); err != nil {
			t.Fatalf("%s SetAnte(0): %v", tc.name, err)
		}
		if err := g.SetPairPlus(100); err != nil {
			t.Fatalf("%s SetPairPlus: %v", tc.name, err)
		}
		mustDeal(t, g)
		if g.Phase() != PhaseRoundOver {
			t.Fatalf("%s: phase = %v, want round-over (no decision for Pair Plus-only)", tc.name, g.Phase())
		}
		s := g.Result()
		if s.Played || s.Folded {
			t.Errorf("%s: Pair Plus-only should be neither played nor folded", tc.name)
		}
		gotWin := s.PairPlus.Outcome == OutcomeWin
		if gotWin != tc.wantWin || s.PairPlus.Net != tc.wantNet {
			t.Errorf("%s: pairplus = %v net %d, want win=%v net %d", tc.name, s.PairPlus.Outcome, s.PairPlus.Net, tc.wantWin, tc.wantNet)
		}
		if s.Net != tc.wantNet {
			t.Errorf("%s: net = %d, want %d", tc.name, s.Net, tc.wantNet)
		}
		checkNet(t, s)
	}
}

// ---- Pair Plus-only skips the decision phase ----

func TestPairPlusOnlySkipsDecision(t *testing.T) {
	g := NewGameWithDeck(1000, deck(
		c(King, Spades), c(King, Hearts), c(Three, Clubs), // player pair
		c(Ace, Spades), c(Queen, Hearts), c(Nine, Clubs), // dealer
	))
	_ = g.SetAnte(0)
	_ = g.SetPairPlus(100)
	mustDeal(t, g)
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	if g.Result().Ante.Bet != 0 {
		t.Error("no ante should be recorded")
	}
	// Pair pays 1:1: escrow 100, returned 200 => +100.
	if g.Bankroll() != 1100 {
		t.Errorf("bankroll = %d, want 1100", g.Bankroll())
	}
}

// ---- Betting validation ----

func TestDealRequiresAWager(t *testing.T) {
	g := NewGameWithDeck(1000, deck(
		c(King, Spades), c(Nine, Hearts), c(Four, Clubs),
		c(Two, Clubs), c(Five, Hearts), c(Nine, Diamonds),
	))
	_ = g.SetAnte(0)
	_ = g.SetPairPlus(0)
	if err := g.Deal(); err != ErrInvalidBet {
		t.Fatalf("Deal with no wagers = %v, want ErrInvalidBet", err)
	}
}

func TestBetsMustMeetMinimum(t *testing.T) {
	g := NewGameWithDeck(1000, nil)
	if err := g.SetAnte(MinBet - 1); err != ErrInvalidBet {
		t.Errorf("SetAnte below min = %v, want ErrInvalidBet", err)
	}
	if err := g.SetPairPlus(MinBet - 1); err != ErrInvalidBet {
		t.Errorf("SetPairPlus below min = %v, want ErrInvalidBet", err)
	}
}

func TestBetsMustBeAffordable(t *testing.T) {
	g := NewGameWithDeck(10, nil)
	if err := g.SetAnte(8); err != nil {
		t.Fatal(err)
	}
	if err := g.SetPairPlus(8); err != ErrInvalidBet {
		t.Errorf("unaffordable combined escrow = %v, want ErrInvalidBet", err)
	}
}

// ---- Escrow atomicity through the deal ----

func TestEscrowRemovedAtDeal(t *testing.T) {
	g := NewGameWithDeck(1000, deck(
		c(King, Spades), c(Nine, Hearts), c(Four, Clubs),
		c(Two, Clubs), c(Five, Hearts), c(Nine, Diamonds),
	))
	_ = g.SetAnte(100)
	_ = g.SetPairPlus(50)
	mustDeal(t, g)
	if g.Bankroll() != 850 {
		t.Fatalf("post-escrow bankroll = %d, want 850", g.Bankroll())
	}
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	// Play posts another 100.
	if g.PlayBet() != 100 {
		t.Errorf("play bet = %d, want 100", g.PlayBet())
	}
}

// ---- Play affordability ----

func TestPlayIllegalWhenUnaffordable(t *testing.T) {
	// Bankroll 5, ante 3: after escrow only 2 remains, cannot post the Play bet.
	g := NewGameWithDeck(5, deck(
		c(King, Spades), c(Nine, Hearts), c(Four, Clubs),
		c(Two, Clubs), c(Five, Hearts), c(Nine, Diamonds),
	))
	mustDeal(t, g)
	if g.CanPlay() {
		t.Error("CanPlay should be false when the Play bet is unaffordable")
	}
	if err := g.Play(); err != ErrIllegalAction {
		t.Errorf("Play = %v, want ErrIllegalAction", err)
	}
	if err := g.Fold(); err != nil {
		t.Fatal(err)
	}
}

// ---- NextHand & game over ----

func TestNextHandCarriesBets(t *testing.T) {
	g := NewGameWithDeck(1000, deck(
		c(Ace, Spades), c(King, Hearts), c(Nine, Clubs),
		c(Queen, Spades), c(Jack, Hearts), c(Three, Clubs),
	))
	_ = g.SetAnte(50)
	_ = g.SetPairPlus(20)
	mustDeal(t, g)
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseBetting {
		t.Fatalf("phase = %v, want betting", g.Phase())
	}
	if g.Ante() != 50 || g.PairPlus() != 20 {
		t.Errorf("carried bets = %d/%d, want 50/20", g.Ante(), g.PairPlus())
	}
	if g.PlayBet() != 0 {
		t.Errorf("play bet should reset to 0, got %d", g.PlayBet())
	}
}

func TestGameOverWhenBroke(t *testing.T) {
	// Bankroll 6, ante 3. Play (posts 3), lose both to a qualifying dealer =>
	// bankroll 0 => game over on NextHand.
	g := NewGameWithDeck(6, deck(
		c(Two, Spades), c(Five, Hearts), c(Nine, Clubs), // player 9-high
		c(King, Spades), c(Queen, Hearts), c(Seven, Clubs), // dealer K-high (qualifies, wins)
	))
	mustDeal(t, g) // ante defaults to MinBet=3
	if !g.CanPlay() {
		t.Fatal("should be able to play with 3 left after escrow")
	}
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	if g.Bankroll() != 0 {
		t.Fatalf("bankroll = %d, want 0", g.Bankroll())
	}
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseGameOver {
		t.Fatalf("phase = %v, want game-over", g.Phase())
	}
}

// ---- Integer money consistency across a full round ----

func TestNetMatchesBankrollDelta(t *testing.T) {
	g := NewGameWithDeck(1000, deck(
		c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), // player trips
		c(King, Spades), c(Queen, Hearts), c(Two, Clubs), // dealer K-high (qualifies)
	))
	_ = g.SetAnte(30)
	_ = g.SetPairPlus(30)
	before := 1000
	mustDeal(t, g)
	if err := g.Play(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	if g.Bankroll()-before != s.Net {
		t.Errorf("bankroll delta = %d, settlement net = %d", g.Bankroll()-before, s.Net)
	}
	checkNet(t, s)
	// Trips: Ante 1:1 (+30), Play 1:1 (+30), Ante Bonus 4:1 (+120),
	// Pair Plus 30:1 (+900) => +1080.
	if s.Net != 1080 {
		t.Errorf("net = %d, want 1080", s.Net)
	}
}
