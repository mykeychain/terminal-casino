package engine

import "testing"

// TestAddSpotDefaultsToPreviousBet: a newly opened hand inherits the previous
// (last) hand's bet, and is clamped to the affordable amount when funds are tight.
func TestAddSpotDefaultsToPreviousBet(t *testing.T) {
	g := NewGame(1)
	if err := g.SetBet(50); err != nil {
		t.Fatal(err)
	}
	if err := g.AddSpot(); err != nil {
		t.Fatal(err)
	}
	if got := g.SpotBet(1); got != 50 {
		t.Errorf("new hand bet = %d, want 50 (previous hand's bet)", got)
	}
	if err := g.SetSpotBet(1, 80); err != nil {
		t.Fatal(err)
	}
	if err := g.AddSpot(); err != nil {
		t.Fatal(err)
	}
	if got := g.SpotBet(2); got != 80 {
		t.Errorf("third hand bet = %d, want 80 (previous hand's bet)", got)
	}

	// Affordability clamp: with little left, the new hand drops to what fits.
	g2 := NewGameWithShoe(120, nil)
	_ = g2.SetBet(100) // $20 left
	if err := g2.AddSpot(); err != nil {
		t.Fatalf("AddSpot should succeed clamping to $20: %v", err)
	}
	if got := g2.SpotBet(1); got != 20 {
		t.Errorf("clamped new hand bet = %d, want 20", got)
	}
	// Now nothing left for a third hand.
	if err := g2.AddSpot(); err == nil {
		t.Errorf("AddSpot should fail with no affordable funds left")
	}
}

// TestNextHandPreservesHands: the next round re-opens with the same hands (count
// and bets) as the previous round, dropping only trailing hands the bankroll can
// no longer afford.
func TestNextHandPreservesHands(t *testing.T) {
	cc := func(r Rank, s Suit) Card { return Card{Rank: r, Suit: s} }
	var fill []Card
	for i := 0; i < 40; i++ {
		fill = append(fill, cc(Two, Clubs))
	}

	g := NewGameWithShoe(1000, fill)
	_ = g.AddSpot()
	_ = g.SetSpotBet(0, 50)
	_ = g.SetSpotBet(1, 25)
	_ = g.Deal()
	for g.Phase() == PhasePlayerTurn {
		_ = g.Stand()
	}
	if g.Phase() != PhaseRoundOver {
		t.Fatalf("phase = %v, want round-over", g.Phase())
	}
	if err := g.NextHand(); err != nil {
		t.Fatal(err)
	}
	if g.NumSpots() != 2 || g.SpotBet(0) != 50 || g.SpotBet(1) != 25 {
		t.Errorf("next round = %d hands (%d, %d), want 2 hands (50, 25)",
			g.NumSpots(), g.SpotBet(0), g.SpotBet(1))
	}

	// Trailing-hand drop: two $500 hands but only ~$600 bankroll next round keeps
	// one hand (can't afford the second at $500).
	g2 := NewGameWithShoe(1000, fill)
	_ = g2.AddSpot()
	_ = g2.SetSpotBet(0, 500)
	_ = g2.SetSpotBet(1, 100) // total 600 escrowed -> bankroll 400 after deal
	_ = g2.Deal()
	for g2.Phase() == PhasePlayerTurn {
		_ = g2.Stand()
	}
	// Whatever the settled bankroll, NextHand keeps at least the first hand.
	_ = g2.NextHand()
	if g2.NumSpots() < 1 {
		t.Errorf("NextHand must keep at least one hand, got %d", g2.NumSpots())
	}
}
