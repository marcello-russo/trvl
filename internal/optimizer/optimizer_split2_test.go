package optimizer

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestRankCandidates_baseline_savings(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "Direct"},
		{searched: true, allInCost: 150, baseCost: 150, currency: "EUR", strategy: "Alt", hackTypes: []string{"positioning"}},
	}

	input := OptimizeInput{MaxResults: 5}
	result := rankCandidates(candidates, input)

	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Baseline == nil {
		t.Fatal("expected baseline to be set")
	}
	if result.Baseline.AllInCost != 200 {
		t.Errorf("baseline allInCost: want 200, got %f", result.Baseline.AllInCost)
	}

	// The cheapest option (150) should show savings of 50 vs baseline (200).
	if result.Options[0].SavingsVsBaseline != 50 {
		t.Errorf("savings vs baseline: want 50, got %f", result.Options[0].SavingsVsBaseline)
	}
}

func TestRankCandidates_no_results(t *testing.T) {
	candidates := []*candidate{
		{searched: false}, // not searched
	}

	input := OptimizeInput{MaxResults: 5}
	result := rankCandidates(candidates, input)

	if result.Success {
		t.Error("expected failure when no results")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestRankCandidates_assigns_ranks(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "A"},
		{searched: true, allInCost: 100, baseCost: 100, currency: "EUR", strategy: "B"},
	}

	input := OptimizeInput{MaxResults: 5}
	result := rankCandidates(candidates, input)

	for i, opt := range result.Options {
		if opt.Rank != i+1 {
			t.Errorf("option %d rank: want %d, got %d", i, i+1, opt.Rank)
		}
	}
}

func TestCandidateToOption_legs(t *testing.T) {
	c := &candidate{
		origin:       "TLL",
		dest:         "BCN",
		departDate:   "2026-06-01",
		strategy:     "Fly from Tallinn",
		hackTypes:    []string{"positioning"},
		transferCost: 30,
		baseCost:     150,
		currency:     "EUR",
		allInCost:    180,
		flights: []models.FlightResult{
			{Price: 150, Currency: "EUR", Duration: 240},
		},
	}

	input := OptimizeInput{Origin: "HEL", Destination: "BCN"}
	opt := candidateToOption(c, 1, input)

	if len(opt.Legs) < 2 {
		t.Fatalf("expected at least 2 legs (ground + flight), got %d", len(opt.Legs))
	}

	if opt.Legs[0].Type != "ground" {
		t.Errorf("first leg type: want ground, got %s", opt.Legs[0].Type)
	}
	if opt.Legs[0].From != "HEL" {
		t.Errorf("first leg from: want HEL, got %s", opt.Legs[0].From)
	}
	if opt.Legs[0].To != "TLL" {
		t.Errorf("first leg to: want TLL, got %s", opt.Legs[0].To)
	}

	if opt.Legs[1].Type != "flight" {
		t.Errorf("second leg type: want flight, got %s", opt.Legs[1].Type)
	}
}

func TestConvertFFStatuses(t *testing.T) {
	statuses := []FFStatus{
		{Alliance: "skyteam", Tier: "gold"},
		{Alliance: "oneworld", Tier: "sapphire"},
	}
	converted := convertFFStatuses(statuses)

	if len(converted) != 2 {
		t.Fatalf("expected 2 converted statuses, got %d", len(converted))
	}
	if converted[0].Alliance != "skyteam" || converted[0].Tier != "gold" {
		t.Errorf("first status: got %+v", converted[0])
	}
	if converted[1].Alliance != "oneworld" || converted[1].Tier != "sapphire" {
		t.Errorf("second status: got %+v", converted[1])
	}
}

func TestCheapestFlight(t *testing.T) {
	flights := []models.FlightResult{
		{Price: 200, Currency: "EUR"},
		{Price: 0, Currency: "EUR"}, // invalid
		{Price: 150, Currency: "EUR"},
		{Price: 180, Currency: "EUR"},
	}
	best := cheapestFlight(flights)
	if best.Price != 150 {
		t.Errorf("cheapestFlight: want 150, got %f", best.Price)
	}
}

func TestOptimize_validation_error(t *testing.T) {
	result, err := Optimize(t.Context(), OptimizeInput{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}
	if result.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestExpandCandidates_departure_tax_AMS(t *testing.T) {
	// AMS is in NL (€26 tax). Some nearby airports may be in zero-tax countries.
	input := OptimizeInput{
		Origin:      "AMS",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
	}
	input.defaults()
	candidates := expandCandidates(input)

	// Check for departure_tax + positioning candidates.
	var taxCandidates []*candidate
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "departure_tax" {
				taxCandidates = append(taxCandidates, c)
				break
			}
		}
	}

	// Whether we get candidates depends on whether any zero-tax alternative
	// has ground cost < €26. We verify the ones that appear are correct.
	for _, c := range taxCandidates {
		if c.origin == c.dest {
			t.Errorf("departure_tax candidate has same origin and dest: %s", c.origin)
		}
		// Must have both departure_tax and positioning hack types.
		hasTax, hasPos := false, false
		for _, h := range c.hackTypes {
			if h == "departure_tax" {
				hasTax = true
			}
			if h == "positioning" {
				hasPos = true
			}
		}
		if !hasTax || !hasPos {
			t.Errorf("departure_tax candidate %s missing hack types: tax=%v pos=%v", c.origin, hasTax, hasPos)
		}
	}
}

