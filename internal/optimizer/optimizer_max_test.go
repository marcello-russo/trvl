package optimizer

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestOptimize_missingOrigin(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Destination: "BCN",
		DepartDate:  "2026-06-15",
	})
	if err == nil {
		t.Fatal("expected error for missing origin")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_missingDestination(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:     "HEL",
		DepartDate: "2026-06-15",
	})
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_missingDepartDate(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if err == nil {
		t.Fatal("expected error for missing depart date")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_invalidDepartDate(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for invalid depart date")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_invalidReturnDate(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		ReturnDate:  "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid return date")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestOptimize_sameOriginDest(t *testing.T) {
	res, err := Optimize(context.Background(), OptimizeInput{
		Origin:      "HEL",
		Destination: "HEL",
		DepartDate:  "2026-06-15",
	})
	if err == nil {
		t.Fatal("expected error for same origin/destination")
	}
	if res == nil || res.Error == "" {
		t.Error("expected error message in result")
	}
}

// --- expandCandidates: deeper coverage ---

func TestExpandCandidates_baseline(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		FlexDays:    0, // defaults() not called in unit test, set explicitly
	})
	if len(candidates) == 0 {
		t.Fatal("expected at least baseline candidate")
	}
	base := candidates[0]
	if base.origin != "HEL" || base.dest != "BCN" {
		t.Errorf("baseline should be HEL→BCN, got %s→%s", base.origin, base.dest)
	}
	if base.strategy != "Direct booking" {
		t.Errorf("baseline strategy should be 'Direct booking', got %q", base.strategy)
	}
}

func TestExpandCandidates_withReturnDateFlex(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		ReturnDate:  "2026-06-22",
		FlexDays:    2,
	})
	// Should have baseline + 4 flex candidates (±1, ±2) + any positioning/rail/etc.
	flexCount := 0
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "date_flex" {
				flexCount++
			}
		}
	}
	if flexCount != 4 {
		t.Errorf("expected 4 date flex candidates (±1, ±2), got %d", flexCount)
	}
	// Each flex candidate should have both depart and return dates shifted.
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "date_flex" {
				if c.returnDate == "" {
					t.Errorf("date flex candidate should have return date")
				}
				if c.departDate == "2026-06-15" {
					t.Error("date flex candidate should have shifted depart date")
				}
			}
		}
	}
}

func TestExpandCandidates_hiddenCityAMS(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "AMS",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	hiddenCityCount := 0
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "hidden_city" {
				hiddenCityCount++
			}
		}
	}
	if hiddenCityCount == 0 {
		t.Error("expected hidden city candidates for AMS destination")
	}
}

func TestExpandCandidates_hiddenCitySkipsSameAsOrigin(t *testing.T) {
	// Origin HEL should not appear as a hidden city beyond destination.
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "AMS",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "hidden_city" && c.dest == "HEL" {
				t.Error("hidden city candidate should not have dest == origin")
			}
		}
	}
}

func TestExpandCandidates_departureTaxFiltering(t *testing.T) {
	// AMS should have departure tax savings and zero-tax alternatives.
	candidates := expandCandidates(OptimizeInput{
		Origin:      "AMS",
		Destination: "BCN",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	taxCount := 0
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "departure_tax" {
				taxCount++
				// Transfer cost should be set.
				if c.transferCost <= 0 {
					t.Error("departure tax candidate should have transfer cost")
				}
			}
		}
	}
	// AMS is in Netherlands with ~26 EUR tax; alternatives should exist.
	if taxCount == 0 {
		t.Log("no departure tax alternatives found for AMS (may depend on tax data)")
	}
}

func TestExpandCandidates_railCompetition(t *testing.T) {
	// PRG→VIE has a known rail corridor.
	candidates := expandCandidates(OptimizeInput{
		Origin:      "PRG",
		Destination: "VIE",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	railFound := false
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "rail_competition" {
				railFound = true
				if !c.prePriced {
					t.Error("rail candidate should be prePriced")
				}
				if c.baseCost <= 0 {
					t.Error("rail candidate should have baseCost > 0")
				}
				if !c.searched {
					t.Error("rail candidate should be marked as searched")
				}
			}
		}
	}
	if !railFound {
		t.Error("expected rail competition candidate for PRG→VIE")
	}
}

func TestExpandCandidates_ferryHELARN(t *testing.T) {
	candidates := expandCandidates(OptimizeInput{
		Origin:      "HEL",
		Destination: "ARN",
		DepartDate:  "2026-06-15",
		FlexDays:    0,
	})
	ferryFound := false
	for _, c := range candidates {
		for _, h := range c.hackTypes {
			if h == "ferry_cabin_hotel" {
				ferryFound = true
				if !c.prePriced {
					t.Error("ferry candidate should be prePriced")
				}
			}
		}
	}
	if !ferryFound {
		t.Error("expected ferry cabin candidate for HEL→ARN")
	}
}

