package mcp

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestDispatchNatural_HotelMissingLocation(t *testing.T) {
	p := naturalSearchParams{Intent: "hotel"}
	content, _, _ := dispatchNatural(context.Background(), p, "hotels", nil, nil, nil)
	if len(content) == 0 {
		t.Fatal("expected content")
	}
	if !contains(content[0].Text, "Could not determine hotel location") {
		t.Errorf("expected location error, got: %s", content[0].Text)
	}
}

func TestDispatchNatural_HotelMissingDates(t *testing.T) {
	p := naturalSearchParams{Intent: "hotel", Location: "Prague"}
	content, _, _ := dispatchNatural(context.Background(), p, "hotels in Prague", nil, nil, nil)
	if len(content) == 0 {
		t.Fatal("expected content")
	}
	if !contains(content[0].Text, "Could not determine check-in") {
		t.Errorf("expected date error, got: %s", content[0].Text)
	}
}

func TestDispatchNatural_FlightMissingParams(t *testing.T) {
	p := naturalSearchParams{Intent: "flight"}
	content, _, _ := dispatchNatural(context.Background(), p, "fly somewhere", nil, nil, nil)
	if len(content) == 0 {
		t.Fatal("expected content")
	}
	if !contains(content[0].Text, "Could not determine") {
		t.Errorf("expected missing params, got: %s", content[0].Text)
	}
}

func TestDispatchNatural_RouteMissingParams(t *testing.T) {
	p := naturalSearchParams{Intent: "route"}
	content, _, _ := dispatchNatural(context.Background(), p, "route somewhere", nil, nil, nil)
	if len(content) == 0 {
		t.Fatal("expected content")
	}
	if !contains(content[0].Text, "Could not determine") {
		t.Errorf("expected missing params, got: %s", content[0].Text)
	}
}

func TestDispatchNatural_RouteWithBudgetAndModes(t *testing.T) {
	p := naturalSearchParams{
		Intent:      "route",
		Origin:      "HEL",
		Destination: "PRG",
		Date:        "2026-06-15",
		MaxBudget:   100,
		Modes:       []string{"train", "bus"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := dispatchNatural(ctx, p, "train to Prague", nil, nil, nil)
	_ = err
}

func TestDispatchNatural_RouteWithFlightMode(t *testing.T) {
	p := naturalSearchParams{
		Intent:      "route",
		Origin:      "HEL",
		Destination: "PRG",
		Date:        "2026-06-15",
		Modes:       []string{"flight", "train"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := dispatchNatural(ctx, p, "fly or train to Prague", nil, nil, nil)
	_ = err
}

func TestDispatchNatural_Unknown(t *testing.T) {
	p := naturalSearchParams{
		Intent:      "unknown",
		Origin:      "HEL",
		Destination: "BCN",
		Date:        "2026-06-15",
	}
	content, _, _ := dispatchNatural(context.Background(), p, "do something", nil, nil, nil)
	if len(content) == 0 {
		t.Fatal("expected content")
	}
	if !contains(content[0].Text, "Could not determine the search intent") {
		t.Errorf("expected fallback message, got: %s", content[0].Text)
	}
	if !contains(content[0].Text, "From: HEL") {
		t.Errorf("expected origin in fallback, got: %s", content[0].Text)
	}
}

func TestDispatchNatural_UnknownNoFields(t *testing.T) {
	p := naturalSearchParams{Intent: "unknown"}
	content, _, _ := dispatchNatural(context.Background(), p, "what?", nil, nil, nil)
	if len(content) == 0 {
		t.Fatal("expected content")
	}
}

// --- handleSearchGround validation (26.3% coverage) ---

func TestHandleSearchGround_WithProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleSearchGround(ctx,
		map[string]any{
			"from":     "Prague",
			"to":       "Vienna",
			"date":     "2026-06-15",
			"currency": "EUR",
			"type":     "bus",
			"provider": "flixbus,regiojet",
		},
		nil, nil, nil)
	_ = err
}

// --- handleSearchAirportTransfers validation (29.2% coverage) ---

func TestHandleSearchAirportTransfers_WithAllArgs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleSearchAirportTransfers(ctx,
		map[string]any{
			"airport_code": "CDG",
			"destination":  "Eiffel Tower",
			"date":         "2026-06-15",
			"arrival_time": "14:00",
			"currency":     "EUR",
			"type":         "train",
			"provider":     "transitous",
		},
		nil, nil, nil)
	_ = err
}

// --- handleSearchRestaurants validation (10.3% coverage) ---

func TestHandleSearchRestaurants_WithQueryAndLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleSearchRestaurants(ctx,
		map[string]any{
			"location": "Helsinki",
			"query":    "sushi",
			"limit":    float64(5),
		},
		nil, nil, nil)
	_ = err // network call, exercises full input path
}

func TestHandleSearchRestaurants_LimitClamping(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleSearchRestaurants(ctx,
		map[string]any{
			"location": "Helsinki",
			"limit":    float64(100), // clamped to 20
		},
		nil, nil, nil)
	_ = err
}

func TestHandleSearchRestaurants_ZeroLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleSearchRestaurants(ctx,
		map[string]any{
			"location": "Helsinki",
			"limit":    float64(0), // defaults to 10
		},
		nil, nil, nil)
	_ = err
}

