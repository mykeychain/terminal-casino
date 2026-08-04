package engine

import "testing"

// deck assembles a stacked deck. Deal order: cards[0..1] are the hole cards,
// cards[2..4] the community cards in reveal order (index 2 revealed first).
func deck(cards ...Card) []Card { return cards }

func mustDeal(t *testing.T, g *Game) {
	t.Helper()
	if err := g.Deal(); err != nil {
		t.Fatalf("Deal: %v", err)
	}
}

func mustRaise(t *testing.T, g *Game, mult int) {
	t.Helper()
	if err := g.Raise(mult); err != nil {
		t.Fatalf("Raise(%d): %v", mult, err)
	}
}

// playThrough sets the ante, deals, and raises by the given multiples on 3rd,
// 4th, and 5th street, settling the hand. It returns the settlement and the
// bankroll delta over the whole round.
func playThrough(t *testing.T, bankroll, ante int, cards []Card, mults [3]int) (Settlement, int) {
	t.Helper()
	g := NewGameWithDeck(bankroll, cards)
	if err := g.SetAnte(ante); err != nil {
		t.Fatalf("SetAnte: %v", err)
	}
	before := g.Bankroll()
	mustDeal(t, g)
	for _, m := range mults {
		mustRaise(t, g, m)
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	s := g.Result()
	delta := g.Bankroll() - before
	if s.Net != delta {
		t.Errorf("Net = %d but bankroll delta = %d", s.Net, delta)
	}
	return s, delta
}

// ---- The spec's headline example: pay on the TOTAL WAGERED ----

func TestTotalWageredFlushExample(t *testing.T) {
	// Ante 25, raise 3x/3x/3x => totalWagered = 25 + 75 + 75 + 75 = 250.
	// A Flush pays 6:1 on the total: profit = 6*250 = 1500, returns 1750.
	flush := deck(
		c(King, Diamonds), c(Nine, Diamonds), // hole
		c(Seven, Diamonds), c(Four, Diamonds), c(Two, Diamonds), // community
	)
	s, delta := playThrough(t, 1000, 25, flush, [3]int{3, 3, 3})
	if s.Category != Flush {
		t.Fatalf("category = %v, want Flush", s.Category)
	}
	if s.TotalWagered != 250 {
		t.Errorf("total wagered = %d, want 250", s.TotalWagered)
	}
	if s.Multiplier != 6 || s.Profit != 1500 || s.Net != 1500 {
		t.Errorf("mult/profit/net = %d/%d/%d, want 6/1500/1500", s.Multiplier, s.Profit, s.Net)
	}
	if delta != 1500 {
		t.Errorf("bankroll delta = %d, want 1500", delta)
	}
	// Bankroll: 1000 - 250 escrowed + 1750 returned = 2500.
	if got := 1000 + delta; got != 2500 {
		t.Errorf("final bankroll = %d, want 2500", got)
	}
}

// ---- Every flat-paying category on the total wager ----

func TestPayTableOnTotalWagered(t *testing.T) {
	// Ante 10, raise 1x on each street => totalWagered = 10*4 = 40.
	const ante, total = 10, 40
	tests := []struct {
		name     string
		cards    []Card
		wantCat  HandCategory
		wantMult int
	}{
		{"royal flush", deck(c(Ace, Spades), c(King, Spades), c(Queen, Spades), c(Jack, Spades), c(Ten, Spades)), RoyalFlush, 500},
		{"straight flush", deck(c(Nine, Spades), c(Eight, Spades), c(Seven, Spades), c(Six, Spades), c(Five, Spades)), StraightFlush, 100},
		{"four of a kind", deck(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), c(Seven, Diamonds), c(Two, Spades)), FourOfAKind, 40},
		{"full house", deck(c(King, Spades), c(King, Hearts), c(King, Clubs), c(Two, Spades), c(Two, Hearts)), FullHouse, 10},
		{"flush", deck(c(King, Diamonds), c(Nine, Diamonds), c(Seven, Diamonds), c(Four, Diamonds), c(Two, Diamonds)), Flush, 6},
		{"straight", deck(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs), c(Six, Diamonds), c(Five, Spades)), Straight, 4},
		{"three of a kind", deck(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), c(King, Diamonds), c(Two, Spades)), ThreeOfAKind, 3},
		{"two pair", deck(c(King, Spades), c(King, Hearts), c(Two, Clubs), c(Two, Diamonds), c(Nine, Spades)), TwoPair, 2},
		{"pair of jacks", deck(c(Jack, Spades), c(Jack, Hearts), c(Nine, Clubs), c(Four, Diamonds), c(Two, Spades)), Pair, 1},
	}
	for _, tc := range tests {
		s, delta := playThrough(t, 1000, ante, tc.cards, [3]int{1, 1, 1})
		if s.Category != tc.wantCat {
			t.Errorf("%s: category = %v, want %v", tc.name, s.Category, tc.wantCat)
		}
		wantNet := tc.wantMult * total
		if s.Outcome != OutcomeWin || s.Net != wantNet || delta != wantNet {
			t.Errorf("%s: outcome %v net %d delta %d, want win net %d", tc.name, s.Outcome, s.Net, delta, wantNet)
		}
	}
}

