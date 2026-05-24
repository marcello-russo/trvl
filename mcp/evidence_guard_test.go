package mcp

import (
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

// TestFlightSummaryNoLieOnPartial is the MIK-4950 evidence guard: when a
// provider timed out and zero flights came back, the summary must NOT claim
// "No flights found" — it must disclose the incomplete coverage instead.
func TestFlightSummaryNoLieOnPartial(t *testing.T) {
	partial := &models.FlightSearchResult{
		Success: true,
		Count:   0,
		Flights: nil,
		ProviderStatuses: []models.ProviderStatus{
			{ID: "google_flights", Status: models.StatusTimeout},
			{ID: "kiwi", Status: models.StatusTimeout},
		},
		Completeness: models.ComputeCompleteness([]models.ProviderStatus{
			{ID: "google_flights", Status: models.StatusTimeout},
			{ID: "kiwi", Status: models.StatusTimeout},
		}),
	}
	got := flightSummary(partial, "HEL", "CDG")
	if strings.Contains(got, "No flights found") {
		t.Errorf("partial search lied with definitive no-results: %q", got)
	}
	if !strings.Contains(got, "incomplete") {
		t.Errorf("partial search omitted the incompleteness caveat: %q", got)
	}

	// Contrast: when every provider was reached and returned nothing, the
	// definitive message is allowed.
	complete := &models.FlightSearchResult{
		Success: true,
		Count:   0,
		ProviderStatuses: []models.ProviderStatus{
			{ID: "google_flights", Status: models.StatusCheckedNoHit},
		},
		Completeness: models.ComputeCompleteness([]models.ProviderStatus{
			{ID: "google_flights", Status: models.StatusCheckedNoHit},
		}),
	}
	if got := flightSummary(complete, "HEL", "CDG"); !strings.Contains(got, "No flights found") {
		t.Errorf("complete-but-empty search should give definitive message, got %q", got)
	}
}
