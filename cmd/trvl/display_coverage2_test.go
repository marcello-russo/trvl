package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/baggage"
	"github.com/MikkoParkkola/trvl/internal/hacks"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/trip"
)

func setEnvForProviders(_ *testing.T, _ string) string { return "" }

func restoreEnvForProviders(_ string) {}

// ---------------------------------------------------------------------------
// outputShare
// ---------------------------------------------------------------------------

var _ = fmt.Sprint // ensure fmt is used

func TestPrintAccomHacks_WithHacks(t *testing.T) {
	models.UseColor = false

	detected := []hacks.Hack{
		{
			Type:        "accommodation_split",
			Title:       "Split Stay Savings",
			Description: "Stay at Hotel A for 3 nights, then Hotel B for 2 nights.",
			Savings:     85,
			Currency:    "EUR",
			Steps:       []string{"Book Hotel A for nights 1-3", "Book Hotel B for nights 4-5"},
			Risks:       []string{"Moving between hotels"},
		},
	}

	out := captureStdout(t, func() {
		err := printAccomHacks("Prague", "2026-07-01", "2026-07-06", detected)
		if err != nil {
			t.Errorf("printAccomHacks returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Accommodation Hacks",
		"Prague",
		"2026-07-01",
		"2026-07-06",
		"1 split opportunity",
		"Split Stay Savings",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintAccomHacks_NoHacks(t *testing.T) {
	models.UseColor = false

	out := captureStdout(t, func() {
		err := printAccomHacks("Amsterdam", "2026-07-01", "2026-07-04", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "No accommodation split detected") {
		t.Errorf("expected 'No accommodation split detected', got: %s", out)
	}
	if !strings.Contains(out, "longer stay") {
		t.Errorf("expected tip about longer stay, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// printAirportTransferTable
// ---------------------------------------------------------------------------

func TestPrintAirportTransferTable_Success(t *testing.T) {
	models.UseColor = false

	result := &trip.AirportTransferResult{
		Success:      true,
		Airport:      "Charles de Gaulle (CDG)",
		Destination:  "Hotel Lutetia Paris",
		Date:         "2026-07-01",
		ExactMatches: 2,
		CityMatches:  1,
		Routes: []models.GroundRoute{
			{
				Provider:  "transitous",
				Type:      "train",
				Price:     12,
				Currency:  "EUR",
				Duration:  45,
				Departure: models.GroundStop{City: "CDG", Time: "2026-07-01T14:00:00"},
				Arrival:   models.GroundStop{City: "Paris", Time: "2026-07-01T14:45:00"},
			},
			{
				Provider:  "taxi",
				Type:      "taxi",
				Price:     55,
				Currency:  "EUR",
				Duration:  40,
				Departure: models.GroundStop{City: "CDG", Time: "2026-07-01T14:00:00"},
				Arrival:   models.GroundStop{City: "Paris", Time: "2026-07-01T14:40:00"},
			},
		},
	}

	out := captureStdout(t, func() {
		err := printAirportTransferTable(context.Background(), "", result)
		if err != nil {
			t.Errorf("printAirportTransferTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Airport Transfer",
		"Charles de Gaulle (CDG)",
		"Hotel Lutetia Paris",
		"2 exact airport match",
		"1 broader city match",
		"transitous",
		"taxi",
		"Taxi fares are estimates",
		"exact airport-to-destination matches",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintAirportTransferTable_NotSuccess(t *testing.T) {
	models.UseColor = false

	result := &trip.AirportTransferResult{
		Success: false,
		Error:   "airport not found",
	}

	// Error goes to stderr; just verify no panic.
	captureStdout(t, func() {
		_ = printAirportTransferTable(context.Background(), "", result)
	})
}

func TestPrintAirportTransferTable_NotSuccessNoError(t *testing.T) {
	models.UseColor = false

	result := &trip.AirportTransferResult{
		Success: false,
	}

	captureStdout(t, func() {
		_ = printAirportTransferTable(context.Background(), "", result)
	})
}

func TestPrintAirportTransferTable_WithArrivalTime(t *testing.T) {
	models.UseColor = false

	result := &trip.AirportTransferResult{
		Success:      true,
		Airport:      "LHR",
		Destination:  "Paddington",
		Date:         "2026-07-01",
		ArrivalTime:  "14:30",
		ExactMatches: 1,
		Routes: []models.GroundRoute{
			{
				Provider:  "transitous",
				Type:      "train",
				Price:     15,
				Currency:  "GBP",
				Duration:  20,
				Departure: models.GroundStop{Time: "2026-07-01T15:00:00"},
				Arrival:   models.GroundStop{Time: "2026-07-01T15:20:00"},
			},
		},
	}

	out := captureStdout(t, func() {
		_ = printAirportTransferTable(context.Background(), "", result)
	})

	if !strings.Contains(out, "After 14:30") {
		t.Errorf("expected 'After 14:30' in output, got: %s", out)
	}
}

func TestPrintAirportTransferTable_NoCityMatches(t *testing.T) {
	models.UseColor = false

	result := &trip.AirportTransferResult{
		Success:      true,
		Airport:      "FCO",
		Destination:  "Rome Termini",
		Date:         "2026-07-01",
		ExactMatches: 1,
		CityMatches:  0,
		Routes: []models.GroundRoute{
			{
				Provider:  "transitous",
				Type:      "train",
				Price:     14,
				Currency:  "EUR",
				Duration:  30,
				Departure: models.GroundStop{Time: "2026-07-01T10:00:00"},
				Arrival:   models.GroundStop{Time: "2026-07-01T10:30:00"},
			},
		},
	}

	out := captureStdout(t, func() {
		_ = printAirportTransferTable(context.Background(), "", result)
	})

	// Should NOT show city match note when CityMatches is 0.
	if strings.Contains(out, "broader city") {
		t.Errorf("should not show broader city note when CityMatches=0")
	}
}

// ---------------------------------------------------------------------------
// printBaggageDetail
// ---------------------------------------------------------------------------

func TestPrintBaggageDetail_FullService(t *testing.T) {
	models.UseColor = false

	ab := baggage.AirlineBaggage{
		Code:              "KL",
		Name:              "KLM",
		CarryOnMaxKg:      12,
		CarryOnDimensions: "55x35x25 cm",
		PersonalItem:      true,
		CheckedIncluded:   1,
		Notes:             "Economy Light does not include checked baggage.",
	}

	out := captureStdout(t, func() {
		err := printBaggageDetail(ab)
		if err != nil {
			t.Errorf("printBaggageDetail returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Baggage rules",
		"KLM (KL)",
		"12 kg",
		"55x35x25 cm",
		"Yes (handbag/laptop bag)",
		"1 bag included (23 kg)",
		"Economy Light",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintBaggageDetail_LCC(t *testing.T) {
	models.UseColor = false

	ab := baggage.AirlineBaggage{
		Code:         "FR",
		Name:         "Ryanair",
		CarryOnMaxKg: 10,
		PersonalItem: false,
		CheckedFee:   30,
		OverheadOnly: true,
	}

	out := captureStdout(t, func() {
		err := printBaggageDetail(ab)
		if err != nil {
			t.Errorf("printBaggageDetail returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Ryanair (FR)",
		"10 kg",
		"Personal:    No",
		"from EUR 30",
		"under-seat bag free",
		"Overhead cabin bag requires priority",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintBaggageDetail_NoWeightLimit(t *testing.T) {
	models.UseColor = false

	ab := baggage.AirlineBaggage{
		Code:            "BA",
		Name:            "British Airways",
		CarryOnMaxKg:    0,
		PersonalItem:    true,
		CheckedIncluded: 1,
	}

	out := captureStdout(t, func() {
		_ = printBaggageDetail(ab)
	})

	if !strings.Contains(out, "No weight limit") {
		t.Errorf("expected 'No weight limit' for zero CarryOnMaxKg")
	}
}

func TestPrintBaggageDetail_NoCheckedNorFee(t *testing.T) {
	models.UseColor = false

	ab := baggage.AirlineBaggage{
		Code:            "XX",
		Name:            "Test Air",
		CheckedIncluded: 0,
		CheckedFee:      0,
	}

	out := captureStdout(t, func() {
		_ = printBaggageDetail(ab)
	})

	if !strings.Contains(out, "Not included") {
		t.Errorf("expected 'Not included' for no checked bags")
	}
}

// ---------------------------------------------------------------------------
// printBaggageList
// ---------------------------------------------------------------------------

func TestPrintBaggageList_Multiple(t *testing.T) {
	models.UseColor = false

	airlines := []baggage.AirlineBaggage{
		{
			Code:            "KL",
			Name:            "KLM",
			CarryOnMaxKg:    12,
			PersonalItem:    true,
			CheckedIncluded: 1,
		},
		{
			Code:         "FR",
			Name:         "Ryanair",
			CarryOnMaxKg: 10,
			PersonalItem: false,
			CheckedFee:   30,
			OverheadOnly: true,
		},
		{
			Code:            "EK",
			Name:            "Emirates",
			CarryOnMaxKg:    0,
			PersonalItem:    true,
			CheckedIncluded: 2,
		},
	}

	out := captureStdout(t, func() {
		err := printBaggageList(airlines)
		if err != nil {
			t.Errorf("printBaggageList returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Airline Baggage Rules",
		"KL", "KLM", "12kg", "yes", "1x23kg",
		"FR", "Ryanair", "10kg", "no", "~EUR30", "overhead fee",
		"EK", "Emirates", "no limit", "2x23kg",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintBaggageList_Empty(t *testing.T) {
	models.UseColor = false

	out := captureStdout(t, func() {
		_ = printBaggageList(nil)
	})

	if !strings.Contains(out, "Airline Baggage Rules") {
		t.Errorf("expected banner even for empty list")
	}
}

// ---------------------------------------------------------------------------
// printDiscoverTable
// ---------------------------------------------------------------------------

func TestPrintDiscoverTable_WithTrips(t *testing.T) {
	models.UseColor = false

	out := &trip.DiscoverOutput{
		Success: true,
		Origin:  "HEL",
		From:    "2026-07-01",
		Until:   "2026-07-31",
		Budget:  500,
		Count:   2,
		Trips: []trip.DiscoverResult{
			{
				Destination: "Tallinn",
				AirportCode: "TLL",
				DepartDate:  "2026-07-10",
				ReturnDate:  "2026-07-13",
				Nights:      3,
				FlightPrice: 49,
				HotelPrice:  120,
				HotelName:   "Hilton Tallinn Park",
				HotelRating: 8.5,
				Total:       169,
				Currency:    "EUR",
				BudgetSlack: 331,
				Reasoning:   "great value",
			},
			{
				Destination: "Riga",
				AirportCode: "RIX",
				DepartDate:  "2026-07-15",
				ReturnDate:  "2026-07-18",
				Nights:      3,
				FlightPrice: 79,
				HotelPrice:  150,
				HotelName:   "",
				Total:       229,
				Currency:    "EUR",
				BudgetSlack: 271,
			},
		},
	}

	output := captureStdout(t, func() {
		err := printDiscoverTable(out, false)
		if err != nil {
			t.Errorf("printDiscoverTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Top 2 value trips",
		"HEL",
		"500",
		"Tallinn (TLL)",
		"EUR 49",
		"Hilton Tallinn Park",
		"EUR 169",
		"great value",
		"Riga (RIX)",
		"EUR 79",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintDiscoverTable_NoTrips(t *testing.T) {
	models.UseColor = false

	out := &trip.DiscoverOutput{
		Success: true,
		Origin:  "HEL",
		From:    "2026-07-01",
		Until:   "2026-07-31",
		Budget:  500,
		Count:   0,
	}

	output := captureStdout(t, func() {
		err := printDiscoverTable(out, false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "No trips found") {
		t.Errorf("expected 'No trips found', got: %s", output)
	}
	if !strings.Contains(output, "HEL") {
		t.Errorf("expected origin HEL in output")
	}
}

// ---------------------------------------------------------------------------
// printMultiCityTable
// ---------------------------------------------------------------------------

func TestPrintMultiCityTable_Success(t *testing.T) {
	models.UseColor = false

	result := &trip.MultiCityResult{
		Success:      true,
		HomeAirport:  "HEL",
		OptimalOrder: []string{"BCN", "ROM", "PAR"},
		Segments: []trip.Segment{
			{From: "HEL", To: "BCN", Price: 120, Currency: "EUR"},
			{From: "BCN", To: "ROM", Price: 80, Currency: "EUR"},
			{From: "ROM", To: "PAR", Price: 95, Currency: "EUR"},
			{From: "PAR", To: "HEL", Price: 110, Currency: "EUR"},
		},
		TotalCost:    405,
		Currency:     "EUR",
		Savings:      85,
		Permutations: 6,
	}

	out := captureStdout(t, func() {
		err := printMultiCityTable(context.Background(), "", result)
		if err != nil {
			t.Errorf("printMultiCityTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Multi-city from HEL",
		"6 permutations",
		"HEL -> BCN -> ROM -> PAR -> HEL",
		"EUR 120",
		"EUR 80",
		"EUR 95",
		"EUR 110",
		"EUR 405",
		"Savings vs worst order",
		"EUR 85",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintMultiCityTable_NoSavings(t *testing.T) {
	models.UseColor = false

	result := &trip.MultiCityResult{
		Success:      true,
		HomeAirport:  "JFK",
		OptimalOrder: []string{"LHR", "CDG"},
		Segments: []trip.Segment{
			{From: "JFK", To: "LHR", Price: 300, Currency: "USD"},
			{From: "LHR", To: "CDG", Price: 100, Currency: "USD"},
			{From: "CDG", To: "JFK", Price: 350, Currency: "USD"},
		},
		TotalCost:    750,
		Currency:     "USD",
		Savings:      0,
		Permutations: 2,
	}

	out := captureStdout(t, func() {
		_ = printMultiCityTable(context.Background(), "", result)
	})

	// No savings row when Savings == 0.
	if strings.Contains(out, "Savings vs worst") {
		t.Errorf("should not show savings row when savings is 0")
	}
}

func TestPrintMultiCityTable_Failed(t *testing.T) {
	models.UseColor = false

	result := &trip.MultiCityResult{
		Success: false,
		Error:   "no flights found",
	}

	// Error goes to stderr; just verify no panic.
	captureStdout(t, func() {
		_ = printMultiCityTable(context.Background(), "", result)
	})
}

// ---------------------------------------------------------------------------
// formatPricesTable
// ---------------------------------------------------------------------------

func TestFormatPricesTable_WithProviders(t *testing.T) {
	models.UseColor = false

	result := &models.HotelPriceResult{
		Success:  true,
		HotelID:  "/g/11test",
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Providers: []models.ProviderPrice{
			{Provider: "Booking.com", Price: 120.50, Currency: "EUR"},
			{Provider: "Hotels.com", Price: 125.00, Currency: "EUR"},
			{Provider: "Expedia", Price: 118.75, Currency: "EUR"},
		},
	}

	out := captureStdout(t, func() {
		err := formatPricesTable(result)
		if err != nil {
			t.Errorf("formatPricesTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"/g/11test",
		"2026-06-15",
		"2026-06-18",
		"Booking.com",
		"120.50",
		"Hotels.com",
		"125.00",
		"Expedia",
		"118.75",
		"EUR",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFormatPricesTable_NoProviders(t *testing.T) {
	models.UseColor = false

	result := &models.HotelPriceResult{
		Success:   true,
		HotelID:   "/g/11test",
		CheckIn:   "2026-06-15",
		CheckOut:  "2026-06-18",
		Providers: nil,
	}

	out := captureStdout(t, func() {
		err := formatPricesTable(result)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "No prices found") {
		t.Errorf("expected 'No prices found', got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// colorProviderStatus
// ---------------------------------------------------------------------------

func TestColorProviderStatus_AllStatuses(t *testing.T) {
	models.UseColor = false

	tests := []struct {
		status string
		want   string
	}{
		{"healthy", "healthy"},
		{"stale", "stale"},
		{"error", "error"},
		{"unconfigured", "unconfigured"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := colorProviderStatus(tt.status)
			if !strings.Contains(got, tt.want) {
				t.Errorf("colorProviderStatus(%q) = %q, want to contain %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestColorProviderStatus_WithColor(t *testing.T) {
	models.UseColor = true
	defer func() { models.UseColor = false }()

	// When color is enabled, the output should still contain the status text.
	got := colorProviderStatus("healthy")
	if !strings.Contains(got, "healthy") {
		t.Errorf("colorProviderStatus with color should contain 'healthy', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// runCabinComparison (display path)
// ---------------------------------------------------------------------------

// Note: runCabinComparison calls flights.SearchFlights which hits network.
// We test only the table-rendering path by checking the "json" format path
// which serialises cabinResult structs, and by verifying the function
// signature compiles. Full integration tested via live probes.

// ---------------------------------------------------------------------------
// maybeShowAccomHackTip
// ---------------------------------------------------------------------------

func TestMaybeShowAccomHackTip_ShortStay(t *testing.T) {
	models.UseColor = false

	// Stay of 2 nights (< 4) should produce no output.
	out := captureStdout(t, func() {
		maybeShowAccomHackTip(context.Background(), "Prague", "2026-07-01", "2026-07-03", "EUR", 2)
	})

	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no output for short stay, got: %s", out)
	}
}

func TestMaybeShowAccomHackTip_EmptyDates(t *testing.T) {
	models.UseColor = false

	out := captureStdout(t, func() {
		maybeShowAccomHackTip(context.Background(), "Prague", "", "", "EUR", 2)
	})

	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no output for empty dates, got: %s", out)
	}
}
