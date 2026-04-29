package hacks

import (
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/preferences"
)

func TestRailFlyStationsForHub_NoHub_BCN(t *testing.T) {
	stations := railFlyStationsForHub("BCN")
	if len(stations) != 0 {
		t.Errorf("expected no stations for BCN, got %d", len(stations))
	}
}

// ============================================================
// Helper functions — edge cases
// ============================================================

func TestRoutesThrough_NoLegs(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Price: 100, Legs: nil},
		},
	}
	// No legs means single-leg = skipped, but len(Flights)>0 so optimistic return.
	got := routesThroughDestination(result, "AMS")
	if !got {
		t.Error("expected true (optimistic fallback)")
	}
}

func TestRoutesThrough_OnlyOneFlightOneLeg(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Price: 100, Legs: []models.FlightLeg{{ArrivalAirport: models.AirportInfo{Code: "AMS"}}}},
		},
	}
	// Single-leg: cannot be hidden-city. Loop skips it, but optimistic fallback.
	got := routesThroughDestination(result, "AMS")
	if !got {
		t.Error("expected true (optimistic fallback, single-leg skipped)")
	}
}

func TestBuildRailFlyHack_LufthansaRisks(t *testing.T) {
	station := railFlyStation{
		IATA: "QKL", City: "Cologne", HubIATA: "FRA",
		Airline: "LH", AirlineName: "Lufthansa",
		TrainProvider: "DB ICE", TrainMinutes: 62,
		FareZone: "Rhineland regional",
	}
	h := buildRailFlyHack("FRA", "BCN", 200, "EUR", 150, "EUR", 50, station, "2026-07-08")
	if h.Type != "rail_fly_arbitrage" {
		t.Errorf("type = %q, want rail_fly_arbitrage", h.Type)
	}
	// Lufthansa should have OUTBOUND enforcement warning.
	found := false
	for _, r := range h.Risks {
		if len(r) > 8 && r[:8] == "OUTBOUND" {
			found = true
		}
	}
	if !found {
		t.Error("expected OUTBOUND risk for Lufthansa station")
	}
}

func TestBuildRailFlyHack_KLMSafe(t *testing.T) {
	station := railFlyStation{
		IATA: "ZWE", City: "Antwerp", HubIATA: "AMS",
		Airline: "KL", AirlineName: "KLM",
		TrainProvider: "Eurostar", TrainMinutes: 60,
		FareZone: "Belgian market",
	}
	h := buildRailFlyHack("AMS", "BCN", 200, "EUR", 150, "EUR", 50, station, "")
	// KLM should have LOW risk note.
	found := false
	for _, r := range h.Risks {
		if len(r) > 3 && r[:3] == "LOW" {
			found = true
		}
	}
	if !found {
		t.Error("expected LOW risk for KLM station")
	}
	// One-way trip type check.
	foundOneWay := false
	for _, s := range h.Steps {
		if len(s) > 0 {
			for _, w := range []string{"one-way"} {
				if hasSub(s, w) {
					foundOneWay = true
				}
			}
		}
	}
	if !foundOneWay {
		t.Error("expected 'one-way' in steps for empty return date")
	}
}

func TestBuildStopoverHack_CustomCurrency(t *testing.T) {
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-07-01", Currency: "USD"}
	prog := StopoverProgram{Airline: "Finnair", Hub: "HEL", MaxNights: 5, Restrictions: "Non-Finnish residents", URL: "https://finnair.com/stopover"}
	f := models.FlightResult{Price: 200, Currency: "SEK"}
	h := buildStopoverHack(in, prog, f, "HEL")
	if h.Currency != "SEK" {
		t.Errorf("currency = %q, want SEK (from flight)", h.Currency)
	}
}

