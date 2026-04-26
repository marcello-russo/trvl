package multidest

// MIK-3080 (partial): tests for the two-stage screen + drill-down ranker.

import (
	"strings"
	"testing"
)

func sampleOrderings() []Ordering {
	return []Ordering{
		{
			Cities: []string{"AMS", "ROM", "BCN"},
			Legs: []Leg{
				{Origin: "AMS", Destination: "ROM", Date: "2026-06-01", Price: 90, Carrier: "FR"},
				{Origin: "ROM", Destination: "BCN", Date: "2026-06-04", Price: 75, Carrier: "VY"},
				{Origin: "BCN", Destination: "AMS", Date: "2026-06-08", Price: 110, Carrier: "VY"},
			},
			Hotels: []HotelCost{
				{City: "ROM", CheckIn: "2026-06-01", CheckOut: "2026-06-04", TotalPrice: 240},
				{City: "BCN", CheckIn: "2026-06-04", CheckOut: "2026-06-08", TotalPrice: 320},
			},
		},
		{
			Cities: []string{"AMS", "BCN", "ROM"},
			Legs: []Leg{
				{Origin: "AMS", Destination: "BCN", Date: "2026-06-01", Price: 95},
				{Origin: "BCN", Destination: "ROM", Date: "2026-06-04", Price: 80},
				{Origin: "ROM", Destination: "AMS", Date: "2026-06-08", Price: 100},
			},
			Hotels: []HotelCost{
				{City: "BCN", TotalPrice: 280},
				{City: "ROM", TotalPrice: 250},
			},
		},
		{
			Cities: []string{"AMS", "ROM", "MAD"},
			Legs: []Leg{
				{Origin: "AMS", Destination: "ROM", Price: 90},
				{Origin: "ROM", Destination: "MAD", Price: 65},
				{Origin: "MAD", Destination: "AMS", Price: 130},
			},
			Hotels: []HotelCost{
				{City: "ROM", TotalPrice: 240},
				{City: "MAD", TotalPrice: 200},
			},
		},
		{
			Cities: []string{"AMS", "MAD", "ROM"},
			Legs: []Leg{
				{Origin: "AMS", Destination: "MAD", Price: 105},
				{Origin: "MAD", Destination: "ROM", Price: 70},
				{Origin: "ROM", Destination: "AMS", Price: 100},
			},
			Hotels: []HotelCost{
				{City: "MAD", TotalPrice: 220},
				{City: "ROM", TotalPrice: 250},
			},
		},
	}
}

func TestScreen_RanksAscendingByFlightTotal(t *testing.T) {
	got := Screen(sampleOrderings(), ScreenOptions{TopKAfterScreen: 4})
	if len(got) != 4 {
		t.Fatalf("got %d, want 4", len(got))
	}
	for i := 1; i < len(got); i++ {
		if flightTotal(got[i-1]) > flightTotal(got[i]) {
			t.Errorf("not sorted ascending: %v then %v", flightTotal(got[i-1]), flightTotal(got[i]))
		}
	}
}

func TestScreen_TopKDefaultsTo3(t *testing.T) {
	got := Screen(sampleOrderings(), ScreenOptions{})
	if len(got) != 3 {
		t.Errorf("default TopKAfterScreen should be 3, got %d", len(got))
	}
}

func TestScreen_DropsLeglessOrderings(t *testing.T) {
	in := append(sampleOrderings(), Ordering{Cities: []string{"X", "Y"}})
	got := Screen(in, ScreenOptions{TopKAfterScreen: 99})
	for _, o := range got {
		if len(o.Legs) == 0 {
			t.Errorf("legless ordering survived: %v", o)
		}
	}
}

func TestDrillDown_SortsByGrandTotal(t *testing.T) {
	screened := Screen(sampleOrderings(), ScreenOptions{TopKAfterScreen: 4})
	got := DrillDown(screened, ScreenOptions{TopKFinal: 4})
	if len(got) != 4 {
		t.Fatalf("got %d, want 4", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].GrandTotal > got[i].GrandTotal {
			t.Errorf("not sorted by grand total: %v then %v", got[i-1].GrandTotal, got[i].GrandTotal)
		}
	}
}

func TestDrillDown_RankAndReasonStrings(t *testing.T) {
	got := ScreenAndDrillDown(sampleOrderings(), ScreenOptions{TopKAfterScreen: 4, TopKFinal: 3})
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	if got[0].Rank != 1 {
		t.Errorf("first rank=%d, want 1", got[0].Rank)
	}
	if !strings.Contains(got[0].Reason, "cheapest") {
		t.Errorf("first Reason=%q should mention cheapest", got[0].Reason)
	}
	if !strings.Contains(got[1].Reason, "more") {
		t.Errorf("second Reason=%q should mention extra cost", got[1].Reason)
	}
}

func TestDrillDown_FlightHotelTotalsAddUp(t *testing.T) {
	got := ScreenAndDrillDown(sampleOrderings(), ScreenOptions{TopKAfterScreen: 4, TopKFinal: 4})
	for _, b := range got {
		if b.GrandTotal != b.FlightTotal+b.HotelTotal {
			t.Errorf("Cities=%v: GrandTotal=%.2f != Flight %.2f + Hotel %.2f", b.Cities, b.GrandTotal, b.FlightTotal, b.HotelTotal)
		}
	}
}

func TestDrillDown_TopKFinalDefaultsTo3(t *testing.T) {
	got := ScreenAndDrillDown(sampleOrderings(), ScreenOptions{TopKAfterScreen: 4})
	if len(got) != 3 {
		t.Errorf("default TopKFinal should be 3, got %d", len(got))
	}
}

func TestScreenAndDrillDown_NilOnEmptyInput(t *testing.T) {
	if got := ScreenAndDrillDown(nil, ScreenOptions{}); got != nil {
		t.Errorf("nil orderings should yield nil, got %v", got)
	}
}

func TestScreen_TopKExceedsLength(t *testing.T) {
	got := Screen(sampleOrderings(), ScreenOptions{TopKAfterScreen: 99})
	if len(got) != 4 {
		t.Errorf("TopK=99 over 4 should clip to 4, got %d", len(got))
	}
}

func TestDrillDown_PreservesLegsAndHotels(t *testing.T) {
	screened := Screen(sampleOrderings(), ScreenOptions{TopKAfterScreen: 1})
	got := DrillDown(screened, ScreenOptions{TopKFinal: 1})
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if len(got[0].Legs) == 0 || len(got[0].Hotels) == 0 {
		t.Errorf("Legs/Hotels not propagated: %+v", got[0])
	}
}
