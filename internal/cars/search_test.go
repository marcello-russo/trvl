package cars

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearch_RequiresPickupAndDates(t *testing.T) {
	t.Parallel()

	_, err := Search(context.Background(), SearchOptions{
		PickupLocation: "HEL",
		PickupDate:     "2026-07-01",
	})
	if err == nil {
		t.Fatal("expected missing dropoff_date error")
	}
}

func TestSearch_NoConfiguredProviderReturnsTypedStatus(t *testing.T) {
	t.Setenv(skyscannerAPIKeyEnv, "")

	result, err := Search(context.Background(), SearchOptions{
		PickupLocation: "HEL",
		PickupDate:     "2026-07-01",
		DropoffDate:    "2026-07-04",
		Currency:       "EUR",
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if result.Success {
		t.Fatal("expected no-provider result to be unsuccessful")
	}
	if result.Count != 0 || len(result.Offers) != 0 {
		t.Fatalf("offers = %d/%d, want none", result.Count, len(result.Offers))
	}
	if len(result.ProviderStatuses) != 1 {
		t.Fatalf("provider statuses = %d, want 1", len(result.ProviderStatuses))
	}
	status := result.ProviderStatuses[0]
	if status.ID != ProviderSkyscanner || status.Status != "skipped" {
		t.Fatalf("status = %#v, want skipped skyscanner", status)
	}
	if status.FixHintCode != "MISSING_CREDENTIAL" {
		t.Fatalf("fix hint code = %q, want MISSING_CREDENTIAL", status.FixHintCode)
	}
	if !strings.Contains(result.Error, "SKYSCANNER_API_KEY") {
		t.Fatalf("error %q should mention SKYSCANNER_API_KEY", result.Error)
	}
}

func TestSearch_SkyscannerFixtureNormalizesOffers(t *testing.T) {
	t.Setenv(skyscannerAPIKeyEnv, "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Fatalf("x-api-key = %q, want test-key", got)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/search/create"):
			_ = json.NewEncoder(w).Encode(map[string]any{"sessionToken": "session-123"})
		case strings.HasSuffix(r.URL.Path, "/search/poll/session-123"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "complete",
				"offers": []map[string]any{
					{
						"provider":       "Skyscanner",
						"supplier":       "Hertz",
						"vehicleClass":   "compact",
						"vehicleName":    "Volkswagen Golf or similar",
						"transmission":   "automatic",
						"fuelPolicy":     "full_to_full",
						"seats":          5,
						"bags":           2,
						"doors":          4,
						"bookingUrl":     "https://example.test/book",
						"freeCancel":     true,
						"unlimitedMiles": true,
						"price": map[string]any{
							"amount":   144.50,
							"currency": "EUR",
						},
					},
					{
						"supplier": "Too Expensive",
						"price":    map[string]any{"amount": 999.0, "currency": "EUR"},
					},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	restore := setSkyscannerBaseURLForTest(server.URL)
	defer restore()

	result, err := Search(context.Background(), SearchOptions{
		PickupLocation:  "Helsinki Airport",
		DropoffLocation: "Helsinki Airport",
		PickupDate:      "2026-07-01",
		DropoffDate:     "2026-07-04",
		PickupTime:      "09:00",
		DropoffTime:     "18:00",
		Currency:        "EUR",
		Passengers:      3,
		MaxPrice:        200,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if !result.Success || result.Count != 1 {
		t.Fatalf("success/count = %v/%d, want true/1: %#v", result.Success, result.Count, result)
	}
	offer := result.Offers[0]
	if offer.Supplier != "Hertz" || offer.VehicleClass != "compact" || offer.Price != 144.50 {
		t.Fatalf("normalized offer = %#v", offer)
	}
	if offer.Currency != "EUR" || offer.Passengers != 3 {
		t.Fatalf("currency/passengers = %s/%d, want EUR/3", offer.Currency, offer.Passengers)
	}
	if offer.Pickup.Location != "Helsinki Airport" || offer.Dropoff.Location != "Helsinki Airport" {
		t.Fatalf("locations not preserved: %#v %#v", offer.Pickup, offer.Dropoff)
	}
	if len(result.ProviderStatuses) != 1 || result.ProviderStatuses[0].Status != "ok" {
		t.Fatalf("provider statuses = %#v, want ok", result.ProviderStatuses)
	}
}