func TestBuildStopoverHack_NoCurrency(t *testing.T) {
	in := DetectorInput{Origin: "HEL", Destination: "BCN", Date: "2026-07-01"}
	prog := StopoverProgram{Airline: "Finnair", Hub: "HEL", MaxNights: 5, Restrictions: "test", URL: "https://test.com"}
	f := models.FlightResult{Price: 200} // no currency
	h := buildStopoverHack(in, prog, f, "HEL")
	if h.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR (fallback)", h.Currency)
	}
}

// ============================================================
// isOvernightRoute — additional edge cases
// ============================================================

func TestIsOvernightRoute_ShortHHMM_Night(t *testing.T) {
	if !isOvernightRoute("21:30", "06:00") {
		t.Error("expected overnight for 21:30 departure")
	}
}

func TestIsOvernightRoute_ShortHHMM_Day(t *testing.T) {
	if isOvernightRoute("10:00", "14:00") {
		t.Error("did not expect overnight for 10:00 departure")
	}
}

func TestIsOvernightRoute_ShortHHMM_EarlyMorning(t *testing.T) {
	if !isOvernightRoute("01:30", "08:00") {
		t.Error("expected overnight for 01:30 departure")
	}
}

func TestIsOvernightRoute_InvalidStrings(t *testing.T) {
	if isOvernightRoute("invalid", "also-invalid") {
		t.Error("did not expect overnight for invalid strings")
	}
}

func TestIsOvernightRoute_FullISO_NightToMorning(t *testing.T) {
	if !isOvernightRoute("2026-07-01T21:00", "2026-07-02T07:00") {
		t.Error("expected overnight for 21:00→07:00 crossing day boundary")
	}
}

func TestIsOvernightRoute_FullISO_SameDay(t *testing.T) {
	if isOvernightRoute("2026-07-01T08:00", "2026-07-01T12:00") {
		t.Error("did not expect overnight for same-day morning")
	}
}

// ============================================================
// parseDatetime — edge cases
// ============================================================

