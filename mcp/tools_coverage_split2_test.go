package mcp

import (
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
	"github.com/MikkoParkkola/trvl/internal/visa"
)

func TestBuildVisaSummary_Failure(t *testing.T) {
	t.Parallel()
	result := visa.Result{
		Success: false,
		Error:   "unknown country",
	}
	got := buildVisaSummary(result)
	if !strings.Contains(got, "failed") {
		t.Errorf("expected failed in summary, got %q", got)
	}
	if !strings.Contains(got, "unknown country") {
		t.Errorf("expected error message in summary")
	}
}

func TestBuildVisaSummary_NoMaxStay(t *testing.T) {
	t.Parallel()
	result := visa.Result{
		Success: true,
		Requirement: visa.Requirement{
			Passport:    "FI",
			Destination: "JP",
			Status:      "visa-required",
		},
	}
	got := buildVisaSummary(result)
	if strings.Contains(got, "Max stay") {
		t.Errorf("should not contain max stay when empty")
	}
}

// ============================================================
// destinationSummary
// ============================================================

func TestDestinationSummary(t *testing.T) {
	t.Parallel()
	info := &models.DestinationInfo{
		Location: "Tokyo",
		Country: models.CountryInfo{
			Name:   "Japan",
			Region: "Asia",
		},
		Weather: models.WeatherInfo{
			Current: models.WeatherDay{
				Date:        "2026-07-01",
				TempHigh:    32,
				TempLow:     24,
				Description: "Humid",
			},
		},
		Safety: models.SafetyInfo{
			Level:    4.5,
			Advisory: "Exercise normal caution",
			Source:   "test",
		},
		Currency: models.CurrencyInfo{
			LocalCurrency: "JPY",
			ExchangeRate:  160.5,
		},
		Holidays: []models.Holiday{{Date: "2026-07-03", Name: "Test Holiday"}},
	}

	got := destinationSummary(info)
	if !strings.Contains(got, "Tokyo") {
		t.Error("should contain location")
	}
	if !strings.Contains(got, "Japan") {
		t.Error("should contain country")
	}
	if !strings.Contains(got, "Asia") {
		t.Error("should contain region")
	}
	if !strings.Contains(got, "Humid") {
		t.Error("should contain weather")
	}
	if !strings.Contains(got, "4.5/5") {
		t.Error("should contain safety level")
	}
	if !strings.Contains(got, "160.50") {
		t.Error("should contain exchange rate")
	}
	if !strings.Contains(got, "1 public") {
		t.Error("should contain holiday count")
	}
}

func TestDestinationSummary_Minimal(t *testing.T) {
	t.Parallel()
	info := &models.DestinationInfo{Location: "Unknown"}
	got := destinationSummary(info)
	if !strings.Contains(got, "Unknown") {
		t.Error("should contain location")
	}
	if strings.Contains(got, "Country") {
		t.Error("should not contain country when empty")
	}
}

// ============================================================
// tripCostSummaryAmount
// ============================================================

func TestTripCostSummaryAmount(t *testing.T) {
	t.Parallel()
	if got := tripCostSummaryAmount(0, "EUR"); got != "unavailable" {
		t.Errorf("zero = %q, want unavailable", got)
	}
	if got := tripCostSummaryAmount(-1, "EUR"); got != "unavailable" {
		t.Errorf("negative = %q, want unavailable", got)
	}
	if got := tripCostSummaryAmount(150, "EUR"); got != "EUR 150" {
		t.Errorf("positive = %q, want EUR 150", got)
	}
}

// ============================================================
// sendProgress (nil-safe)
// ============================================================

func TestSendProgress_NilFunc(t *testing.T) {
	t.Parallel()
	// Should not panic.
	sendProgress(nil, 50, 100, "test")
}

func TestSendProgress_WithFunc(t *testing.T) {
	t.Parallel()
	called := false
	fn := func(progress, total float64, message string) {
		called = true
		if progress != 50 || total != 100 || message != "test" {
			t.Errorf("unexpected args: %f, %f, %q", progress, total, message)
		}
	}
	sendProgress(fn, 50, 100, "test")
	if !called {
		t.Error("progress func not called")
	}
}

// ============================================================
// normalizeTierName
// ============================================================

