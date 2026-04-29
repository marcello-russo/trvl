package hacks

import (
	"context"
	"testing"
)

func TestDestinationAlternatives_knownDest(t *testing.T) {
	// BCN has known destination alternatives in the static map.
	alts := DestinationAlternatives("BCN")
	if len(alts) == 0 {
		t.Fatal("expected alternatives for BCN, got none")
	}
	for _, a := range alts {
		if a.IATA == "" {
			t.Error("alternative IATA must not be empty")
		}
		if a.City == "" {
			t.Errorf("alternative %s must have a city", a.IATA)
		}
	}
}

func TestDestinationAlternatives_unknownDest(t *testing.T) {
	alts := DestinationAlternatives("XXX")
	if alts != nil {
		t.Errorf("expected nil for unknown dest, got %d alternatives", len(alts))
	}
}

func TestDestinationAlternatives_fieldsPopulated(t *testing.T) {
	// Test a destination known to have alternatives.
	for _, dest := range []string{"BCN", "LHR", "CDG", "FCO"} {
		alts := DestinationAlternatives(dest)
		if len(alts) == 0 {
			continue // not all may be in the map
		}
		for _, a := range alts {
			if a.Mode == "" {
				t.Errorf("DestinationAlternatives(%s): %s has empty Mode", dest, a.IATA)
			}
		}
	}
}

// --- accessors.go: RailFlyStationsForHub (0% coverage) ---

func TestRailFlyStationsForHub_known(t *testing.T) {
	stations := RailFlyStationsForHub("AMS")
	if len(stations) == 0 {
		t.Fatal("expected rail+fly stations for AMS hub")
	}
	for _, s := range stations {
		if s.IATA == "" || s.City == "" {
			t.Errorf("station must have IATA and City; got %+v", s)
		}
		if s.HubIATA != "AMS" {
			t.Errorf("expected HubIATA=AMS, got %s", s.HubIATA)
		}
	}
}

func TestRailFlyStationsForHub_FRA_fields(t *testing.T) {
	stations := RailFlyStationsForHub("FRA")
	if len(stations) == 0 {
		t.Fatal("expected rail+fly stations for FRA hub")
	}
	for _, s := range stations {
		if s.TrainMins <= 0 {
			t.Errorf("station %s should have positive TrainMins", s.IATA)
		}
		if s.AirlineName == "" {
			t.Errorf("station %s should have AirlineName", s.IATA)
		}
		if s.FareZone == "" {
			t.Errorf("station %s should have FareZone", s.IATA)
		}
	}
}

func TestRailFlyStationsForHub_unknownHub(t *testing.T) {
	stations := RailFlyStationsForHub("XXX")
	if len(stations) != 0 {
		t.Errorf("expected no stations for unknown hub, got %d", len(stations))
	}
}

func TestRailFlyStationsForHub_CDG_accessor(t *testing.T) {
	stations := RailFlyStationsForHub("CDG")
	if len(stations) == 0 {
		t.Fatal("expected rail+fly stations for CDG hub")
	}
}

func TestRailFlyStationsForHub_ZRH_accessor(t *testing.T) {
	stations := RailFlyStationsForHub("ZRH")
	if len(stations) == 0 {
		t.Fatal("expected rail+fly stations for ZRH hub")
	}
}

// --- detectCurrencyArbitrage: input validation (17% -> higher) ---

