package hacks_test

import (
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/hacks"
)

// TestHiddenCityFixture_AMS_HEL_RIX tests the canonical AMS→HEL→RIX route where
// the user disembarks at HEL (home airport) on an AMS→RIX ticket via Finnair,
// saving ~30% vs a direct AMS→HEL booking.
func TestHiddenCityFixture_AMS_HEL_RIX(t *testing.T) {
	// Fixture: AY sells AMS→HEL→RIX for €89.50; direct AMS→HEL costs €129.
	offers := []hacks.HiddenCityOffer{
		{
			Origin:          "AMS",
			Hub:             "HEL",
			HubBeyond:       "RIX",
			Carrier:         "AY",
			Price:           89.50,
			Currency:        "EUR",
			CarryOnOnly:     true, // boarding pass issued to HEL only
			SeparateTickets: false,
			LayoverMinutes:  90, // comfortable HEL layover
		},
	}
	candidates := hacks.ExpandMatrix(offers, hacks.MatrixOptions{
		AllowHiddenCity: true,
		DirectBaseline:  129.0,
		DepartDate:      "2026-05-15",
	})

	if len(candidates) == 0 {
		t.Fatal("expected at least one hidden-city candidate for AMS→HEL→RIX")
	}
	c := candidates[0]
	if c.Origin != "AMS" {
		t.Errorf("origin: want AMS, got %s", c.Origin)
	}
	if c.Hub != "HEL" {
		t.Errorf("hub: want HEL, got %s", c.Hub)
	}
	if c.HubBeyond != "RIX" {
		t.Errorf("hub_beyond: want RIX, got %s", c.HubBeyond)
	}
	if c.Price != 89.50 {
		t.Errorf("price: want 89.50, got %.2f", c.Price)
	}
	if c.SavingsEUR <= 0 {
		t.Errorf("savings_eur: want >0, got %.2f", c.SavingsEUR)
	}
	if c.SavingsPct <= 0 {
		t.Errorf("savings_pct: want >0, got %.2f", c.SavingsPct)
	}
	if c.BookingURL == "" {
		t.Error("booking_url: want non-empty")
	}
	// AY should produce a Finnair booking URL
	if !strings.Contains(c.BookingURL, "finnair.com") {
		t.Errorf("booking_url: want finnair.com URL, got %s", c.BookingURL)
	}
}

// TestHiddenCityFixture_AMS_FRA_Stop tests AMS→FRA routing where FRA is the
// desired destination but LH sells it cheaper as AMS→FRA→MUC.
func TestHiddenCityFixture_AMS_FRA_Stop(t *testing.T) {
	offers := []hacks.HiddenCityOffer{
		{
			Origin:          "AMS",
			Hub:             "FRA",
			HubBeyond:       "MUC",
			Carrier:         "LH",
			Price:           65.00,
			Currency:        "EUR",
			CarryOnOnly:     true,
			SeparateTickets: false,
			LayoverMinutes:  75,
		},
	}
	candidates := hacks.ExpandMatrix(offers, hacks.MatrixOptions{
		AllowHiddenCity: true,
		DirectBaseline:  95.0,
		DepartDate:      "2026-06-01",
	})

	if len(candidates) == 0 {
		t.Fatal("expected at least one hidden-city candidate for AMS→FRA→MUC")
	}
	c := candidates[0]
	if c.Hub != "FRA" {
		t.Errorf("hub: want FRA, got %s", c.Hub)
	}
	if c.HubBeyond != "MUC" {
		t.Errorf("hub_beyond: want MUC, got %s", c.HubBeyond)
	}
	// LH group should produce a Lufthansa booking URL
	if !strings.Contains(c.BookingURL, "lufthansa.com") {
		t.Errorf("booking_url: want lufthansa.com URL, got %s", c.BookingURL)
	}
}