// ---- Pair pay tiers through settlement ----

func TestPairTiersSettlement(t *testing.T) {
	// Ante 10, 1x/1x/1x => total 40.
	pushEights := deck(c(Eight, Spades), c(Eight, Hearts), c(Nine, Clubs), c(Four, Diamonds), c(Two, Spades))
	s, delta := playThrough(t, 1000, 10, pushEights, [3]int{1, 1, 1})
	if s.Outcome != OutcomePush || s.Net != 0 || delta != 0 {
		t.Errorf("pair of 8s: outcome %v net %d delta %d, want push 0 0", s.Outcome, s.Net, delta)
	}

	lossFours := deck(c(Four, Spades), c(Four, Hearts), c(Nine, Clubs), c(Seven, Diamonds), c(Two, Spades))
	s, delta = playThrough(t, 1000, 10, lossFours, [3]int{1, 1, 1})
	if s.Outcome != OutcomeLoss || s.Net != -40 || delta != -40 {
		t.Errorf("pair of 4s: outcome %v net %d delta %d, want loss -40 -40", s.Outcome, s.Net, delta)
	}

	highCard := deck(c(King, Spades), c(Nine, Hearts), c(Seven, Clubs), c(Four, Diamonds), c(Two, Spades))
	s, delta = playThrough(t, 1000, 10, highCard, [3]int{1, 1, 1})
	if s.Outcome != OutcomeLoss || s.Net != -40 || delta != -40 {
		t.Errorf("high card: outcome %v net %d delta %d, want loss -40 -40", s.Outcome, s.Net, delta)
	}
}

// ---- Folding at each street forfeits exactly the accumulated wager ----