func TestParseDatetime_WithSeconds(t *testing.T) {
	_, err := parseDatetime("2026-07-01T10:30:00+02:00")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseDatetime_WithoutTimezone(t *testing.T) {
	_, err := parseDatetime("2026-07-01T10:30:00")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseDatetime_ShortForm(t *testing.T) {
	_, err := parseDatetime("2026-07-01T10:30")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseDatetime_SpaceForm(t *testing.T) {
	_, err := parseDatetime("2026-07-01 10:30")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseDatetime_Invalid(t *testing.T) {
	_, err := parseDatetime("not-a-date")
	if err == nil {
		t.Error("expected error for invalid datetime")
	}
}

// ============================================================
// layoverMinutes — more cases
// ============================================================

func TestLayoverMinutes_NegativeDiff(t *testing.T) {
	got := layoverMinutes("2026-07-01T12:00", "2026-07-01T10:00")
	if got != 0 {
		t.Errorf("expected 0 for negative diff, got %d", got)
	}
}

func TestLayoverMinutes_ParseError(t *testing.T) {
	got := layoverMinutes("invalid", "2026-07-01T10:00")
	if got != 0 {
		t.Errorf("expected 0 for parse error, got %d", got)
	}
}

func TestLayoverMinutes_LongLayover(t *testing.T) {
	got := layoverMinutes("2026-07-01T08:00", "2026-07-01T16:00")
	if got != 480 {
		t.Errorf("expected 480 minutes, got %d", got)
	}
}

// ============================================================
// dateDelta — more cases
// ============================================================

func TestDateDelta_ValidDates(t *testing.T) {
	got := dateDelta("2026-07-01", "2026-07-04")
	if got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestDateDelta_InvalidBase(t *testing.T) {
	got := dateDelta("invalid", "2026-07-04")
	if got != 0 {
		t.Errorf("expected 0 for invalid base, got %d", got)
	}
}

func TestDateDelta_InvalidAlt(t *testing.T) {
	got := dateDelta("2026-07-01", "invalid")
	if got != 0 {
		t.Errorf("expected 0 for invalid alt, got %d", got)
	}
}

func TestDateDelta_NegativeDelta(t *testing.T) {
	got := dateDelta("2026-07-05", "2026-07-01")
	if got != -4 {
		t.Errorf("expected -4, got %d", got)
	}
}

// ============================================================
// isCheapDay — comprehensive
// ============================================================

func TestIsCheapDay_Wednesday(t *testing.T) {
	if !isCheapDay(time.Wednesday) {
		t.Error("Wednesday should be a cheap day")
	}
}

func TestIsCheapDay_Saturday(t *testing.T) {
	if !isCheapDay(time.Saturday) {
		t.Error("Saturday should be a cheap day")
	}
}

func TestIsCheapDay_Monday(t *testing.T) {
	if isCheapDay(time.Monday) {
		t.Error("Monday should not be a cheap day")
	}
}

func TestIsCheapDay_Thursday(t *testing.T) {
	if isCheapDay(time.Thursday) {
		t.Error("Thursday should not be a cheap day")
	}
}

func TestIsCheapDay_Sunday(t *testing.T) {
	if isCheapDay(time.Sunday) {
		t.Error("Sunday should not be a cheap day")
	}
}

// ============================================================
// cityFromCode — additional codes
// ============================================================

func TestCityFromCode_KnownCities(t *testing.T) {
	tests := map[string]string{
		"HEL": "Helsinki",
		"TLL": "Tallinn",
		"AMS": "Amsterdam",
		"CDG": "Paris",
		"LHR": "London",
		"PRG": "Prague",
		"BCN": "Barcelona",
		"IST": "Istanbul",
	}
	for code, want := range tests {
		if got := cityFromCode(code); got != want {
			t.Errorf("cityFromCode(%s) = %q, want %q", code, got, want)
		}
	}
}

func TestCityFromCode_Unknown(t *testing.T) {
	if got := cityFromCode("XYZ"); got != "XYZ" {
		t.Errorf("cityFromCode(XYZ) = %q, want XYZ (passthrough)", got)
	}
}

// ============================================================
// trimToHHMM — edge cases
// ============================================================

func TestTrimToHHMM_Short(t *testing.T) {
	got := trimToHHMM("10:30")
	if got != "10:30" {
		t.Errorf("trimToHHMM(10:30) = %q, want 10:30", got)
	}
}

func TestTrimToHHMM_LongISO(t *testing.T) {
	got := trimToHHMM("2026-07-01T14:45:00+02:00")
	if got != "14:45" {
		t.Errorf("trimToHHMM long = %q, want 14:45", got)
	}
}

func TestTrimToHHMM_ExactlyShort(t *testing.T) {
	got := trimToHHMM("2026-07-01T09:15")
	if got != "09:15" {
		t.Errorf("trimToHHMM exactly 16 = %q, want 09:15", got)
	}
}

// ============================================================
// hubCityName — additional airports
// ============================================================

func TestHubCityName_Known(t *testing.T) {
	tests := map[string]string{
		"HEL": "Helsinki",
		"KEF": "Reykjavik",
		"IST": "Istanbul",
		"DOH": "Doha",
		"DXB": "Dubai",
		"SIN": "Singapore",
	}
	for code, want := range tests {
		if got := hubCityName(code); got != want {
			t.Errorf("hubCityName(%s) = %q, want %q", code, got, want)
		}
	}
}

func TestHubCityName_Unknown(t *testing.T) {
	if got := hubCityName("XYZ"); got != "XYZ" {
		t.Errorf("hubCityName(XYZ) = %q, want XYZ", got)
	}
}

// ============================================================
// matchStopoverProgram — edge cases
// ============================================================

func TestMatchStopoverProgram_AllProgramsByHub(t *testing.T) {
	hubs := []string{"HEL", "KEF", "LIS", "IST", "DOH", "DXB", "SIN", "AUH"}
	for _, hub := range hubs {
		_, ok := matchStopoverProgram(hub, "XX")
		if !ok {
			t.Errorf("expected match for hub %s via hub-only lookup", hub)
		}
	}
}

// ============================================================
// loyaltyConflictNote — nil prefs
// ============================================================

func TestLoyaltyConflictNote_NilPrefs(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Legs: []models.FlightLeg{{AirlineCode: "LH", Airline: "Lufthansa"}}},
		},
	}
	got := loyaltyConflictNote(result, nil)
	if got != "" {
		t.Errorf("expected empty for nil prefs, got %q", got)
	}
}

func TestLoyaltyConflictNote_WithMatchingLoyalty(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{
			{Legs: []models.FlightLeg{{AirlineCode: "LH", Airline: "Lufthansa"}}},
		},
	}
	prefs := &preferences.Preferences{
		LoyaltyAirlines: []string{"LH"},
	}
	got := loyaltyConflictNote(result, prefs)
	if got == "" {
		t.Error("expected non-empty for matching loyalty airline")
	}
}

// ============================================================
// parseHour — edge cases
// ============================================================

func TestParseHour_FullISO(t *testing.T) {
	var h int
	_, err := parseHour("2026-07-01T14:30", &h)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if h != 14 {
		t.Errorf("hour = %d, want 14", h)
	}
}

func TestParseHour_ShortHHMM(t *testing.T) {
	var h int
	_, err := parseHour("08:45", &h)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if h != 8 {
		t.Errorf("hour = %d, want 8", h)
	}
}

func TestParseHour_Invalid(t *testing.T) {
	var h int
	_, err := parseHour("not-valid", &h)
	if err == nil {
		t.Error("expected error for invalid input")
	}
	if err.Error() != "cannot parse hour from: not-valid" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ============================================================
// addDays — edge cases
// ============================================================

func TestAddDays_InvalidDate(t *testing.T) {
	got := addDays("invalid", 3)
	if got != "" {
		t.Errorf("expected empty for invalid date, got %q", got)
	}
}

func TestAddDays_CrossMonthBoundary(t *testing.T) {
	got := addDays("2026-07-30", 3)
	if got != "2026-08-02" {
		t.Errorf("addDays(2026-07-30, 3) = %q, want 2026-08-02", got)
	}
}

// ============================================================
// adjustReturnDate
// ============================================================

func TestAdjustReturnDate_Empty(t *testing.T) {
	got := adjustReturnDate("", 3)
	if got != "" {
		t.Errorf("expected empty for empty return date, got %q", got)
	}
}

func TestAdjustReturnDate_Valid(t *testing.T) {
	got := adjustReturnDate("2026-07-10", -2)
	if got != "2026-07-08" {
		t.Errorf("adjustReturnDate = %q, want 2026-07-08", got)
	}
}

// ============================================================
// flightCurrency — edge cases
// ============================================================

func TestFlightCurrency_NilResult(t *testing.T) {
	got := flightCurrency(nil, "USD")
	if got != "USD" {
		t.Errorf("expected USD fallback, got %q", got)
	}
}

func TestFlightCurrency_EmptyFlights(t *testing.T) {
	result := &models.FlightSearchResult{Success: true, Flights: nil}
	got := flightCurrency(result, "USD")
	if got != "USD" {
		t.Errorf("expected USD fallback, got %q", got)
	}
}

func TestFlightCurrency_WithCurrency(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{{Currency: "SEK"}},
	}
	got := flightCurrency(result, "USD")
	if got != "SEK" {
		t.Errorf("expected SEK, got %q", got)
	}
}

func TestFlightCurrency_EmptyCurrencyInFlight(t *testing.T) {
	result := &models.FlightSearchResult{
		Success: true,
		Flights: []models.FlightResult{{Currency: ""}},
	}
	got := flightCurrency(result, "USD")
	if got != "USD" {
		t.Errorf("expected USD fallback, got %q", got)
	}
}

// ============================================================
// minFlightPrice — edge cases
// ============================================================

func TestMinFlightPrice_Nil(t *testing.T) {
	got := minFlightPrice(nil)
	if got != 0 {
		t.Errorf("expected 0, got %f", got)
	}
}
