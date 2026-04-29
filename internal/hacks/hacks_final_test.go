package hacks

import (
	"context"
	"testing"
	"time"
)

func hasSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ============================================================
// buildStopoverHack — direct test
// ============================================================

func TestDetectDateFlex_EmptyOrigin(t *testing.T) {
	got := detectDateFlex(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectDateFlex_EmptyDate(t *testing.T) {
	got := detectDateFlex(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectDateFlex_EmptyDestination(t *testing.T) {
	got := detectDateFlex(context.Background(), DetectorInput{Origin: "HEL", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty destination")
	}
}

func TestDetectCurrencyArbitrage_EmptyOrigin(t *testing.T) {
	got := detectCurrencyArbitrage(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectCurrencyArbitrage_EmptyDate(t *testing.T) {
	got := detectCurrencyArbitrage(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectHiddenCity_EmptyOrigin(t *testing.T) {
	got := detectHiddenCity(context.Background(), DetectorInput{Destination: "AMS", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectHiddenCity_EmptyDate(t *testing.T) {
	got := detectHiddenCity(context.Background(), DetectorInput{Origin: "HEL", Destination: "AMS"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectHiddenCity_UnknownDestination(t *testing.T) {
	// Destination not in hiddenCityExtensions.
	got := detectHiddenCity(context.Background(), DetectorInput{Origin: "HEL", Destination: "TLL", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for destination not in hiddenCityExtensions")
	}
}

func TestDetectLowCostCarrier_EmptyOrigin(t *testing.T) {
	got := detectLowCostCarrier(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectLowCostCarrier_EmptyDate(t *testing.T) {
	got := detectLowCostCarrier(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectMultiStop_EmptyOrigin(t *testing.T) {
	got := detectMultiStop(context.Background(), DetectorInput{Destination: "PRG", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectMultiStop_EmptyDate(t *testing.T) {
	got := detectMultiStop(context.Background(), DetectorInput{Origin: "HEL", Destination: "PRG"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectMultiStop_UnknownDestination(t *testing.T) {
	got := detectMultiStop(context.Background(), DetectorInput{Origin: "HEL", Destination: "TLL", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for destination not in multistopHubs")
	}
}

func TestDetectSplit_EmptyOrigin(t *testing.T) {
	got := detectSplit(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01", ReturnDate: "2026-07-05"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectSplit_EmptyDate(t *testing.T) {
	got := detectSplit(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN", ReturnDate: "2026-07-05"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectSplit_EmptyReturnDate(t *testing.T) {
	got := detectSplit(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty return date")
	}
}

func TestDetectStopover_EmptyOrigin(t *testing.T) {
	got := detectStopover(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectStopover_EmptyDate(t *testing.T) {
	got := detectStopover(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectOpenJaw_EmptyOrigin(t *testing.T) {
	got := detectOpenJaw(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01", ReturnDate: "2026-07-05"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectOpenJaw_EmptyDate(t *testing.T) {
	got := detectOpenJaw(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN", ReturnDate: "2026-07-05"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectOpenJaw_EmptyReturnDate(t *testing.T) {
	got := detectOpenJaw(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty return date")
	}
}

func TestDetectPositioning_EmptyOrigin(t *testing.T) {
	got := detectPositioning(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectPositioning_EmptyDate(t *testing.T) {
	got := detectPositioning(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectPositioning_UnknownOrigin(t *testing.T) {
	got := detectPositioning(context.Background(), DetectorInput{Origin: "TLL", Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for origin not in nearbyAirports")
	}
}

func TestDetectThrowaway_EmptyOrigin(t *testing.T) {
	got := detectThrowaway(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectThrowaway_EmptyDate(t *testing.T) {
	got := detectThrowaway(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectNightTransport_EmptyOrigin(t *testing.T) {
	got := detectNightTransport(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectNightTransport_EmptyDate(t *testing.T) {
	got := detectNightTransport(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectFerryPositioning_EmptyOrigin(t *testing.T) {
	got := detectFerryPositioning(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectFerryPositioning_EmptyDate(t *testing.T) {
	got := detectFerryPositioning(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectFerryPositioning_UnknownOrigin(t *testing.T) {
	got := detectFerryPositioning(context.Background(), DetectorInput{Origin: "BCN", Destination: "MAD", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for origin not in ferryPositioningRoutes")
	}
}

func TestDetectTuesdayBooking_EmptyOrigin(t *testing.T) {
	got := detectTuesdayBooking(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-03"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectTuesdayBooking_EmptyDate(t *testing.T) {
	got := detectTuesdayBooking(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectTuesdayBooking_InvalidDateFormat(t *testing.T) {
	got := detectTuesdayBooking(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN", Date: "not-a-date"})
	if got != nil {
		t.Error("expected nil for invalid date")
	}
}

func TestDetectTuesdayBooking_CheapDay(t *testing.T) {
	// A Tuesday is a cheap day, not expensive — should return nil.
	// Find a Tuesday.
	d := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC) // 2026-07-07 is Tuesday
	got := detectTuesdayBooking(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
		Date:        d.Format("2006-01-02"),
	})
	if got != nil {
		t.Error("expected nil for Tuesday departure (cheap day)")
	}
}

func TestDetectMultiModalSkipFlight_EmptyOrigin(t *testing.T) {
	got := detectMultiModalSkipFlight(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectMultiModalSkipFlight_EmptyDate(t *testing.T) {
	got := detectMultiModalSkipFlight(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectMultiModalPositioning_EmptyOrigin(t *testing.T) {
	got := detectMultiModalPositioning(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectMultiModalPositioning_EmptyDate(t *testing.T) {
	got := detectMultiModalPositioning(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectMultiModalPositioning_UnknownOrigin(t *testing.T) {
	got := detectMultiModalPositioning(context.Background(), DetectorInput{Origin: "BCN", Destination: "MAD", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for origin not in multiModalHubs")
	}
}

func TestDetectMultiModalOpenJawGround_EmptyOrigin(t *testing.T) {
	got := detectMultiModalOpenJawGround(context.Background(), DetectorInput{Destination: "DBV", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectMultiModalOpenJawGround_EmptyDate(t *testing.T) {
	got := detectMultiModalOpenJawGround(context.Background(), DetectorInput{Origin: "HEL", Destination: "DBV"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectMultiModalOpenJawGround_UnknownDestination(t *testing.T) {
	got := detectMultiModalOpenJawGround(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for destination not in nearbyHubs")
	}
}

func TestDetectMultiModalReturnSplit_EmptyOrigin(t *testing.T) {
	got := detectMultiModalReturnSplit(context.Background(), DetectorInput{Destination: "BCN", Date: "2026-07-01", ReturnDate: "2026-07-05"})
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectMultiModalReturnSplit_EmptyDate(t *testing.T) {
	got := detectMultiModalReturnSplit(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN", ReturnDate: "2026-07-05"})
	if got != nil {
		t.Error("expected nil for empty date")
	}
}

func TestDetectMultiModalReturnSplit_EmptyReturnDate(t *testing.T) {
	got := detectMultiModalReturnSplit(context.Background(), DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-07-01"})
	if got != nil {
		t.Error("expected nil for empty return date")
	}
}

// ============================================================
// DetectRailFlyArbitrage validation guards
// ============================================================

func TestDetectRailFlyArbitrage_EmptyOrigin(t *testing.T) {
	got := DetectRailFlyArbitrage(context.Background(), "", "BCN", "2026-07-01", "")
	if got != nil {
		t.Error("expected nil for empty origin")
	}
}

func TestDetectRailFlyArbitrage_EmptyDestination(t *testing.T) {
	got := DetectRailFlyArbitrage(context.Background(), "AMS", "", "2026-07-01", "")
	if got != nil {
		t.Error("expected nil for empty destination")
	}
}

func TestDetectRailFlyArbitrage_EmptyDepartDate(t *testing.T) {
	got := DetectRailFlyArbitrage(context.Background(), "AMS", "BCN", "", "")
	if got != nil {
		t.Error("expected nil for empty depart date")
	}
}

func TestDetectRailFlyArbitrage_NoStationsForHub(t *testing.T) {
	// TLL is not a hub for any rail-fly station.
	got := DetectRailFlyArbitrage(context.Background(), "TLL", "BCN", "2026-07-01", "")
	if got != nil {
		t.Error("expected nil for origin without rail-fly stations")
	}
}

func TestRailFlyStationsForHub_CDG_Coverage(t *testing.T) {
	stations := railFlyStationsForHub("CDG")
	if len(stations) == 0 {
		t.Error("expected stations for CDG hub")
	}
	for _, st := range stations {
		if st.HubIATA != "CDG" {
			t.Errorf("station %s has hub %s, want CDG", st.City, st.HubIATA)
		}
	}
}

func TestRailFlyStationsForHub_ZRH_Coverage(t *testing.T) {
	stations := railFlyStationsForHub("ZRH")
	if len(stations) == 0 {
		t.Error("expected stations for ZRH hub")
	}
}