func TestDetectCurrencyArbitrage_noDate(t *testing.T) {
	h := detectCurrencyArbitrage(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectCurrencyArbitrage_noOrigin(t *testing.T) {
	h := detectCurrencyArbitrage(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectCurrencyArbitrage_noDestination(t *testing.T) {
	h := detectCurrencyArbitrage(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectDateFlex: input validation (12% -> higher) ---

func TestDetectDateFlex_noDate(t *testing.T) {
	h := detectDateFlex(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectDateFlex_noOrigin(t *testing.T) {
	h := detectDateFlex(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectDateFlex_noDestination(t *testing.T) {
	h := detectDateFlex(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectFerryPositioning: input validation (18% -> higher) ---

func TestDetectFerryPositioning_noDate(t *testing.T) {
	h := detectFerryPositioning(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectFerryPositioning_noOrigin(t *testing.T) {
	h := detectFerryPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectFerryPositioning_noDestination(t *testing.T) {
	h := detectFerryPositioning(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectFerryPositioning_unknownOrigin(t *testing.T) {
	h := detectFerryPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "XXX",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for origin with no ferry routes")
	}
}

// --- detectHiddenCity: input validation (13% -> higher) ---

func TestDetectHiddenCity_noDate(t *testing.T) {
	h := detectHiddenCity(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "AMS",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectHiddenCity_noOrigin(t *testing.T) {
	h := detectHiddenCity(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "AMS",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectHiddenCity_noDestination(t *testing.T) {
	h := detectHiddenCity(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectHiddenCity_unknownDest(t *testing.T) {
	h := detectHiddenCity(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "XXX",
	})
	if h != nil {
		t.Error("expected nil for destination without hidden city extensions")
	}
}

// --- detectLowCostCarrier: input validation (20% -> higher) ---

func TestDetectLowCostCarrier_noDate(t *testing.T) {
	h := detectLowCostCarrier(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectLowCostCarrier_noOrigin(t *testing.T) {
	h := detectLowCostCarrier(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectLowCostCarrier_noDestination(t *testing.T) {
	h := detectLowCostCarrier(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectMultiStop: input validation (21% -> higher) ---

func TestDetectMultiStop_noDate(t *testing.T) {
	h := detectMultiStop(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectMultiStop_noOrigin(t *testing.T) {
	h := detectMultiStop(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectMultiStop_noDestination(t *testing.T) {
	h := detectMultiStop(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectMultiStop_unknownDest(t *testing.T) {
	h := detectMultiStop(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "XXX",
	})
	if h != nil {
		t.Error("expected nil for destination without multistop hubs")
	}
}

// --- detectMultiModalOpenJawGround: input validation (9% -> higher) ---

func TestDetectMultiModalOpenJawGround_noDate(t *testing.T) {
	h := detectMultiModalOpenJawGround(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "DBV",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectMultiModalOpenJawGround_noOrigin(t *testing.T) {
	h := detectMultiModalOpenJawGround(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "DBV",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectMultiModalOpenJawGround_noDestination(t *testing.T) {
	h := detectMultiModalOpenJawGround(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectMultiModalPositioning: input validation (20% -> higher) ---

func TestDetectMultiModalPositioning_noDate(t *testing.T) {
	h := detectMultiModalPositioning(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectMultiModalPositioning_noOrigin(t *testing.T) {
	h := detectMultiModalPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectMultiModalPositioning_noDestination(t *testing.T) {
	h := detectMultiModalPositioning(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectMultiModalReturnSplit: input validation (9% -> higher) ---

func TestDetectMultiModalReturnSplit_noDate(t *testing.T) {
	h := detectMultiModalReturnSplit(context.Background(), DetectorInput{
		ReturnDate:  "2026-06-22",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectMultiModalReturnSplit_noOrigin(t *testing.T) {
	h := detectMultiModalReturnSplit(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		ReturnDate:  "2026-06-22",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectMultiModalReturnSplit_noDestination(t *testing.T) {
	h := detectMultiModalReturnSplit(context.Background(), DetectorInput{
		Date:       "2026-06-15",
		ReturnDate: "2026-06-22",
		Origin:     "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectMultiModalReturnSplit_noReturnDate(t *testing.T) {
	h := detectMultiModalReturnSplit(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing return date")
	}
}

// --- detectMultiModalSkipFlight: input validation (13% -> higher) ---

func TestDetectMultiModalSkipFlight_noDestination(t *testing.T) {
	h := detectMultiModalSkipFlight(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectNightTransport: input validation (35% -> higher) ---

func TestDetectNightTransport_noDate(t *testing.T) {
	h := detectNightTransport(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectNightTransport_noOrigin(t *testing.T) {
	h := detectNightTransport(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectNightTransport_noDestination(t *testing.T) {
	h := detectNightTransport(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectOpenJaw: input validation (14% -> higher) ---

func TestDetectOpenJaw_noDate(t *testing.T) {
	h := detectOpenJaw(context.Background(), DetectorInput{
		ReturnDate:  "2026-06-22",
		Origin:      "HEL",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectOpenJaw_noOrigin(t *testing.T) {
	h := detectOpenJaw(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		ReturnDate:  "2026-06-22",
		Destination: "PRG",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectOpenJaw_noDestination(t *testing.T) {
	h := detectOpenJaw(context.Background(), DetectorInput{
		Date:       "2026-06-15",
		ReturnDate: "2026-06-22",
		Origin:     "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

// --- detectPositioning: input validation (26% -> higher) ---

func TestDetectPositioning_noDate(t *testing.T) {
	h := detectPositioning(context.Background(), DetectorInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}

func TestDetectPositioning_noOrigin(t *testing.T) {
	h := detectPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing origin")
	}
}

func TestDetectPositioning_noDestination(t *testing.T) {
	h := detectPositioning(context.Background(), DetectorInput{
		Date:   "2026-06-15",
		Origin: "HEL",
	})
	if h != nil {
		t.Error("expected nil for missing destination")
	}
}

func TestDetectPositioning_unknownOrigin(t *testing.T) {
	h := detectPositioning(context.Background(), DetectorInput{
		Date:        "2026-06-15",
		Origin:      "XXX",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for origin without nearby airports")
	}
}

// --- detectSplit: input validation (19% -> higher) ---

func TestDetectSplit_noDate(t *testing.T) {
	h := detectSplit(context.Background(), DetectorInput{
		ReturnDate:  "2026-06-22",
		Origin:      "HEL",
		Destination: "BCN",
	})
	if h != nil {
		t.Error("expected nil for missing date")
	}
}