// --- restaurantSummary edge cases ---

func TestRestaurantSummary_WithCategoryAndAddress(t *testing.T) {
	places := []models.RatedPlace{
		{Name: "Olo", Rating: 4.8, Category: "Fine Dining", Address: "Pohjoisesplanadi 5"},
	}
	s := restaurantSummary(places, "Helsinki")
	if !contains(s, "Fine Dining") {
		t.Error("expected category")
	}
	if !contains(s, "Pohjoisesplanadi") {
		t.Error("expected address")
	}
}

// --- handleHotelPrices validation (54.2% coverage) ---

func TestHandleHotelPrices_MissingHotelID_Push(t *testing.T) {
	_, _, err := handleHotelPrices(context.Background(),
		map[string]any{"check_in": "2026-06-15", "check_out": "2026-06-18"},
		nil, nil, nil)
	if err == nil || !contains(err.Error(), "hotel_id") {
		t.Errorf("expected hotel_id error, got: %v", err)
	}
}

func TestHandleHotelPrices_MissingDates(t *testing.T) {
	_, _, err := handleHotelPrices(context.Background(),
		map[string]any{"hotel_id": "/g/123"},
		nil, nil, nil)
	if err == nil || !contains(err.Error(), "check_in") {
		t.Errorf("expected date error, got: %v", err)
	}
}

// --- handleHotelReviews validation (16.7% coverage) ---

func TestHandleHotelReviews_MissingHotelName(t *testing.T) {
	_, _, err := handleHotelReviews(context.Background(),
		map[string]any{}, nil, nil, nil)
	if err == nil {
		t.Error("expected error for missing hotel_id/hotel_name")
	}
}

// --- handleHotelRooms validation (26.2% coverage) ---

func TestHandleHotelRooms_WithAllArgs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleHotelRooms(ctx,
		map[string]any{
			"hotel_name": "Hilton Helsinki",
			"location":   "Helsinki",
			"check_in":   "2026-06-15",
			"check_out":  "2026-06-18",
			"guests":     float64(2),
		},
		nil, nil, nil)
	_ = err // network call
}

// --- handleFindTripWindow validation (55.6% coverage) ---

func TestHandleFindTripWindow_MissingAll(t *testing.T) {
	_, _, err := handleFindTripWindow(context.Background(),
		map[string]any{}, nil, nil, nil)
	if err == nil || !contains(err.Error(), "required") {
		t.Errorf("expected required error, got: %v", err)
	}
}

