package railpass

// MIK-3086 (partial): tests for the rail-pass break-even calculator.

import (
	"strings"
	"testing"
)

func interrailGlobal7() PassOption {
	return PassOption{Name: "Interrail Global 7-in-1month", Cost: 416, Days: 7}
}

func amsToBcnSegments() []PointToPointSegment {
	// Realistic walk-up flexible fares for a 5-leg AMS->BCN run.
	return []PointToPointSegment{
		{Operator: "Eurostar", Origin: "AMS", Destination: "PAR", Date: "2026-06-15", Price: 220, ReservationFee: 30},
		{Operator: "TGV", Origin: "PAR", Destination: "LYS", Date: "2026-06-17", Price: 95, ReservationFee: 10},
		{Operator: "Renfe-AVE", Origin: "LYS", Destination: "BCN", Date: "2026-06-19", Price: 120, ReservationFee: 15},
		{Operator: "Renfe", Origin: "BCN", Destination: "MAD", Date: "2026-06-21", Price: 75, ReservationFee: 0},
		{Operator: "Renfe-AVE", Origin: "MAD", Destination: "SVQ", Date: "2026-06-23", Price: 85, ReservationFee: 5},
	}
}

func TestEvaluatePass_BuyVerdictWhenPassCheaper(t *testing.T) {
	got := EvaluatePass(interrailGlobal7(), amsToBcnSegments())
	if got.Verdict != VerdictBuyPass {
		t.Errorf("Verdict=%q, want %q", got.Verdict, VerdictBuyPass)
	}
	// p2p = 220+95+120+75+85 = 595; pass+fees = 416 + 60 = 476; savings = 119.
	if got.PointToPointTotal != 595 {
		t.Errorf("PointToPointTotal=%.2f, want 595", got.PointToPointTotal)
	}
	if got.PassTotalEffective != 476 {
		t.Errorf("PassTotalEffective=%.2f, want 476", got.PassTotalEffective)
	}
	if got.Savings != 119 {
		t.Errorf("Savings=%.2f, want 119", got.Savings)
	}
}

func TestEvaluatePass_SkipVerdictWhenPassMoreExpensive(t *testing.T) {
	cheap := []PointToPointSegment{
		{Operator: "Renfe", Origin: "BCN", Destination: "MAD", Date: "2026-06-21", Price: 30},
		{Operator: "Renfe", Origin: "MAD", Destination: "SVQ", Date: "2026-06-23", Price: 35},
	}
	got := EvaluatePass(interrailGlobal7(), cheap)
	if got.Verdict != VerdictSkipPass {
		t.Errorf("Verdict=%q, want %q", got.Verdict, VerdictSkipPass)
	}
}

func TestEvaluatePass_MarginalWhenWithinTenPercent(t *testing.T) {
	// p2p = 400; pass = 416 (no fees). Gap = -16 (-4%) → marginal.
	pass := PassOption{Name: "Interrail Global 5-day", Cost: 416, Days: 5, ReservationsIncluded: true}
	segs := []PointToPointSegment{
		{Operator: "X", Origin: "A", Destination: "B", Price: 100},
		{Operator: "X", Origin: "B", Destination: "C", Price: 100},
		{Operator: "X", Origin: "C", Destination: "D", Price: 100},
		{Operator: "X", Origin: "D", Destination: "E", Price: 100},
	}
	got := EvaluatePass(pass, segs)
	if got.Verdict != VerdictMarginal {
		t.Errorf("Verdict=%q, want %q", got.Verdict, VerdictMarginal)
	}
}

func TestEvaluatePass_BreakEvenCount(t *testing.T) {
	// avg = 595/5 = 119; pass cost 416; break-even = ceil(416/119) = 4.
	got := EvaluatePass(interrailGlobal7(), amsToBcnSegments())
	if got.BreakEvenSegments != 4 {
		t.Errorf("BreakEvenSegments=%d, want 4", got.BreakEvenSegments)
	}
}

func TestEvaluatePass_ReservationsIncludedSkipsFees(t *testing.T) {
	pass := interrailGlobal7()
	pass.ReservationsIncluded = true
	got := EvaluatePass(pass, amsToBcnSegments())
	if got.PassTotalEffective != 416 {
		t.Errorf("PassTotalEffective=%.2f, want 416 (fees included in pass)", got.PassTotalEffective)
	}
}

func TestEvaluatePass_ZeroSegmentsShortCircuits(t *testing.T) {
	got := EvaluatePass(interrailGlobal7(), nil)
	if got.Verdict != "" {
		t.Errorf("Verdict=%q, want zero-value when no segments", got.Verdict)
	}
	if !strings.Contains(got.Reason, "no priced segments") {
		t.Errorf("Reason=%q should explain", got.Reason)
	}
}

func TestEvaluatePass_NonPositivePriceSkipped(t *testing.T) {
	got := EvaluatePass(interrailGlobal7(), []PointToPointSegment{
		{Operator: "Eurostar", Price: 220},
		{Operator: "Bad", Price: 0},
		{Operator: "Bad2", Price: -10},
	})
	if got.SegmentsScored != 1 {
		t.Errorf("SegmentsScored=%d, want 1 (zero/negative skipped)", got.SegmentsScored)
	}
}

func TestEvaluateAll_SortsByEffectiveCost(t *testing.T) {
	passes := []PassOption{
		{Name: "Interrail Global 4-day", Cost: 286, Days: 4},
		{Name: "Interrail Global 7-day", Cost: 416, Days: 7},
		{Name: "Interrail Global 10-day", Cost: 516, Days: 10},
	}
	got := EvaluateAll(passes, amsToBcnSegments())
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].PassTotalEffective > got[i].PassTotalEffective {
			t.Errorf("not sorted ascending: %.2f then %.2f", got[i-1].PassTotalEffective, got[i].PassTotalEffective)
		}
	}
}

func TestEvaluateAll_FailedScoresSortLast(t *testing.T) {
	passes := []PassOption{
		{Name: "Pass-A", Cost: 100},
		{Name: "Pass-B", Cost: 200},
	}
	// nil segments → both fail to evaluate.
	got := EvaluateAll(passes, nil)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	for _, r := range got {
		if r.PassTotalEffective != 0 {
			t.Errorf("expected zero PassTotalEffective when no segments; got %v", r)
		}
	}
}

func TestEvaluatePass_HighSavingsTriggersBuyVerbatimReason(t *testing.T) {
	got := EvaluatePass(interrailGlobal7(), amsToBcnSegments())
	if !strings.Contains(got.Reason, "buy") {
		t.Errorf("Reason=%q should mention buy", got.Reason)
	}
}
