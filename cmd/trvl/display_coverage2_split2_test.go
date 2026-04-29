package main

import (
	"context"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/hotels"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/trip"
	"github.com/MikkoParkkola/trvl/internal/tripwindow"
	"github.com/MikkoParkkola/trvl/internal/visa"
	"github.com/MikkoParkkola/trvl/internal/weather"
)

func TestMaybeShowAccomHackTip_InvalidDates(t *testing.T) {
	models.UseColor = false

	out := captureStdout(t, func() {
		maybeShowAccomHackTip(context.Background(), "Prague", "not-a-date", "also-not", "EUR", 2)
	})

	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no output for invalid dates, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// maybeShowStarNudge
// ---------------------------------------------------------------------------

func TestMaybeShowStarNudge_NonSearchCommand(t *testing.T) {
	// Should be a no-op for non-search commands.
	// We just verify it does not panic.
	maybeShowStarNudge("version", "table")
}

// ---------------------------------------------------------------------------
// printReviewsTable
// ---------------------------------------------------------------------------

func TestPrintReviewsTable_WithReviews(t *testing.T) {
	models.UseColor = false

	result := &models.HotelReviewResult{
		Success: true,
		HotelID: "/g/11test",
		Name:    "Grand Hotel",
		Summary: models.ReviewSummary{
			AverageRating: 4.2,
			TotalReviews:  150,
		},
		Reviews: []models.HotelReview{
			{Rating: 5.0, Author: "Alice", Date: "2026-03-01", Text: "Excellent stay!"},
			{Rating: 3.0, Author: "Bob", Date: "2026-02-15", Text: "Decent but noisy."},
		},
	}

	out := captureStdout(t, func() {
		err := printReviewsTable(result)
		if err != nil {
			t.Errorf("printReviewsTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Grand Hotel",
		"4.2/5",
		"150 total reviews",
		"Alice",
		"Bob",
		"Excellent stay!",
		"Decent but noisy",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintReviewsTable_NoReviews(t *testing.T) {
	models.UseColor = false

	result := &models.HotelReviewResult{
		Success: true,
		Summary: models.ReviewSummary{AverageRating: 0, TotalReviews: 0},
	}

	out := captureStdout(t, func() {
		_ = printReviewsTable(result)
	})

	if !strings.Contains(out, "No reviews found") {
		t.Errorf("expected 'No reviews found', got: %s", out)
	}
}

func TestPrintReviewsTable_LongReviewTruncated(t *testing.T) {
	models.UseColor = false

	longText := strings.Repeat("A", 100)
	result := &models.HotelReviewResult{
		Success: true,
		Summary: models.ReviewSummary{AverageRating: 4.0, TotalReviews: 1},
		Reviews: []models.HotelReview{
			{Rating: 4.0, Author: "Test", Date: "2026-01-01", Text: longText},
		},
	}

	out := captureStdout(t, func() {
		_ = printReviewsTable(result)
	})

	if !strings.Contains(out, "...") {
		t.Errorf("expected truncated review with '...'")
	}
}

// ---------------------------------------------------------------------------
// starRating
// ---------------------------------------------------------------------------

func TestStarRating_FullStars(t *testing.T) {
	got := starRating(5.0)
	fullStars := strings.Count(got, "\u2605")
	if fullStars != 5 {
		t.Errorf("starRating(5.0) has %d full stars, want 5", fullStars)
	}
}

func TestStarRating_HalfStar(t *testing.T) {
	got := starRating(3.5)
	fullStars := strings.Count(got, "\u2605")
	if fullStars != 3 {
		t.Errorf("starRating(3.5) has %d full stars, want 3", fullStars)
	}
}

func TestStarRating_Zero(t *testing.T) {
	got := starRating(0)
	emptyStars := strings.Count(got, "\u2606")
	if emptyStars != 5 {
		t.Errorf("starRating(0) has %d empty stars, want 5", emptyStars)
	}
}

// ---------------------------------------------------------------------------
// formatRoomsTable
// ---------------------------------------------------------------------------

func TestFormatRoomsTable_WithRooms(t *testing.T) {
	models.UseColor = false

	result := &hotels.RoomAvailability{
		Success:  true,
		HotelID:  "/g/11test",
		Name:     "Hotel Lutetia",
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
		Rooms: []hotels.RoomType{
			{Name: "Standard Double", Price: 200, Currency: "EUR", MaxGuests: 2, Provider: "Google"},
			{Name: "Deluxe Suite", Price: 450, Currency: "EUR", MaxGuests: 3, Provider: "Google", Amenities: []string{"minibar", "balcony"}},
		},
	}

	out := captureStdout(t, func() {
		err := formatRoomsTable(result)
		if err != nil {
			t.Errorf("formatRoomsTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Rooms",
		"Hotel Lutetia",
		"2026-06-15",
		"2026-06-18",
		"Standard Double",
		"Deluxe Suite",
		"200",
		"450",
		"minibar",
		"Cheapest: 200 EUR",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestFormatRoomsTable_NoRooms(t *testing.T) {
	models.UseColor = false

	result := &hotels.RoomAvailability{
		Success:  true,
		HotelID:  "/g/11test",
		Name:     "Empty Hotel",
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
	}

	out := captureStdout(t, func() {
		_ = formatRoomsTable(result)
	})

	if !strings.Contains(out, "No room types found") {
		t.Errorf("expected 'No room types found', got: %s", out)
	}
}

func TestFormatRoomsTable_FallbackToHotelID(t *testing.T) {
	models.UseColor = false

	result := &hotels.RoomAvailability{
		Success:  true,
		HotelID:  "/g/11test",
		Name:     "", // no name
		CheckIn:  "2026-06-15",
		CheckOut: "2026-06-18",
	}

	out := captureStdout(t, func() {
		_ = formatRoomsTable(result)
	})

	if !strings.Contains(out, "/g/11test") {
		t.Errorf("expected hotel ID as fallback name")
	}
}

// ---------------------------------------------------------------------------
// printVisaResult
// ---------------------------------------------------------------------------

func TestPrintVisaResult_VisaFree(t *testing.T) {
	models.UseColor = false

	result := visa.Result{
		Success: true,
		Requirement: visa.Requirement{
			Passport:    "FI",
			Destination: "JP",
			Status:      "visa-free",
			MaxStay:     "90 days",
			Notes:       "Purpose of visit must be tourism or business.",
		},
	}

	out := captureStdout(t, func() {
		err := printVisaResult(result)
		if err != nil {
			t.Errorf("printVisaResult returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Visa",
		"FI",
		"JP",
		"Visa free",
		"90 days",
		"Purpose of visit",
		"advisory only",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintVisaResult_VisaRequired(t *testing.T) {
	models.UseColor = false

	result := visa.Result{
		Success: true,
		Requirement: visa.Requirement{
			Passport:    "US",
			Destination: "CN",
			Status:      "visa-required",
			MaxStay:     "30 days",
		},
	}

	out := captureStdout(t, func() {
		_ = printVisaResult(result)
	})

	if !strings.Contains(out, "Visa required") {
		t.Errorf("expected 'Visa required' in output")
	}
}

func TestPrintVisaResult_NoMaxStay(t *testing.T) {
	models.UseColor = false

	result := visa.Result{
		Success: true,
		Requirement: visa.Requirement{
			Passport:    "FI",
			Destination: "SE",
			Status:      "freedom-of-movement",
		},
	}

	out := captureStdout(t, func() {
		_ = printVisaResult(result)
	})

	// Should NOT show "Max stay:" when empty.
	if strings.Contains(out, "Max stay:") {
		t.Errorf("should not show Max stay when empty")
	}
}

// ---------------------------------------------------------------------------
// colorizeVisaStatus
// ---------------------------------------------------------------------------

func TestColorizeVisaStatus_AllStatuses(t *testing.T) {
	models.UseColor = false

	tests := []struct {
		status string
		label  string
		want   string
	}{
		{"visa-free", "Visa free", "Visa free"},
		{"freedom-of-movement", "Freedom of movement", "Freedom of movement"},
		{"visa-on-arrival", "Visa on arrival", "Visa on arrival"},
		{"e-visa", "E-visa", "E-visa"},
		{"visa-required", "Visa required", "Visa required"},
		{"unknown-status", "Unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := colorizeVisaStatus(tt.status, tt.label)
			if !strings.Contains(got, tt.want) {
				t.Errorf("colorizeVisaStatus(%q, %q) = %q, want to contain %q", tt.status, tt.label, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// printWeatherTable
// ---------------------------------------------------------------------------

func TestPrintWeatherTable_WithForecasts(t *testing.T) {
	models.UseColor = false

	result := &weather.WeatherResult{
		Success: true,
		City:    "Prague",
		Forecasts: []weather.Forecast{
			{Date: "2026-04-12", TempMax: 18, TempMin: 8, Precipitation: 0, Description: "Sunny"},
			{Date: "2026-04-13", TempMax: 28, TempMin: 15, Precipitation: 6, Description: "Rain"},
			{Date: "2026-04-14", TempMax: 3, TempMin: -2, Precipitation: 0, Description: "Clear"},
		},
	}

	out := captureStdout(t, func() {
		err := printWeatherTable(result, "2026-04-12", "2026-04-14")
		if err != nil {
			t.Errorf("printWeatherTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Weather",
		"Prague",
		"Sunny",
		"Rain",
		"6mm",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintWeatherTable_NoForecasts(t *testing.T) {
	models.UseColor = false

	result := &weather.WeatherResult{
		Success: true,
		City:    "Unknown",
	}

	out := captureStdout(t, func() {
		_ = printWeatherTable(result, "2026-04-12", "2026-04-14")
	})

	if !strings.Contains(out, "No forecast data") {
		t.Errorf("expected 'No forecast data', got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// printWeekendTable
// ---------------------------------------------------------------------------

func TestPrintWeekendTable_WithDestinations(t *testing.T) {
	models.UseColor = false

	result := &trip.WeekendResult{
		Success: true,
		Origin:  "HEL",
		Month:   "July 2026",
		Nights:  2,
		Count:   2,
		Destinations: []trip.WeekendDestination{
			{
				Destination: "Tallinn",
				AirportCode: "TLL",
				FlightPrice: 49,
				HotelPrice:  80,
				HotelName:   "CityBox Tallinn",
				Total:       129,
				Currency:    "EUR",
				Stops:       0,
			},
			{
				Destination: "Stockholm",
				AirportCode: "ARN",
				FlightPrice: 89,
				HotelPrice:  120,
				Total:       209,
				Currency:    "EUR",
				Stops:       0,
			},
		},
	}

	out := captureStdout(t, func() {
		err := printWeekendTable(context.Background(), "", result)
		if err != nil {
			t.Errorf("printWeekendTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Weekend getaways from HEL",
		"July 2026",
		"2 nights",
		"Tallinn",
		"TLL",
		"EUR 49",
		"CityBox Tallinn",
		"EUR 129",
		"Stockholm",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintWeekendTable_Empty(t *testing.T) {
	models.UseColor = false

	result := &trip.WeekendResult{
		Success: true,
		Origin:  "HEL",
		Count:   0,
	}

	out := captureStdout(t, func() {
		_ = printWeekendTable(context.Background(), "", result)
	})

	if !strings.Contains(out, "No destinations found") {
		t.Errorf("expected 'No destinations found'")
	}
}

func TestPrintWeekendTable_Failed(t *testing.T) {
	models.UseColor = false

	result := &trip.WeekendResult{
		Success: false,
		Error:   "timeout",
	}

	// Error goes to stderr; just verify no panic.
	captureStdout(t, func() {
		_ = printWeekendTable(context.Background(), "", result)
	})
}

// ---------------------------------------------------------------------------
// printWhenTable
// ---------------------------------------------------------------------------

func TestPrintWhenTable_WithCandidates(t *testing.T) {
	models.UseColor = false

	candidates := []tripwindow.Candidate{
		{
			Start:             "2026-07-10",
			End:               "2026-07-14",
			Nights:            4,
			EstimatedCost:     350,
			FlightCost:        200,
			HotelCost:         150,
			HotelName:         "CityBox",
			Currency:          "EUR",
			OverlapsPreferred: true,
		},
		{
			Start:         "2026-07-17",
			End:           "2026-07-20",
			Nights:        3,
			EstimatedCost: 0,
			FlightCost:    0,
			HotelCost:     0,
			Currency:      "EUR",
		},
	}

	out := captureStdout(t, func() {
		err := printWhenTable(candidates, "HEL", "BCN")
		if err != nil {
			t.Errorf("printWhenTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Travel windows from HEL to BCN",
		"2026-07-10",
		"2026-07-14",
		"EUR 350",
		"EUR 200",
		"preferred",
		"CityBox",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintWhenTable_Empty(t *testing.T) {
	models.UseColor = false

	out := captureStdout(t, func() {
		_ = printWhenTable(nil, "HEL", "NRT")
	})

	if !strings.Contains(out, "No open travel windows") {
		t.Errorf("expected 'No open travel windows'")
	}
}

// ---------------------------------------------------------------------------
// printSuggestTable
// ---------------------------------------------------------------------------

func TestPrintSuggestTable_Success(t *testing.T) {
	models.UseColor = false

	result := &trip.SmartDateResult{
		Success:      true,
		Origin:       "HEL",
		Destination:  "BCN",
		AveragePrice: 250,
		Currency:     "EUR",
		CheapestDates: []trip.CheapDate{
			{Date: "2026-07-08", DayOfWeek: "Wednesday", Price: 199, Currency: "EUR"},
			{Date: "2026-07-15", DayOfWeek: "Wednesday", Price: 210, Currency: "EUR"},
		},
		Insights: []trip.DateInsight{
			{Type: "cheapest", Description: "Wednesday departures are 15% cheaper on average."},
		},
	}

	out := captureStdout(t, func() {
		err := printSuggestTable(context.Background(), "", result)
		if err != nil {
			t.Errorf("printSuggestTable returned error: %v", err)
		}
	})

	for _, want := range []string{
		"HEL",
		"BCN",
		"EUR 250",
		"2026-07-08",
		"Wednesday",
		"EUR 199",
		"15% cheaper",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintSuggestTable_Failed(t *testing.T) {
	models.UseColor = false

	result := &trip.SmartDateResult{
		Success: false,
		Error:   "no dates",
	}

	// Error goes to stderr; no panic.
	captureStdout(t, func() {
		_ = printSuggestTable(context.Background(), "", result)
	})
}

// ---------------------------------------------------------------------------
// printTripDetail
// ---------------------------------------------------------------------------