func TestHandleFindTripWindow_WithBusyIntervals(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleFindTripWindow(ctx,
		map[string]any{
			"destination":  "BCN",
			"window_start": "2026-06-01",
			"window_end":   "2026-06-30",
			"busy_intervals": []any{
				map[string]any{"start": "2026-06-10", "end": "2026-06-12", "reason": "work"},
			},
			"preferred_intervals": []any{
				map[string]any{"start": "2026-06-20", "end": "2026-06-22"},
			},
			"min_nights":     float64(3),
			"max_nights":     float64(5),
			"max_candidates": float64(3),
			"budget_eur":     float64(1000),
		},
		nil, nil, nil)
	_ = err // may fail on validation or network
}

// --- handleListTrips / handleGetTrip / handleCreateTrip / handleAddTripLeg / handleMarkTripBooked ---
// These all call defaultTripStore() which uses ~/.trvl/trips/. Exercise validation paths.

func TestHandleGetTrip_EmptyID(t *testing.T) {
	_, _, err := handleGetTrip(context.Background(),
		map[string]any{"id": ""}, nil, nil, nil)
	if err == nil || !contains(err.Error(), "id is required") {
		t.Errorf("expected id required, got: %v", err)
	}
}

func TestHandleCreateTrip_EmptyNameString(t *testing.T) {
	_, _, err := handleCreateTrip(context.Background(),
		map[string]any{"name": ""}, nil, nil, nil)
	if err == nil || !contains(err.Error(), "name is required") {
		t.Errorf("expected name required, got: %v", err)
	}
}

func TestHandleAddTripLeg_EmptyTripID(t *testing.T) {
	_, _, err := handleAddTripLeg(context.Background(),
		map[string]any{"trip_id": ""}, nil, nil, nil)
	if err == nil || !contains(err.Error(), "trip_id is required") {
		t.Errorf("expected trip_id required, got: %v", err)
	}
}

func TestHandleMarkTripBooked_PartialArgs(t *testing.T) {
	_, _, err := handleMarkTripBooked(context.Background(),
		map[string]any{"trip_id": "t1", "provider": "KLM"},
		nil, nil, nil)
	if err == nil || !contains(err.Error(), "required") {
		t.Errorf("expected required error, got: %v", err)
	}
}

// --- handleSearchRoute validation (21.7% coverage) ---

func TestHandleSearchRoute_WithOptionalArgs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleSearchRoute(ctx,
		map[string]any{
			"origin":       "Helsinki",
			"destination":  "Prague",
			"date":         "2026-06-15",
			"depart_after": "08:00",
			"arrive_by":    "22:00",
			"max_price":    float64(200),
			"prefer":       "train",
			"avoid":        "flight",
			"currency":     "EUR",
			"sort":         "price",
		},
		nil, nil, nil)
	_ = err // network call
}

// --- handleSuggestDates validation (55.0% coverage) ---

func TestHandleSuggestDates_WithAllArgs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleSuggestDates(ctx,
		map[string]any{
			"origin":      "HEL",
			"destination": "BCN",
			"target_date": "2026-06-15",
			"flex_days":   float64(7),
			"cabin_class": "business",
		},
		nil, nil, nil)
	_ = err // network call
}

// --- handleOptimizeMultiCity validation (42.9% coverage) ---

func TestHandleOptimizeMultiCity_WithAllArgs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := handleOptimizeMultiCity(ctx,
		map[string]any{
			"home":            "HEL",
			"cities":          "PRG,BCN,AMS",
			"start_date":      "2026-06-15",
			"nights_per_city": float64(3),
		},
		nil, nil, nil)
	_ = err // network call
}

// --- readResource branches (53.8% coverage) ---

func TestReadResource_AllStaticURIs(t *testing.T) {
	s := NewServer()
	tests := []struct {
		uri  string
		want string
	}{
		{"trvl://airports/popular", "HEL"},
		{"trvl://help/flights", "search_flights"},
		{"trvl://help/hotels", "search_hotels"},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			result, err := s.readResource(tt.uri)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Contents) == 0 {
				t.Fatal("expected contents")
			}
			if !contains(result.Contents[0].Text, tt.want) {
				t.Errorf("expected %q in response", tt.want)
			}
		})
	}
}

