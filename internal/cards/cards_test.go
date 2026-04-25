package cards

// MIK-3083: tests for the multi-card reward ranker.

import (
	"strings"
	"testing"
)

func amexGold() Card {
	return Card{
		Name: "FB Amex Gold",
		MCCMultipliers: map[string]float64{
			"travel":  4,
			"airline": 4,
			"hotel":   3,
			"default": 1,
		},
		PointValueEUR: 0.012,
		IntroOffer:    "60k pts after 2k EUR spend",
		FXFeePct:      0,
	}
}

func visaCurve() Card {
	return Card{
		Name: "Visa Curve",
		MCCMultipliers: map[string]float64{
			"default": 1,
		},
		PointValueEUR: 0.005,
		FXFeePct:      0.5,
	}
}

func mastercardN26() Card {
	return Card{
		Name: "MasterCard N26",
		MCCMultipliers: map[string]float64{
			"default": 0,
		},
		PointValueEUR: 0.005,
		FXFeePct:      0,
	}
}

func TestRank_AirlinePurchaseFavoursHighMultiplierCard(t *testing.T) {
	booking := Booking{PriceEUR: 500, MCC: "airline"}
	got := Rank([]Card{visaCurve(), amexGold(), mastercardN26()}, booking)
	if len(got) != 3 {
		t.Fatalf("got %d rankings, want 3", len(got))
	}
	if got[0].Card.Name != "FB Amex Gold" {
		t.Errorf("top card = %q, want FB Amex Gold (×4 on airline)", got[0].Card.Name)
	}
	// Sanity: rewards on Amex should beat Curve's by at least the
	// 4x multiplier × 0.012 EUR/pt × 500 EUR ratio.
	if got[0].RewardValueEUR <= got[1].RewardValueEUR {
		t.Errorf("Amex reward %.2f <= next %.2f", got[0].RewardValueEUR, got[1].RewardValueEUR)
	}
	if !strings.Contains(got[0].Reasoning, "FB Amex Gold") {
		t.Errorf("Reasoning missing card name: %q", got[0].Reasoning)
	}
	if !strings.Contains(got[0].Reasoning, "intro") {
		t.Errorf("Reasoning should mention intro offer: %q", got[0].Reasoning)
	}
}

func TestRank_ForeignCurrencyAppliesFXFee(t *testing.T) {
	booking := Booking{PriceEUR: 500, MCC: "default", ForeignCurrency: true}
	got := Rank([]Card{visaCurve()}, booking)
	if got[0].FXFeeEUR <= 0 {
		t.Errorf("FXFeeEUR = %.2f, want > 0 (foreign + Curve has 0.5%% fee)", got[0].FXFeeEUR)
	}
	if !strings.Contains(got[0].Reasoning, "FX fee") {
		t.Errorf("Reasoning should mention FX fee: %q", got[0].Reasoning)
	}
}

func TestRank_HomeCurrencySkipsFXFee(t *testing.T) {
	booking := Booking{PriceEUR: 500, MCC: "default", ForeignCurrency: false}
	got := Rank([]Card{visaCurve()}, booking)
	if got[0].FXFeeEUR != 0 {
		t.Errorf("home currency FXFeeEUR = %.2f, want 0", got[0].FXFeeEUR)
	}
}

func TestRank_NoCardsReturnsEmpty(t *testing.T) {
	got := Rank(nil, Booking{PriceEUR: 100, MCC: "default"})
	if len(got) != 0 {
		t.Errorf("Rank(nil) returned %d rankings, want 0", len(got))
	}
	if Recommend(got) != nil {
		t.Errorf("Recommend(empty) should return nil")
	}
}

func TestRank_NetCostNeverNegativeWithoutIntroBoost(t *testing.T) {
	// Even an aggressive 4x multiplier × 0.012 EUR/pt = 4.8% earn rate
	// on a 100 EUR booking is only 4.8 EUR back → net cost stays positive.
	booking := Booking{PriceEUR: 100, MCC: "airline"}
	got := Rank([]Card{amexGold()}, booking)
	if got[0].NetCostEUR <= 0 {
		t.Errorf("NetCostEUR = %.2f, want > 0 (rewards should not exceed price at realistic rates)", got[0].NetCostEUR)
	}
}

func TestLookupMultiplier_FallsBackToDefault(t *testing.T) {
	m := map[string]float64{"default": 1.5}
	if got := lookupMultiplier(m, "groceries"); got != 1.5 {
		t.Errorf("missing key fallback = %v, want 1.5 (default)", got)
	}
	if got := lookupMultiplier(m, "DEFAULT"); got != 1.5 {
		t.Errorf("case-insensitive default lookup = %v, want 1.5", got)
	}
}

func TestLookupMultiplier_NilMapReturnsOne(t *testing.T) {
	if got := lookupMultiplier(nil, "anything"); got != 1.0 {
		t.Errorf("nil map = %v, want 1.0", got)
	}
}

func TestLookupMultiplier_EmptyMCCFallsBack(t *testing.T) {
	m := map[string]float64{"default": 2}
	if got := lookupMultiplier(m, "  "); got != 2 {
		t.Errorf("blank mcc = %v, want 2 (default)", got)
	}
}

func TestRank_StableOrderingForTies(t *testing.T) {
	a := Card{Name: "A", MCCMultipliers: map[string]float64{"default": 1}, PointValueEUR: 0.01}
	b := Card{Name: "B", MCCMultipliers: map[string]float64{"default": 1}, PointValueEUR: 0.01}
	got := Rank([]Card{a, b}, Booking{PriceEUR: 100, MCC: "default"})
	if got[0].Card.Name != "A" || got[1].Card.Name != "B" {
		t.Errorf("stable ordering broken: got %q,%q want A,B", got[0].Card.Name, got[1].Card.Name)
	}
}

func TestMCCLabel(t *testing.T) {
	if got := mccLabel(""); !strings.Contains(got, "uncategorised") {
		t.Errorf("empty MCC label = %q, want 'uncategorised'", got)
	}
	if got := mccLabel("travel"); got != "travel" {
		t.Errorf("MCC label passthrough = %q", got)
	}
}