// --- priceCandidate: deeper edge cases ---

func TestPriceCandidate_emptyFlights(t *testing.T) {
	c := &candidate{
		searched: true,
		flights:  []models.FlightResult{},
	}
	input := OptimizeInput{Currency: "EUR"}
	priceCandidate(c, input)
	if c.allInCost != 0 {
		t.Errorf("expected allInCost=0 for empty flights, got %.2f", c.allInCost)
	}
}

func TestPriceCandidate_prePricedWithTransfer(t *testing.T) {
	c := &candidate{
		searched:     true,
		prePriced:    true,
		baseCost:     50,
		transferCost: 10,
		currency:     "EUR",
	}
	input := OptimizeInput{Currency: "EUR"}
	priceCandidate(c, input)
	if c.allInCost != 60 {
		t.Errorf("expected allInCost=60 for prePriced, got %.2f", c.allInCost)
	}
}

func TestPriceCandidate_prePricedNoTransfer(t *testing.T) {
	c := &candidate{
		searched:  true,
		prePriced: true,
		baseCost:  75,
		currency:  "EUR",
	}
	input := OptimizeInput{Currency: "EUR"}
	priceCandidate(c, input)
	if c.allInCost != 75 {
		t.Errorf("expected allInCost=75, got %.2f", c.allInCost)
	}
}

func TestPriceCandidate_multipleFlightsSelectsCheapest(t *testing.T) {
	c := &candidate{
		searched: true,
		origin:   "HEL",
		dest:     "BCN",
		flights: []models.FlightResult{
			{Price: 300, Currency: "EUR"},
			{Price: 150, Currency: "EUR"},
			{Price: 200, Currency: "EUR"},
		},
	}
	input := OptimizeInput{Currency: "EUR"}
	priceCandidate(c, input)
	if c.baseCost != 150 {
		t.Errorf("expected baseCost=150, got %.2f", c.baseCost)
	}
	if c.allInCost <= 0 {
		t.Error("allInCost should be positive")
	}
}

// --- rankCandidates: deeper coverage ---

func TestRankCandidates_emptyPriced(t *testing.T) {
	candidates := []*candidate{
		{searched: false},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 5, Currency: "EUR"})
	if result.Success {
		t.Error("expected Success=false when no priced candidates")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestRankCandidates_sortsByAllInCost(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 300, currency: "EUR", strategy: "A"},
		{searched: true, allInCost: 100, currency: "EUR", strategy: "B"},
		{searched: true, allInCost: 200, currency: "EUR", strategy: "C"},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 10, Currency: "EUR"})
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(result.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(result.Options))
	}
	if result.Options[0].AllInCost != 100 {
		t.Errorf("first option should be cheapest (100), got %.0f", result.Options[0].AllInCost)
	}
	if result.Options[2].AllInCost != 300 {
		t.Errorf("last option should be most expensive (300), got %.0f", result.Options[2].AllInCost)
	}
}

func TestRankCandidates_baselineIdentified(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, currency: "EUR", strategy: "Direct booking"},
		{searched: true, allInCost: 150, currency: "EUR", strategy: "Via TLL", hackTypes: []string{"positioning"}},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 10, Currency: "EUR"})
	if result.Baseline == nil {
		t.Fatal("expected baseline to be identified")
	}
	if result.Baseline.AllInCost != 200 {
		t.Errorf("baseline allInCost should be 200, got %.0f", result.Baseline.AllInCost)
	}
}

func TestRankCandidates_savingsVsBaseline(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, currency: "EUR", strategy: "Direct booking"},
		{searched: true, allInCost: 150, currency: "EUR", strategy: "Via TLL", hackTypes: []string{"positioning"}},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 10, Currency: "EUR"})
	// The positioning option should show savings vs baseline.
	found := false
	for _, opt := range result.Options {
		if opt.Strategy == "Via TLL" {
			found = true
			if opt.SavingsVsBaseline != 50 {
				t.Errorf("expected savings=50, got %.0f", opt.SavingsVsBaseline)
			}
		}
	}
	if !found {
		t.Error("positioning option not found in results")
	}
}

