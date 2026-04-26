package watch

import (
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/dealquality"
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

func TestResolveWindowDatesNextNd(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	from, to, err := ResolveWindowDates("next_30d", "next_90d", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedFrom := now.AddDate(0, 0, 30)
	expectedTo := now.AddDate(0, 0, 90)
	if !from.Equal(expectedFrom) {
		t.Errorf("from: expected %s, got %s", expectedFrom, from)
	}
	if !to.Equal(expectedTo) {
		t.Errorf("to: expected %s, got %s", expectedTo, to)
	}
}

func TestResolveWindowDatesLiteral(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	from, to, err := ResolveWindowDates("2026-07-01", "2026-09-01", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from.Format("2006-01-02") != "2026-07-01" {
		t.Errorf("unexpected from: %s", from)
	}
	if to.Format("2006-01-02") != "2026-09-01" {
		t.Errorf("unexpected to: %s", to)
	}
}

func TestResolveWindowDatesInvalidOrder(t *testing.T) {
	now := time.Now()
	_, _, err := ResolveWindowDates("next_90d", "next_30d", now)
	if err == nil {
		t.Error("expected error for from > to")
	}
}

func TestResolveFavouritesExplicit(t *testing.T) {
	w := Watch{Favourites: []string{"BCN", "PRG"}}
	result := ResolveFavourites(w, nil)
	if len(result) != 2 || result[0] != "BCN" {
		t.Errorf("expected explicit favourites, got %v", result)
	}
}

func TestResolveFavouritesFromProfile(t *testing.T) {
	w := Watch{}
	prefs := &preferences.Preferences{
		BucketList:      []string{"BCN", "PRG", "ZRH"},
		PreviousTrips:   []string{"PRG", "TXL"},
		AirportAffinity: map[string]float64{"BCN": 0.8, "PRG": 0.5, "ZRH": 0.2, "TXL": 0.4},
	}
	result := ResolveFavourites(w, prefs)
	// BCN (0.8>=0.3) ✓, PRG (0.5>=0.3) ✓, ZRH (0.2<0.3) ✗, TXL (0.4>=0.3) ✓
	if len(result) != 3 {
		t.Errorf("expected 3 destinations (BCN,PRG,TXL), got %d: %v", len(result), result)
	}
}

func TestResolveFavouritesEmptyAffinityFallback(t *testing.T) {
	w := Watch{}
	prefs := &preferences.Preferences{
		BucketList:    []string{"BCN", "PRG"},
		PreviousTrips: []string{"TXL"},
		// No AirportAffinity
	}
	result := ResolveFavourites(w, prefs)
	// Falls back to BucketList only
	if len(result) != 2 {
		t.Errorf("expected BucketList fallback (2 items), got %d: %v", len(result), result)
	}
}

func TestScoreOpportunityFormula(t *testing.T) {
	prefs := &preferences.Preferences{
		BudgetFlightMax: 500,
		AirportAffinity: map[string]float64{"BCN": 0.9},
		BucketList:      []string{"BCN"},
	}

	// 15 samples around 200 EUR to give meaningful DQ score
	samples := make([]dealquality.Sample, 15)
	for i := 0; i < 15; i++ {
		samples[i] = dealquality.Sample{
			Route: "HEL-BCN", Season: "Q3",
			Date:  "2026-07-15",
			Price: float64(150 + i*10),
			Kind:  "flight",
		}
	}

	os := ScoreOpportunity("BCN", "2026-07-01", "2026-07-08", prefs, samples, 160)
	if os.Destination != "BCN" {
		t.Errorf("unexpected destination: %s", os.Destination)
	}
	if os.Nights != 7 {
		t.Errorf("expected 7 nights, got %d", os.Nights)
	}
	if os.OverallScore < 0 || os.OverallScore > 100 {
		t.Errorf("overall score out of range: %d", os.OverallScore)
	}
	// Components should be populated
	if os.ProfileMatch < 0 || os.ProfileMatch > 100 {
		t.Errorf("profile match out of range: %d", os.ProfileMatch)
	}
	if os.RequestMatch < 0 || os.RequestMatch > 100 {
		t.Errorf("request match out of range: %d", os.RequestMatch)
	}
	if os.DealQuality < 0 || os.DealQuality > 100 {
		t.Errorf("deal quality out of range: %d", os.DealQuality)
	}
}

func TestScoreOpportunityInsufficientHistory(t *testing.T) {
	prefs := &preferences.Preferences{BudgetFlightMax: 500}
	os := ScoreOpportunity("PRG", "2026-08-01", "2026-08-07", prefs, nil, 200)
	// With no history, DealQuality returns 50 → DQ component = 0.4*50 = 20
	// OverallScore should be reasonable (not zero, not panic)
	if os.DealQuality != 50 {
		t.Errorf("expected DealQuality=50 for insufficient history, got %d", os.DealQuality)
	}
}

func TestScoreOpportunityCompositeWeights(t *testing.T) {
	// Verify the formula: 0.4*PM + 0.2*RM + 0.4*DQ
	prefs := &preferences.Preferences{BudgetFlightMax: 1000}
	samples := make([]dealquality.Sample, 10)
	for i := 0; i < 10; i++ {
		samples[i] = dealquality.Sample{
			Route: "HEL-ZRH", Season: "Q3",
			Date: "2026-07-01", Price: float64(200 + i*20), Kind: "flight",
		}
	}
	os := ScoreOpportunity("ZRH", "2026-07-10", "2026-07-17", prefs, samples, 200)

	expected := int(0.4*float64(os.ProfileMatch) + 0.2*float64(os.RequestMatch) + 0.4*float64(os.DealQuality))
	if expected > 100 {
		expected = 100
	}
	if os.OverallScore != expected {
		t.Errorf("formula mismatch: 0.4*%d + 0.2*%d + 0.4*%d = %d, got OverallScore=%d",
			os.ProfileMatch, os.RequestMatch, os.DealQuality, expected, os.OverallScore)
	}
}