func TestFoldAtThirdStreet(t *testing.T) {
	g := NewGameWithDeck(1000, sampleDeck())
	_ = g.SetAnte(10)
	mustDeal(t, g)
	if err := g.Fold(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	// Only the ante is at risk at 3rd street.
	if !s.Folded || s.TotalWagered != 10 || s.Net != -10 || g.Bankroll() != 990 {
		t.Errorf("3rd-street fold: total %d net %d bankroll %d, want 10 / -10 / 990",
			s.TotalWagered, s.Net, g.Bankroll())
	}
}

func TestFoldAtFourthStreet(t *testing.T) {
	g := NewGameWithDeck(1000, sampleDeck())
	_ = g.SetAnte(10)
	mustDeal(t, g)
	mustRaise(t, g, 2) // 3rd-street bet = 20
	if g.Phase() != Phase4thStreet {
		t.Fatalf("phase = %v, want 4th-street", g.Phase())
	}
	if err := g.Fold(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	// Ante + 3rd-street bet = 10 + 20 = 30 forfeited.
	if s.TotalWagered != 30 || s.Net != -30 || g.Bankroll() != 970 {
		t.Errorf("4th-street fold: total %d net %d bankroll %d, want 30 / -30 / 970",
			s.TotalWagered, s.Net, g.Bankroll())
	}
}

func TestFoldAtFifthStreet(t *testing.T) {
	g := NewGameWithDeck(1000, sampleDeck())
	_ = g.SetAnte(10)
	mustDeal(t, g)
	mustRaise(t, g, 1) // 3rd = 10
	mustRaise(t, g, 3) // 4th = 30
	if g.Phase() != Phase5thStreet {
		t.Fatalf("phase = %v, want 5th-street", g.Phase())
	}
	if err := g.Fold(); err != nil {
		t.Fatal(err)
	}
	s := g.Result()
	// Ante + 3rd + 4th = 10 + 10 + 30 = 50 forfeited.
	if s.TotalWagered != 50 || s.Net != -50 || g.Bankroll() != 950 {
		t.Errorf("5th-street fold: total %d net %d bankroll %d, want 50 / -50 / 950",
			s.TotalWagered, s.Net, g.Bankroll())
	}
}

// ---- Raise multiple validation ----

func TestRaiseMultipleMustBeOneTwoOrThree(t *testing.T) {
	g := NewGameWithDeck(1000, sampleDeck())
	_ = g.SetAnte(10)
	mustDeal(t, g)
	if err := g.Raise(0); err != ErrInvalidBet {
		t.Errorf("Raise(0) = %v, want ErrInvalidBet", err)
	}
	if err := g.Raise(4); err != ErrInvalidBet {
		t.Errorf("Raise(4) = %v, want ErrInvalidBet", err)
	}
	if err := g.Raise(2); err != nil {
		t.Errorf("Raise(2) = %v, want nil", err)
	}
}

// ---- Bankroll policy (b): cap raises as you go ----

func TestUnaffordableRaisesRejected(t *testing.T) {
	// Bankroll 25, ante 10 => after escrow 15 remains. Legal multiples: only 1x
	// (10 <= 15); 2x (20) and 3x (30) are unaffordable.
	g := NewGameWithDeck(25, sampleDeck())
	_ = g.SetAnte(10)
	mustDeal(t, g)
	if got := g.LegalRaises(); len(got) != 1 || got[0] != 1 {
		t.Errorf("LegalRaises = %v, want [1]", got)
	}
	if g.CanRaise(2) || g.CanRaise(3) {
		t.Error("2x and 3x must be unaffordable")
	}
	if err := g.Raise(2); err != ErrIllegalAction {
		t.Errorf("Raise(2) = %v, want ErrIllegalAction", err)
	}
}

func TestOnlyFoldWhenCannotAffordOneX(t *testing.T) {
	// Bankroll 10, ante 10 => after escrow 0 remains: not even 1x is affordable,
	// so only Fold is legal (documented edge case of policy (b)).
	g := NewGameWithDeck(10, sampleDeck())
	_ = g.SetAnte(10)
	mustDeal(t, g)
	if len(g.LegalRaises()) != 0 {
		t.Errorf("LegalRaises = %v, want empty", g.LegalRaises())
	}
	if g.CanRaise(1) {
		t.Error("1x must be unaffordable with 0 left")
	}
	if err := g.Raise(1); err != ErrIllegalAction {
		t.Errorf("Raise(1) = %v, want ErrIllegalAction", err)
	}
	if err := g.Fold(); err != nil {
		t.Fatalf("Fold must remain legal: %v", err)
	}
}

// ---- Ante validation ----

func TestAnteValidation(t *testing.T) {
	g := NewGameWithDeck(1000, nil)
	if err := g.SetAnte(MinBet - 1); err != ErrInvalidBet {
		t.Errorf("SetAnte below min = %v, want ErrInvalidBet", err)
	}
	g2 := NewGameWithDeck(5, nil)
	if err := g2.SetAnte(6); err != ErrInvalidBet {
		t.Errorf("SetAnte above bankroll = %v, want ErrInvalidBet", err)
	}
}

// ---- Community reveal one at a time ----

func TestCommunityRevealSequence(t *testing.T) {
	g := NewGameWithDeck(1000, sampleDeck())
	_ = g.SetAnte(10)
	mustDeal(t, g)
	if r := revealedCount(g); r != 0 {
		t.Fatalf("after deal revealed = %d, want 0", r)
	}
	mustRaise(t, g, 1)
	if r := revealedCount(g); r != 1 || !g.CommunityCards()[0].Revealed {
		t.Fatalf("after 3rd-street raise revealed = %d (index0 %v)", r, g.CommunityCards()[0].Revealed)
	}
	mustRaise(t, g, 1)
	if r := revealedCount(g); r != 2 {
		t.Fatalf("after 4th-street raise revealed = %d, want 2", r)
	}
	mustRaise(t, g, 1)
	if r := revealedCount(g); r != 3 {
		t.Fatalf("after 5th-street raise revealed = %d, want 3", r)
	}
}

func revealedCount(g *Game) int {
	n := 0
	for _, cc := range g.CommunityCards() {
		if cc.Revealed {
			n++
		}
	}
	return n
}

// ---- Visible-floor made-hand hint ----

func TestVisibleFloorHint(t *testing.T) {
	tests := []struct {
		name              string
		hole              [2]Card
		wantPush, wantWin bool
	}{
		{"pair of jacks guarantees win", [2]Card{c(Jack, Spades), c(Jack, Hearts)}, true, true},
		{"pair of eights guarantees push", [2]Card{c(Eight, Spades), c(Eight, Hearts)}, true, false},
		{"pair of fours guarantees nothing", [2]Card{c(Four, Spades), c(Four, Hearts)}, false, false},
		{"no pair guarantees nothing", [2]Card{c(King, Spades), c(Nine, Hearts)}, false, false},
	}
	for _, tc := range tests {
		g := NewGameWithDeck(1000, deck(
			tc.hole[0], tc.hole[1],
			c(Two, Clubs), c(Three, Diamonds), c(Five, Spades),
		))
		_ = g.SetAnte(10)
		mustDeal(t, g)
		push, win := g.VisibleFloor()
		if push != tc.wantPush || win != tc.wantWin {
			t.Errorf("%s: floor push=%v win=%v, want %v %v", tc.name, push, win, tc.wantPush, tc.wantWin)
		}
	}
}

// ---- NextHand carry / clamp / game over ----

func TestNextHandCarriesAnte(t *testing.T) {
	g := NewGameWithDeck(1000, sampleDeck())
	_ = g.SetAnte(50)
	mustDeal(t, g)
	mustRaise(t, g, 1)
	mustRaise(t, g, 1)
	mustRaise(t, g, 1)
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseBetting || g.Ante() != 50 {
		t.Errorf("after NextHand phase %v ante %d, want betting / 50", g.Phase(), g.Ante())
	}
	if g.TotalWagered() != 0 {
		t.Errorf("total wagered should reset before deal, got %d", g.TotalWagered())
	}
}

func TestNextHandClampsAnteToBankroll(t *testing.T) {
	// Lose the whole ante down to a bankroll below the carried ante.
	g := NewGameWithDeck(60, sampleDeck())
	_ = g.SetAnte(50)
	mustDeal(t, g)
	if err := g.Fold(); err != nil { // forfeit 50 => bankroll 10
		t.Fatal(err)
	}
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Ante() != 10 {
		t.Errorf("clamped ante = %d, want 10", g.Ante())
	}
}

func TestGameOverWhenBroke(t *testing.T) {
	// Bankroll 3, ante 3: fold forfeits it, leaving 0 < MinBet => game over.
	g := NewGameWithDeck(3, sampleDeck())
	mustDeal(t, g) // ante defaults to MinBet = 3
	if err := g.Fold(); err != nil {
		t.Fatal(err)
	}
	if g.Bankroll() != 0 {
		t.Fatalf("bankroll = %d, want 0", g.Bankroll())
	}
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.Phase() != PhaseGameOver {
		t.Errorf("phase = %v, want game-over", g.Phase())
	}
}

// ---- Net-equals-bankroll-delta invariant across outcomes ----

func TestNetMatchesBankrollDelta(t *testing.T) {
	cases := []struct {
		name  string
		ante  int
		cards []Card
		mults [3]int
	}{
		{"four of a kind win", 30, deck(c(Seven, Spades), c(Seven, Hearts), c(Seven, Clubs), c(Seven, Diamonds), c(Two, Spades)), [3]int{3, 3, 3}},
		{"straight win", 12, deck(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs), c(Six, Diamonds), c(Five, Spades)), [3]int{2, 1, 3}},
		{"pair of eights push", 20, deck(c(Eight, Spades), c(Eight, Hearts), c(Nine, Clubs), c(Four, Diamonds), c(Two, Spades)), [3]int{1, 2, 1}},
		{"high card loss", 15, deck(c(King, Spades), c(Nine, Hearts), c(Seven, Clubs), c(Four, Diamonds), c(Two, Spades)), [3]int{1, 1, 1}},
	}
	for _, tc := range cases {
		// playThrough already asserts Net == bankroll delta.
		s, _ := playThrough(t, 1000, tc.ante, tc.cards, tc.mults)
		if s.TotalWagered <= 0 {
			t.Errorf("%s: total wagered = %d, want > 0", tc.name, s.TotalWagered)
		}
	}
}

// ---- Pay-table configurability (Barona: Straight 5:1) ----

func TestPayTableConfigurable(t *testing.T) {
	old := StraightPayout
	StraightPayout = 5 // Barona variant
	defer func() { StraightPayout = old }()

	// Ante 10, 1x/1x/1x => total 40; Straight now pays 5:1 => +200.
	straight := deck(c(Nine, Spades), c(Eight, Hearts), c(Seven, Clubs), c(Six, Diamonds), c(Five, Spades))
	s, delta := playThrough(t, 1000, 10, straight, [3]int{1, 1, 1})
	if s.Category != Straight {
		t.Fatalf("category = %v, want Straight", s.Category)
	}
	if s.Multiplier != 5 || s.Net != 200 || delta != 200 {
		t.Errorf("Barona straight: mult %d net %d delta %d, want 5 / 200 / 200", s.Multiplier, s.Net, delta)
	}
}

// sampleDeck is a five-card stacked deck whose final hand is a plain King-high
// (a loss), used by tests that care about flow, folds, and bankroll rather than
// the paid category.
func sampleDeck() []Card {
	return deck(
		c(King, Spades), c(Nine, Hearts), // hole
		c(Seven, Clubs), c(Four, Diamonds), c(Two, Spades), // community
	)
}