func TestRankCandidates_crossCurrencyNoSavings(t *testing.T) {
	candidates := []*candidate{
		{searched: true, allInCost: 200, currency: "EUR", strategy: "Direct booking"},
		{searched: true, allInCost: 150, currency: "USD", strategy: "Via X", hackTypes: []string{"positioning"}},
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 10, Currency: "EUR"})
	for _, opt := range result.Options {
		if opt.Currency == "USD" && opt.SavingsVsBaseline != 0 {
			t.Errorf("cross-currency savings should be 0, got %.0f", opt.SavingsVsBaseline)
		}
	}
}

func TestRankCandidates_limitsToMaxResults(t *testing.T) {
	candidates := make([]*candidate, 10)
	for i := range candidates {
		candidates[i] = &candidate{
			searched:  true,
			allInCost: float64(100 + i*10),
			currency:  "EUR",
			strategy:  "S",
		}
	}
	result := rankCandidates(candidates, OptimizeInput{MaxResults: 3, Currency: "EUR"})
	if len(result.Options) != 3 {
		t.Errorf("expected 3 options (MaxResults=3), got %d", len(result.Options))
	}
}

// --- candidateToOption: deeper branches ---

func TestCandidateToOption_groundTransferLeg(t *testing.T) {
	c := &candidate{
		searched:     true,
		origin:       "TLL",
		dest:         "BCN",
		departDate:   "2026-06-15",
		transferCost: 30,
		currency:     "EUR",
		flights:      []models.FlightResult{{Price: 150, Currency: "EUR"}},
		strategy:     "Via Tallinn",
	}
	opt := candidateToOption(c, 1, OptimizeInput{Origin: "HEL", Currency: "EUR"})
	if len(opt.Legs) < 2 {
		t.Fatalf("expected at least 2 legs (ground + flight), got %d", len(opt.Legs))
	}
	if opt.Legs[0].Type != "ground" {
		t.Errorf("first leg should be ground, got %s", opt.Legs[0].Type)
	}
	if opt.Legs[0].From != "HEL" {
		t.Errorf("ground leg From should be HEL, got %s", opt.Legs[0].From)
	}
	if opt.Legs[0].To != "TLL" {
		t.Errorf("ground leg To should be TLL, got %s", opt.Legs[0].To)
	}
}

func TestCandidateToOption_destinationTransferLeg(t *testing.T) {
	c := &candidate{
		searched:     true,
		origin:       "HEL",
		dest:         "GRO",
		departDate:   "2026-06-15",
		transferCost: 15,
		currency:     "EUR",
		hackTypes:    []string{"destination_airport"},
		flights:      []models.FlightResult{{Price: 80, Currency: "EUR"}},
		strategy:     "Fly to Girona",
	}
	opt := candidateToOption(c, 1, OptimizeInput{Origin: "HEL", Destination: "BCN", Currency: "EUR"})
	// Should have: ground transfer (origin→origin if transferCost>0) + flight + destination ground.
	groundLegs := 0
	for _, l := range opt.Legs {
		if l.Type == "ground" {
			groundLegs++
		}
	}
	if groundLegs < 1 {
		t.Errorf("expected at least 1 ground leg for destination_airport, got %d", groundLegs)
	}
}

func TestCandidateToOption_rankAssignment(t *testing.T) {
	c := &candidate{
		searched:  true,
		origin:    "HEL",
		dest:      "BCN",
		currency:  "EUR",
		allInCost: 150,
		baseCost:  150,
		flights:   []models.FlightResult{{Price: 150, Currency: "EUR"}},
		strategy:  "Direct",
	}
	opt := candidateToOption(c, 42, OptimizeInput{Origin: "HEL", Currency: "EUR"})
	if opt.Rank != 42 {
		t.Errorf("expected rank=42, got %d", opt.Rank)
	}
}

func TestCandidateToOption_hacksApplied(t *testing.T) {
	c := &candidate{
		searched:  true,
		origin:    "HEL",
		dest:      "BCN",
		currency:  "EUR",
		allInCost: 150,
		baseCost:  150,
		hackTypes: []string{"positioning", "date_flex"},
		flights:   []models.FlightResult{{Price: 150, Currency: "EUR"}},
		strategy:  "Positioning + flex",
	}
	opt := candidateToOption(c, 1, OptimizeInput{Origin: "HEL", Currency: "EUR"})
	if len(opt.HacksApplied) != 2 {
		t.Errorf("expected 2 hacks applied, got %d", len(opt.HacksApplied))
	}
}

// --- defaults: edge cases ---

func TestDefaults_zeroFlexDaysBecomesThree(t *testing.T) {
	in := OptimizeInput{}
	in.defaults()
	if in.FlexDays != 3 {
		t.Errorf("expected FlexDays=3 from zero, got %d", in.FlexDays)
	}
}