func TestNormalizeTierName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{"Gold", "gold"},
		{"Elite Plus", "elite_plus"},
		{"SAPPHIRE", "sapphire"},
		{"  Silver  ", "silver"},
		{"elite-plus", "elite_plus"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeTierName(tt.in); got != tt.want {
				t.Errorf("normalizeTierName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ============================================================
// tierDisplayName
// ============================================================

func TestTierDisplayName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		alliance string
		tier     string
		want     string
	}{
		{"oneworld", "sapphire", "Sapphire"},
		{"oneworld", "emerald", "Emerald"},
		{"star_alliance", "gold", "Alliance Gold"},
		{"skyteam", "elite_plus", "Elite Plus"},
		{"unknown", "gold", "Gold"},             // fallback
		{"unknown", "elite_plus", "Elite Plus"}, // fallback
	}
	for _, tt := range tests {
		t.Run(tt.alliance+"_"+tt.tier, func(t *testing.T) {
			if got := tierDisplayName(tt.alliance, tt.tier); got != tt.want {
				t.Errorf("tierDisplayName(%q, %q) = %q, want %q", tt.alliance, tt.tier, got, tt.want)
			}
		})
	}
}

// ============================================================
// ffStatusToCards
// ============================================================

func TestFfStatusToCards_Empty(t *testing.T) {
	t.Parallel()
	got := ffStatusToCards(nil)
	if len(got) != 0 {
		t.Errorf("expected nil for empty programs, got %v", got)
	}
}

func TestFfStatusToCards_OneworldSapphire(t *testing.T) {
	t.Parallel()
	programs := []preferences.FrequentFlyerStatus{
		{AirlineCode: "AY", Alliance: "oneworld", Tier: "Sapphire"},
	}
	got := ffStatusToCards(programs)
	if len(got) == 0 {
		t.Fatal("expected cards")
	}
	// Should include "Oneworld Sapphire" and "Finnair Plus Sapphire".
	hasAlliance := false
	hasAirline := false
	for _, c := range got {
		if c == "Oneworld Sapphire" {
			hasAlliance = true
		}
		if c == "Finnair Plus Sapphire" {
			hasAirline = true
		}
	}
	if !hasAlliance {
		t.Errorf("missing Oneworld Sapphire in %v", got)
	}
	if !hasAirline {
		t.Errorf("missing Finnair Plus Sapphire in %v", got)
	}
}

func TestFfStatusToCards_StarAllianceGold(t *testing.T) {
	t.Parallel()
	programs := []preferences.FrequentFlyerStatus{
		{AirlineCode: "LH", Alliance: "star alliance", Tier: "Gold"},
	}
	got := ffStatusToCards(programs)
	hasAlliance := false
	hasAirline := false
	for _, c := range got {
		if c == "Star Alliance Gold" {
			hasAlliance = true
		}
		if c == "Miles & More Alliance Gold" {
			hasAirline = true
		}
	}
	if !hasAlliance {
		t.Errorf("missing Star Alliance Gold in %v", got)
	}
	if !hasAirline {
		t.Errorf("missing Miles & More Alliance Gold in %v", got)
	}
}

func TestFfStatusToCards_NoDuplicates(t *testing.T) {
	t.Parallel()
	// Two programs with same alliance + tier should not produce duplicates.
	programs := []preferences.FrequentFlyerStatus{
		{AirlineCode: "AY", Alliance: "oneworld", Tier: "Sapphire"},
		{AirlineCode: "BA", Alliance: "oneworld", Tier: "Sapphire"},
	}
	got := ffStatusToCards(programs)
	seen := make(map[string]int)
	for _, c := range got {
		seen[strings.ToLower(c)]++
	}
	for k, v := range seen {
		if v > 1 {
			t.Errorf("duplicate card %q appears %d times", k, v)
		}
	}
}

// ============================================================
// notifyTripUpdate
// ============================================================

func TestNotifyTripUpdate_NoTripID(t *testing.T) {
	t.Parallel()
	s := &Server{
		handlers: make(map[string]ToolHandler),
		subs:     make(map[string]bool),
	}
	// Should not panic with nil args or empty trip_id.
	s.notifyTripUpdate(nil)
	s.notifyTripUpdate(map[string]any{})
}

func TestNotifyTripUpdate_WithName(t *testing.T) {
	t.Parallel()
	s := &Server{
		handlers: make(map[string]ToolHandler),
		subs:     make(map[string]bool),
	}
	// create_trip has name but no trip_id — should still not panic.
	s.notifyTripUpdate(map[string]any{"name": "My Trip"})
}

func TestNotifyTripUpdate_WithTripID(t *testing.T) {
	t.Parallel()
	s := &Server{
		handlers: make(map[string]ToolHandler),
		subs:     make(map[string]bool),
	}
	// With trip_id, should not panic (no writer = no-op).
	s.notifyTripUpdate(map[string]any{"trip_id": "trip_abc"})
}

// ============================================================
// addResourceLinks
// ============================================================

func TestAddResourceLinks_FlightSearch(t *testing.T) {
	t.Parallel()
	s := &Server{
		handlers: make(map[string]ToolHandler),
		subs:     make(map[string]bool),
	}
	args := map[string]any{
		"origin":         "hel",
		"destination":    "bcn",
		"departure_date": "2026-07-01",
	}
	content := s.addResourceLinks(nil, args)
	if len(content) != 1 {
		t.Fatalf("expected 1 resource link, got %d", len(content))
	}
	if content[0].Type != "resource_link" {
		t.Errorf("type = %q, want resource_link", content[0].Type)
	}
	if !strings.Contains(content[0].URI, "HEL-BCN") {
		t.Errorf("URI = %q, should contain HEL-BCN", content[0].URI)
	}
}

func TestAddResourceLinks_HotelSearch(t *testing.T) {
	t.Parallel()
	s := &Server{
		handlers: make(map[string]ToolHandler),
		subs:     make(map[string]bool),
	}
	args := map[string]any{
		"location":  "Barcelona Spain",
		"check_in":  "2026-07-01",
		"check_out": "2026-07-08",
	}
	content := s.addResourceLinks(nil, args)
	if len(content) != 1 {
		t.Fatalf("expected 1 resource link, got %d", len(content))
	}
	if !strings.Contains(content[0].URI, "Barcelona_Spain") {
		t.Errorf("URI = %q, should sanitize spaces to underscores", content[0].URI)
	}
}

func TestAddResourceLinks_NoMatch(t *testing.T) {
	t.Parallel()
	s := &Server{
		handlers: make(map[string]ToolHandler),
		subs:     make(map[string]bool),
	}
	content := s.addResourceLinks(nil, map[string]any{})
	if len(content) != 0 {
		t.Errorf("expected 0 resource links, got %d", len(content))
	}
}

// ============================================================
// getLogLevel / setLogLevel
// ============================================================

func TestGetSetLogLevel(t *testing.T) {
	orig := getLogLevel()
	defer setLogLevel(orig)

	setLogLevel("debug")
	if got := getLogLevel(); got != "debug" {
		t.Errorf("got %q, want debug", got)
	}
	setLogLevel("error")
	if got := getLogLevel(); got != "error" {
		t.Errorf("got %q, want error", got)
	}
}