func TestExpandCandidates_rail_competition(t *testing.T) {
	// MAD→BCN is a competitive rail corridor.
	input := OptimizeInput{
		Origin:      "MAD",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		FlexDays:    -1, // disable date flex to simplify
	}
	input.defaults()
	candidates := expandCandidates(input)

	var railCandidate *candidate
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "rail_competition" {
				railCandidate = c
				break
			}
		}
		if railCandidate != nil {
			break
		}
	}

	if railCandidate == nil {
		t.Fatal("expected rail_competition candidate for MAD→BCN")
	}
	if !railCandidate.prePriced {
		t.Error("rail_competition candidate should be prePriced")
	}
	if !railCandidate.searched {
		t.Error("rail_competition candidate should be marked searched")
	}
	if railCandidate.baseCost != 7 {
		t.Errorf("rail baseCost: got %.0f, want 7", railCandidate.baseCost)
	}

	// Verify both hack types.
	hasRail, hasGround := false, false
	for _, h := range railCandidate.hackTypes {
		if h == "rail_competition" {
			hasRail = true
		}
		if h == "ground_alternative" {
			hasGround = true
		}
	}
	if !hasRail || !hasGround {
		t.Errorf("expected hackTypes [rail_competition, ground_alternative], got %v", railCandidate.hackTypes)
	}
}

func TestExpandCandidates_ferry_cabin(t *testing.T) {
	// HEL→ARN has an overnight ferry.
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "ARN",
		DepartDate:  "2026-06-01",
		FlexDays:    -1, // disable date flex
	}
	input.defaults()
	candidates := expandCandidates(input)

	var ferryCandidate *candidate
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "ferry_cabin_hotel" {
				ferryCandidate = c
				break
			}
		}
		if ferryCandidate != nil {
			break
		}
	}

	if ferryCandidate == nil {
		t.Fatal("expected ferry_cabin_hotel candidate for HEL→ARN")
	}
	if !ferryCandidate.prePriced {
		t.Error("ferry_cabin_hotel candidate should be prePriced")
	}
	if !ferryCandidate.searched {
		t.Error("ferry_cabin_hotel candidate should be marked searched")
	}
	if ferryCandidate.baseCost != 35 {
		t.Errorf("ferry baseCost: got %.0f, want 35", ferryCandidate.baseCost)
	}
}

func TestExpandCandidates_no_rail_for_non_corridor(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		FlexDays:    -1,
	}
	input.defaults()
	candidates := expandCandidates(input)

	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "rail_competition" {
				t.Error("unexpected rail_competition candidate for HEL→BCN")
			}
		}
	}
}

func TestExpandCandidates_no_ferry_for_non_route(t *testing.T) {
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-01",
		FlexDays:    -1,
	}
	input.defaults()
	candidates := expandCandidates(input)

	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "ferry_cabin_hotel" {
				t.Error("unexpected ferry_cabin_hotel candidate for HEL→BCN")
			}
		}
	}
}

