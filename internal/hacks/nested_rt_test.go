package hacks

// MIK-3076: tests for the nested round-trip combinator.

import (
	"testing"
)

// canonicalWindows builds two visit windows priced so that the
// nested-from-B pairing wins by a clear margin — used to validate the
// canonical "overlapping round-trip" outcome plus all derived savings.
func canonicalWindows() (VisitWindow, VisitWindow) {
	w1 := VisitWindow{
		DepartDate:     "2026-06-01",
		ReturnDate:     "2026-06-05",
		LegFromAToB:    180,
		LegFromBToA:    180,
		RoundTripFromA: 280, // long-stay round-trip is cheap
		RoundTripFromB: 260,
	}
	w2 := VisitWindow{
		DepartDate:     "2026-06-15",
		ReturnDate:     "2026-06-19",
		LegFromAToB:    180,
		LegFromBToA:    180,
		RoundTripFromA: 320, // short-stay quote is more expensive
		RoundTripFromB: 220, // B-rooted quote is the cheapest piece
	}
	return w1, w2
}

func TestEnumeratePairings_AllVariantsReturnedWhenPriced(t *testing.T) {
	w1, w2 := canonicalWindows()
	got := EnumeratePairings(w1, w2)
	if len(got) != 4 {
		t.Fatalf("expected 4 pairings, got %d", len(got))
	}
	want := map[PairingKind]bool{
		PairingNaive:       false,
		PairingTwoOneWays:  false,
		PairingNestedFromA: false,
		PairingNestedFromB: false,
	}
	for _, p := range got {
		if _, ok := want[p.Kind]; !ok {
			t.Errorf("unexpected kind %q", p.Kind)
		}
		want[p.Kind] = true
	}
	for k, present := range want {
		if !present {
			t.Errorf("missing pairing kind %q", k)
		}
	}
}

func TestEnumeratePairings_SkipsWhenWindowUnpriced(t *testing.T) {
	w1, w2 := canonicalWindows()
	w2.LegFromAToB = 0 // unpriced
	if got := EnumeratePairings(w1, w2); got != nil {
		t.Errorf("expected nil when window unpriced, got %v", got)
	}
}

func TestEnumeratePairings_NestedFromBSkippedWhenRoundTripFromBMissing(t *testing.T) {
	w1, w2 := canonicalWindows()
	w2.RoundTripFromB = 0
	got := EnumeratePairings(w1, w2)
	for _, p := range got {
		if p.Kind == PairingNestedFromB {
			t.Errorf("PairingNestedFromB should be skipped when RoundTripFromB is missing")
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 pairings, got %d", len(got))
	}
}

func TestRank_SortsAscendingByCost(t *testing.T) {
	in := []PairingResult{
		{Kind: PairingNaive, Cost: 600},
		{Kind: PairingNestedFromB, Cost: 500},
		{Kind: PairingTwoOneWays, Cost: 720},
	}
	got := Rank(in, 0)
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	if got[0].Cost > got[1].Cost || got[1].Cost > got[2].Cost {
		t.Errorf("not sorted ascending: %+v", got)
	}
}

func TestRank_SavingsRelativeToNaiveBaseline(t *testing.T) {
	w1, w2 := canonicalWindows()
	got := EnumerateAndRank(w1, w2, 0)

	var naiveCost float64
	for _, p := range got {
		if p.Kind == PairingNaive {
			naiveCost = p.Cost
		}
	}
	if naiveCost == 0 {
		t.Fatalf("expected naive baseline in results")
	}

	for _, p := range got {
		wantSavings := naiveCost - p.Cost
		if wantSavings < 0 {
			wantSavings = 0
		}
		if p.SavingsEUR != wantSavings {
			t.Errorf("Kind=%q SavingsEUR=%.2f, want %.2f", p.Kind, p.SavingsEUR, wantSavings)
		}
	}
}

func TestRank_NestedFromBWinsCanonicalScenario(t *testing.T) {
	w1, w2 := canonicalWindows()
	got := EnumerateAndRank(w1, w2, 1)
	if len(got) != 1 {
		t.Fatalf("got %d top results, want 1", len(got))
	}
	if got[0].Kind != PairingNestedFromB {
		t.Errorf("top pairing = %q, want %q (overlapping round-trip should win)", got[0].Kind, PairingNestedFromB)
	}
}

func TestRank_KClippedToCandidateCount(t *testing.T) {
	w1, w2 := canonicalWindows()
	got := EnumerateAndRank(w1, w2, 99)
	if len(got) != 4 {
		t.Errorf("K=99 should clip to 4 candidates, got %d", len(got))
	}
}

func TestRank_KZeroReturnsAll(t *testing.T) {
	w1, w2 := canonicalWindows()
	got := EnumerateAndRank(w1, w2, 0)
	if len(got) != 4 {
		t.Errorf("K=0 should return all candidates, got %d", len(got))
	}
}

func TestNaivePairing_PrefersRoundTripWhenCheaper(t *testing.T) {
	// Two one-ways = 180+180 = 360; round-trip = 280 → RT wins.
	w1, w2 := canonicalWindows()
	got := EnumeratePairings(w1, w2)
	for _, p := range got {
		if p.Kind == PairingNaive {
			want := w1.RoundTripFromA + w2.RoundTripFromA
			if p.Cost != want {
				t.Errorf("naive cost = %.2f, want %.2f", p.Cost, want)
			}
		}
	}
}

func TestNaivePairing_FallsBackToOneWaysWhenRoundTripMissing(t *testing.T) {
	w1, w2 := canonicalWindows()
	w1.RoundTripFromA = 0
	w2.RoundTripFromA = 0
	got := EnumeratePairings(w1, w2)
	for _, p := range got {
		if p.Kind == PairingNaive {
			want := w1.LegFromAToB + w1.LegFromBToA + w2.LegFromAToB + w2.LegFromBToA
			if p.Cost != want {
				t.Errorf("naive cost = %.2f, want %.2f (one-way fallback)", p.Cost, want)
			}
		}
	}
}

func TestRank_SavingsClampedAtZeroWhenAllAboveBaseline(t *testing.T) {
	in := []PairingResult{
		{Kind: PairingNaive, Cost: 500},
		{Kind: PairingTwoOneWays, Cost: 720}, // above baseline
	}
	got := Rank(in, 0)
	for _, p := range got {
		if p.SavingsEUR < 0 {
			t.Errorf("Kind=%q SavingsEUR=%.2f, want clamped at 0", p.Kind, p.SavingsEUR)
		}
	}
}

func TestEnumerateAndRank_NilOnUnpriced(t *testing.T) {
	w1, _ := canonicalWindows()
	if got := EnumerateAndRank(w1, VisitWindow{}, 3); got != nil {
		t.Errorf("unpriced window should produce nil, got %v", got)
	}
}

func TestRank_LegBreakdownPreserved(t *testing.T) {
	w1, w2 := canonicalWindows()
	got := EnumerateAndRank(w1, w2, 0)
	for _, p := range got {
		if p.LegBreakdown == nil {
			t.Errorf("Kind=%q has nil LegBreakdown", p.Kind)
		}
		var sum float64
		for _, v := range p.LegBreakdown {
			sum += v
		}
		if sum != p.Cost {
			t.Errorf("Kind=%q breakdown sum %.2f != Cost %.2f", p.Kind, sum, p.Cost)
		}
	}
}
