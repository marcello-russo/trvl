package optimizer

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestValidateInput_missing_origin(t *testing.T) {
	err := validateInput(OptimizeInput{
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	})
	if err == nil {
		t.Fatal("expected error for missing origin")
	}
}

func TestValidateInput_missing_destination(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:     "HEL",
		DepartDate: "2026-06-01",
	})
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
}

func TestValidateInput_missing_depart_date(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if err == nil {
		t.Fatal("expected error for missing departure date")
	}
}

func TestValidateInput_invalid_depart_date(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for invalid departure date")
	}
}

func TestValidateInput_invalid_return_date(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		ReturnDate:  "bad",
	})
	if err == nil {
		t.Fatal("expected error for invalid return date")
	}
}

func TestValidateInput_same_origin_dest(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "HEL",
		DepartDate:  "2026-06-01",
	})
	if err == nil {
		t.Fatal("expected error when origin == destination")
	}
}

func TestValidateInput_same_origin_dest_case_insensitive(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "hel",
		Destination: "HEL",
		DepartDate:  "2026-06-01",
	})
	if err == nil {
		t.Fatal("expected error when origin == destination (case insensitive)")
	}
}

func TestValidateInput_valid(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInput_valid_roundtrip(t *testing.T) {
	err := validateInput(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		ReturnDate:  "2026-06-08",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaults(t *testing.T) {
	in := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	in.defaults()

	if in.Guests != 1 {
		t.Errorf("expected Guests=1, got %d", in.Guests)
	}
	if in.FlexDays != 3 {
		t.Errorf("expected FlexDays=3, got %d", in.FlexDays)
	}
	if in.MaxResults != 5 {
		t.Errorf("expected MaxResults=5, got %d", in.MaxResults)
	}
	if in.MaxAPICalls != 15 {
		t.Errorf("expected MaxAPICalls=15, got %d", in.MaxAPICalls)
	}
	if in.Currency != "EUR" {
		t.Errorf("expected Currency=EUR, got %s", in.Currency)
	}
}

func TestDefaults_preserves_explicit(t *testing.T) {
	in := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		Guests:      2,
		FlexDays:    5,
		MaxResults:  10,
		MaxAPICalls: 20,
		Currency:    "USD",
	}
	in.defaults()

	if in.Guests != 2 {
		t.Errorf("expected Guests=2, got %d", in.Guests)
	}
	if in.FlexDays != 5 {
		t.Errorf("expected FlexDays=5, got %d", in.FlexDays)
	}
	if in.MaxResults != 10 {
		t.Errorf("expected MaxResults=10, got %d", in.MaxResults)
	}
	if in.MaxAPICalls != 20 {
		t.Errorf("expected MaxAPICalls=20, got %d", in.MaxAPICalls)
	}
	if in.Currency != "USD" {
		t.Errorf("expected Currency=USD, got %s", in.Currency)
	}
}

func TestExpandCandidates_baseline_always_present(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	if len(candidates) == 0 {
		t.Fatal("expected at least baseline candidate")
	}

	baseline := candidates[0]
	if baseline.origin != "HEL" {
		t.Errorf("baseline origin: got %s, want HEL", baseline.origin)
	}
	if baseline.dest != "BCN" {
		t.Errorf("baseline dest: got %s, want BCN", baseline.dest)
	}
	if baseline.strategy != "Direct booking" {
		t.Errorf("baseline strategy: got %q, want %q", baseline.strategy, "Direct booking")
	}
	if len(baseline.hackTypes) != 0 {
		t.Errorf("baseline should have no hackTypes, got %v", baseline.hackTypes)
	}
}

func TestExpandCandidates_HEL_has_alternatives(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// HEL has nearby airports (TLL, RIX, VNO) and multimodal hubs (TLL, RIX, ARN).
	// Must have more than just the baseline.
	if len(candidates) < 3 {
		t.Errorf("expected at least 3 candidates for HEL->BCN, got %d", len(candidates))
	}

	// Check that TLL appears as a positioning alternative.
	found := false
	for _, c := range candidates {
		if c.origin == "TLL" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected TLL as alternative origin for HEL")
	}
}

func TestExpandCandidates_BCN_destination_alternatives(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// BCN has destination alternative: GRO (Girona).
	found := false
	for _, c := range candidates {
		if c.dest == "GRO" {
			found = true
			if c.transferCost <= 0 {
				t.Error("expected transfer cost for GRO destination alternative")
			}
			break
		}
	}
	if !found {
		t.Error("expected GRO as alternative destination for BCN")
	}
}

func TestExpandCandidates_AMS_rail_fly(t *testing.T) {
	input := OptimizeInput{
		Origin:      "AMS",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// AMS is a KLM hub with rail+fly stations: ZWE (Antwerp), ZYR (Brussels-Midi).
	foundZWE := false
	for _, c := range candidates {
		if c.origin == "ZWE" {
			foundZWE = true
			if c.transferCost != 0 {
				t.Errorf("rail+fly transfer cost should be 0 (included in ticket), got %f", c.transferCost)
			}
			hackFound := false
			for _, h := range c.hackTypes {
				if h == "rail_fly_arbitrage" {
					hackFound = true
				}
			}
			if !hackFound {
				t.Error("expected rail_fly_arbitrage hack type for ZWE")
			}
			break
		}
	}
	if !foundZWE {
		t.Error("expected ZWE as rail+fly origin for AMS hub")
	}
}

func TestExpandCandidates_unknown_origin_unknown_dest(t *testing.T) {
	input := OptimizeInput{
		Origin:      "XYZ",
		Destination: "QQQ",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// Unknown origin + unknown destination: baseline + date-flex candidates only.
	// FlexDays defaults to 3, so we get 1 baseline + 6 date-flex = 7.
	if len(candidates) != 7 {
		t.Errorf("expected 7 candidates for unknown origin+dest (1 baseline + 6 date-flex), got %d", len(candidates))
	}
}

func TestExpandCandidates_skips_self_referential(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "TLL",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// TLL should not appear as alternative origin when TLL is the destination.
	for _, c := range candidates {
		if c.origin == "TLL" && c.dest == "TLL" {
			t.Error("candidate should not have same origin and destination")
		}
	}
}

func TestPriceCandidate_no_flights(t *testing.T) {
	c := &candidate{searched: true}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	if c.allInCost != 0 {
		t.Errorf("expected allInCost=0 for no flights, got %f", c.allInCost)
	}
}

func TestPriceCandidate_with_transfer_cost(t *testing.T) {
	c := &candidate{
		searched:     true,
		transferCost: 30,
		flights: []models.FlightResult{
			{Price: 100, Currency: "EUR"},
		},
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	// AllInCost should include transfer cost.
	if c.allInCost < 130 {
		t.Errorf("expected allInCost >= 130 (100 flight + 30 transfer), got %f", c.allInCost)
	}
}

func TestPriceCandidate_selects_cheapest(t *testing.T) {
	c := &candidate{
		searched: true,
		flights: []models.FlightResult{
			{Price: 200, Currency: "EUR"},
			{Price: 150, Currency: "EUR"},
			{Price: 180, Currency: "EUR"},
		},
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	if c.baseCost != 150 {
		t.Errorf("expected baseCost=150 (cheapest), got %f", c.baseCost)
	}
}

func TestRankCandidates_sorts_by_allInCost(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "A"},
		{searched: true, allInCost: 100, baseCost: 100, currency: "EUR", strategy: "B"},
		{searched: true, allInCost: 150, baseCost: 150, currency: "EUR", strategy: "C"},
	}

	input := OptimizeInput{MaxResults: 3}
	result := rankCandidates(candidates, input)

	if !result.Success {
		t.Fatal("expected success")
	}
	if len(result.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(result.Options))
	}
	if result.Options[0].AllInCost != 100 {
		t.Errorf("rank 1 allInCost: want 100, got %f", result.Options[0].AllInCost)
	}
	if result.Options[1].AllInCost != 150 {
		t.Errorf("rank 2 allInCost: want 150, got %f", result.Options[1].AllInCost)
	}
	if result.Options[2].AllInCost != 200 {
		t.Errorf("rank 3 allInCost: want 200, got %f", result.Options[2].AllInCost)
	}
}

func TestRankCandidates_limits_results(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 300, baseCost: 300, currency: "EUR", strategy: "A"},
		{searched: true, allInCost: 100, baseCost: 100, currency: "EUR", strategy: "B"},
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "C"},
	}

	input := OptimizeInput{MaxResults: 2}
	result := rankCandidates(candidates, input)

	if len(result.Options) != 2 {
		t.Fatalf("expected 2 options (MaxResults=2), got %d", len(result.Options))
	}
}