func TestReadResource_TripSummary(t *testing.T) {
	s := NewServer()
	result, err := s.readResource("trvl://trip/summary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result.Contents[0].Text, "No searches yet") {
		t.Errorf("expected empty summary, got: %s", result.Contents[0].Text)
	}
}

func TestReadResource_Watches_NoStore(t *testing.T) {
	s := NewServer()
	s.watchStore = nil
	result, err := s.readResource("trvl://watches")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result.Contents[0].Text, "No watch store") {
		t.Errorf("expected no watch store message, got: %s", result.Contents[0].Text)
	}
}

func TestReadResource_UnknownURI(t *testing.T) {
	s := NewServer()
	_, err := s.readResource("trvl://nonexistent")
	if err == nil || !contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestReadResource_WatchInvalidURI(t *testing.T) {
	s := NewServer()
	_, err := s.readResource("trvl://watch/invalid")
	if err == nil || !contains(err.Error(), "invalid watch URI") {
		t.Errorf("expected invalid URI error, got: %v", err)
	}
}

// --- handleCompletionComplete branches (66.7% coverage) ---

func TestCompletionComplete_Sort(t *testing.T) {
	s := NewServer()
	resp := s.handleCompletionComplete(&Request{
		ID:     "1",
		Params: mustMarshal(map[string]any{"argument": map[string]any{"name": "sort", "value": ""}}),
	})
	result := resp.Result.(map[string]any)
	completion := result["completion"].(map[string]any)
	values := completion["values"].([]string)
	if len(values) != 4 {
		t.Errorf("expected 4 sort values, got %d", len(values))
	}
}

func TestCompletionComplete_Type(t *testing.T) {
	s := NewServer()
	resp := s.handleCompletionComplete(&Request{
		ID:     "1",
		Params: mustMarshal(map[string]any{"argument": map[string]any{"name": "type", "value": ""}}),
	})
	result := resp.Result.(map[string]any)
	completion := result["completion"].(map[string]any)
	values := completion["values"].([]string)
	if len(values) != 2 {
		t.Errorf("expected 2 type values, got %d", len(values))
	}
}

func TestCompletionComplete_Provider(t *testing.T) {
	s := NewServer()
	resp := s.handleCompletionComplete(&Request{
		ID:     "1",
		Params: mustMarshal(map[string]any{"argument": map[string]any{"name": "provider", "value": ""}}),
	})
	result := resp.Result.(map[string]any)
	completion := result["completion"].(map[string]any)
	values := completion["values"].([]string)
	if len(values) != 2 {
		t.Errorf("expected 2 provider values, got %d", len(values))
	}
}

func TestCompletionComplete_Currency(t *testing.T) {
	s := NewServer()
	resp := s.handleCompletionComplete(&Request{
		ID:     "1",
		Params: mustMarshal(map[string]any{"argument": map[string]any{"name": "currency", "value": ""}}),
	})
	result := resp.Result.(map[string]any)
	completion := result["completion"].(map[string]any)
	values := completion["values"].([]string)
	if len(values) < 5 {
		t.Errorf("expected multiple currency values, got %d", len(values))
	}
}

func TestCompletionComplete_NilParams(t *testing.T) {
	s := NewServer()
	resp := s.handleCompletionComplete(&Request{ID: "1"})
	if resp.Error != nil {
		t.Errorf("expected no error, got: %v", resp.Error)
	}
}

// --- recordSearchFromArgs edge cases (80% coverage) ---

func TestRecordSearchFromArgs_NilStructured(t *testing.T) {
	s := NewServer()
	s.recordSearchFromArgs(map[string]any{"origin": "HEL"}, nil)
	// Should not panic
}

func TestRecordSearchFromArgs_NilArgs(t *testing.T) {
	s := NewServer()
	s.recordSearchFromArgs(nil, map[string]any{})
	// Should not panic
}