func TestPriceCandidate_prePriced(t *testing.T) {
	c := &candidate{
		searched:     true,
		prePriced:    true,
		baseCost:     7,
		transferCost: 0,
		currency:     "EUR",
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	if c.allInCost != 7 {
		t.Errorf("prePriced allInCost: got %.0f, want 7", c.allInCost)
	}
	if c.bagCost != 0 {
		t.Errorf("prePriced bagCost: got %.0f, want 0", c.bagCost)
	}
}

func TestPriceCandidate_prePriced_with_transfer(t *testing.T) {
	c := &candidate{
		searched:     true,
		prePriced:    true,
		baseCost:     35,
		transferCost: 10,
		currency:     "EUR",
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	if c.allInCost != 45 {
		t.Errorf("prePriced allInCost: got %.0f, want 45 (35+10)", c.allInCost)
	}
}

func TestCandidateToOption_prePriced_ground_leg(t *testing.T) {
	c := &candidate{
		origin:     "MAD",
		dest:       "BCN",
		departDate: "2026-06-01",
		strategy:   "Take train (AVE, AVLO, Ouigo, Iryo) — fares from €7",
		hackTypes:  []string{"rail_competition", "ground_alternative"},
		prePriced:  true,
		searched:   true,
		baseCost:   7,
		currency:   "EUR",
		allInCost:  7,
	}

	input := OptimizeInput{Origin: "MAD", Destination: "BCN"}
	opt := candidateToOption(c, 1, input)

	if len(opt.Legs) == 0 {
		t.Fatal("expected at least 1 leg for pre-priced candidate")
	}
	if opt.Legs[0].Type != "ground" {
		t.Errorf("pre-priced leg type: want ground, got %s", opt.Legs[0].Type)
	}
	if opt.Legs[0].Price != 7 {
		t.Errorf("pre-priced leg price: want 7, got %.0f", opt.Legs[0].Price)
	}
}

func TestRankCandidates_cross_currency_no_savings(t *testing.T) {
	// When baseline is RUB and option is EUR, savings should be 0 (not cross-currency nonsense).
	candidates := []*candidate{
		{searched: true, allInCost: 7000, baseCost: 7000, currency: "RUB", strategy: "Direct"},
		{searched: true, prePriced: true, allInCost: 7, baseCost: 7, currency: "EUR", strategy: "Train", hackTypes: []string{"rail_competition"}},
	}

	input := OptimizeInput{MaxResults: 5, Currency: "RUB"}
	result := rankCandidates(candidates, input)

	if !result.Success {
		t.Fatal("expected success")
	}

	// The RUB baseline should rank first (same currency as input).
	if result.Options[0].Currency != "RUB" {
		t.Errorf("rank 1 should be same-currency (RUB), got %s", result.Options[0].Currency)
	}

	// The EUR option should have zero savings (can't compare cross-currency).
	for _, opt := range result.Options {
		if opt.Currency == "EUR" && opt.SavingsVsBaseline != 0 {
			t.Errorf("cross-currency option should have 0 savings, got %.0f", opt.SavingsVsBaseline)
		}
	}
}

func TestPriceCandidate_error_fare_flagged(t *testing.T) {
	// HEL→BCN one-way, €20. Long-haul floor is €60, error threshold is €30.
	// priceCandidate should append "error_fare" to hackTypes.
	c := &candidate{
		origin:   "HEL",
		dest:     "BCN",
		searched: true,
		flights: []models.FlightResult{
			{Price: 20, Currency: "EUR"},
		},
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	found := false
	for _, h := range c.hackTypes {
		if h == "error_fare" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error_fare in hackTypes, got %v", c.hackTypes)
	}
}

func TestPriceCandidate_flash_sale_flagged(t *testing.T) {
	// HEL→BCN one-way, €45. Below floor (€60) but above error threshold (€30).
	c := &candidate{
		origin:   "HEL",
		dest:     "BCN",
		searched: true,
		flights: []models.FlightResult{
			{Price: 45, Currency: "EUR"},
		},
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	found := false
	for _, h := range c.hackTypes {
		if h == "flash_sale" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected flash_sale in hackTypes, got %v", c.hackTypes)
	}
}

func TestPriceCandidate_normal_price_no_error_fare(t *testing.T) {
	// HEL→BCN one-way, €200. Normal price, no hack should be appended.
	c := &candidate{
		origin:   "HEL",
		dest:     "BCN",
		searched: true,
		flights: []models.FlightResult{
			{Price: 200, Currency: "EUR"},
		},
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	for _, h := range c.hackTypes {
		if h == "error_fare" || h == "flash_sale" {
			t.Errorf("unexpected hack %q in hackTypes for normal price", h)
		}
	}
}

func TestPriceCandidate_error_fare_preserves_existing_hacks(t *testing.T) {
	// Candidate already has "positioning" hack; error_fare should be appended, not replace.
	c := &candidate{
		origin:    "HEL",
		dest:      "BCN",
		searched:  true,
		hackTypes: []string{"positioning"},
		flights: []models.FlightResult{
			{Price: 20, Currency: "EUR"},
		},
	}
	input := OptimizeInput{}
	input.defaults()
	priceCandidate(c, input)

	if len(c.hackTypes) < 2 {
		t.Fatalf("expected at least 2 hackTypes, got %v", c.hackTypes)
	}
	if c.hackTypes[0] != "positioning" {
		t.Errorf("first hackType should be positioning, got %q", c.hackTypes[0])
	}
	foundErrorFare := false
	for _, h := range c.hackTypes {
		if h == "error_fare" {
			foundErrorFare = true
		}
	}
	if !foundErrorFare {
		t.Errorf("expected error_fare appended to hackTypes, got %v", c.hackTypes)
	}
}

func TestRankCandidates_same_currency_savings(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, baseCost: 200, currency: "EUR", strategy: "Direct"},
		{searched: true, allInCost: 150, baseCost: 150, currency: "EUR", strategy: "Alt", hackTypes: []string{"positioning"}},
	}

	input := OptimizeInput{MaxResults: 5, Currency: "EUR"}
	result := rankCandidates(candidates, input)

	// Same currency — savings should be computed.
	if result.Options[0].SavingsVsBaseline != 50 {
		t.Errorf("same-currency savings: want 50, got %.0f", result.Options[0].SavingsVsBaseline)
	}
}
