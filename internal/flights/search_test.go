package flights

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/jsonutil"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// loadGoldenResponse loads and parses the golden test response file.
// The file simulates the inner JSON structure after DecodeFlightResponse
// has stripped the anti-XSSI prefix and parsed the outer envelope.
func loadGoldenResponse(t *testing.T) []any {
	t.Helper()
	data, err := os.ReadFile("testdata/flight_response.json")
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	// The golden file represents the outer response array.
	// DecodeFlightResponse would extract outer[0][2] as the inner JSON.
	// Our golden file is structured as: [ [null, null, flights_data] ]
	// So we parse the whole thing, then extract [0][2] to match what
	// ExtractFlightData expects.
	var outer []any
	if err := json.Unmarshal(data, &outer); err != nil {
		t.Fatalf("unmarshal golden file: %v", err)
	}

	first, ok := outer[0].([]any)
	if !ok || len(first) < 3 {
		t.Fatalf("golden file outer[0] invalid")
	}

	return first[2].([]any)
}

func TestParseFlights_GoldenFile(t *testing.T) {
	rawFlights := loadGoldenResponse(t)

	// rawFlights is the array of flight entries at the [2] position.
	flights := parseFlights(rawFlights)

	if len(flights) != 3 {
		t.Fatalf("expected 3 flights, got %d", len(flights))
	}

	// Flight 1: Direct HEL->NRT on Finnair
	f1 := flights[0]
	if f1.Price != 523 {
		t.Errorf("flight 1 price: got %v, want 523", f1.Price)
	}
	if f1.Currency != "EUR" {
		t.Errorf("flight 1 currency: got %q, want EUR", f1.Currency)
	}
	if f1.Duration != 780 {
		t.Errorf("flight 1 duration: got %d, want 780", f1.Duration)
	}
	if f1.Stops != 0 {
		t.Errorf("flight 1 stops: got %d, want 0", f1.Stops)
	}
	if len(f1.Legs) != 1 {
		t.Fatalf("flight 1 legs: got %d, want 1", len(f1.Legs))
	}

	leg := f1.Legs[0]
	if leg.DepartureAirport.Code != "HEL" {
		t.Errorf("leg dep airport: got %q, want HEL", leg.DepartureAirport.Code)
	}
	if leg.ArrivalAirport.Code != "NRT" {
		t.Errorf("leg arr airport: got %q, want NRT", leg.ArrivalAirport.Code)
	}
	if leg.Airline != "Finnair" {
		t.Errorf("leg airline: got %q, want Finnair", leg.Airline)
	}
	if leg.AirlineCode != "AY" {
		t.Errorf("leg airline code: got %q, want AY", leg.AirlineCode)
	}
	if leg.FlightNumber != "AY 79" {
		t.Errorf("leg flight number: got %q, want AY 79", leg.FlightNumber)
	}
	if leg.DepartureTime != "2026-06-15T10:30" {
		t.Errorf("leg dep time: got %q, want 2026-06-15T10:30", leg.DepartureTime)
	}
	if leg.ArrivalTime != "2026-06-16T07:15" {
		t.Errorf("leg arr time: got %q, want 2026-06-16T07:15", leg.ArrivalTime)
	}
	if leg.Duration != 780 {
		t.Errorf("leg duration: got %d, want 780", leg.Duration)
	}

	// Flight 2: 1-stop HEL->FRA->NRT on Lufthansa
	f2 := flights[1]
	if f2.Price != 487 {
		t.Errorf("flight 2 price: got %v, want 487", f2.Price)
	}
	if f2.Stops != 1 {
		t.Errorf("flight 2 stops: got %d, want 1", f2.Stops)
	}
	if len(f2.Legs) != 2 {
		t.Fatalf("flight 2 legs: got %d, want 2", len(f2.Legs))
	}
	if f2.Legs[0].ArrivalAirport.Code != "FRA" {
		t.Errorf("flight 2 leg 0 arr: got %q, want FRA", f2.Legs[0].ArrivalAirport.Code)
	}
	if f2.Legs[1].DepartureAirport.Code != "FRA" {
		t.Errorf("flight 2 leg 1 dep: got %q, want FRA", f2.Legs[1].DepartureAirport.Code)
	}
	if f2.Duration != 1470 {
		t.Errorf("flight 2 duration: got %d, want 1470", f2.Duration)
	}

	// Flight 3: Direct on JAL
	f3 := flights[2]
	if f3.Price != 612 {
		t.Errorf("flight 3 price: got %v, want 612", f3.Price)
	}
	if f3.Legs[0].Airline != "Japan Airlines" {
		t.Errorf("flight 3 airline: got %q, want Japan Airlines", f3.Legs[0].Airline)
	}
}

func TestParseFlights_EmptyInput(t *testing.T) {
	flights := parseFlights(nil)
	if flights != nil {
		t.Errorf("expected nil for nil input, got %d flights", len(flights))
	}

	flights = parseFlights([]any{})
	if flights != nil {
		t.Errorf("expected nil for empty input, got %d flights", len(flights))
	}
}

func TestParseFlights_MalformedEntries(t *testing.T) {
	// Should skip entries that aren't arrays or are too short
	malformed := []any{
		"not an array",
		42,
		nil,
		[]any{}, // too short (< 2 elements)
		[]any{"only one element"},
	}

	flights := parseFlights(malformed)
	if len(flights) != 0 {
		t.Errorf("expected 0 flights from malformed input, got %d", len(flights))
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"valid", []any{float64(2026), float64(6), float64(15), float64(10), float64(30)}, "2026-06-15T10:30"},
		{"midnight", []any{float64(2026), float64(1), float64(1), float64(0), float64(0)}, "2026-01-01T00:00"},
		{"nil", nil, ""},
		{"short array", []any{float64(2026), float64(6)}, ""},
		{"not array", "2026-06-15", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTime(tt.in)
			if got != tt.want {
				t.Errorf("formatTime(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
	}
	for _, tt := range tests {
		got := toString(tt.in)
		if got != tt.want {
			t.Errorf("toString(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		in   any
		want int
	}{
		{nil, 0},
		{float64(42), 42},
		{float64(0), 0},
		{"not a number", 0},
	}
	for _, tt := range tests {
		got := jsonutil.ToInt(tt.in)
		if got != tt.want {
			t.Errorf("toInt(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestBuildFilters_OneWay(t *testing.T) {
	opts := SearchOptions{Adults: 1, CabinClass: models.Economy}
	filters := buildFilters("HEL", "NRT", "2026-06-15", opts)

	// Verify it produces a marshalable structure
	data, err := json.Marshal(filters)
	if err != nil {
		t.Fatalf("marshal filters: %v", err)
	}

	// Parse back and verify structure
	var arr []any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(arr) != 6 {
		t.Fatalf("expected 6 top-level elements, got %d", len(arr))
	}

	// arr[1] is settings
	settings, ok := arr[1].([]any)
	if !ok {
		t.Fatalf("arr[1] not array")
	}

	// Trip type should be 2 (one-way)
	if tripType, ok := settings[2].(float64); !ok || int(tripType) != 2 {
		t.Errorf("trip type: got %v, want 2", settings[2])
	}

	// Cabin class should be 1 (economy)
	if cabin, ok := settings[5].(float64); !ok || int(cabin) != 1 {
		t.Errorf("cabin class: got %v, want 1", settings[5])
	}
}

func TestBuildFilters_RoundTrip(t *testing.T) {
	opts := SearchOptions{
		Adults:     2,
		CabinClass: models.Business,
		ReturnDate: "2026-06-20",
		SortBy:     models.SortCheapest,
	}
	filters := buildFilters("HEL", "NRT", "2026-06-15", opts)

	data, err := json.Marshal(filters)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var arr []any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	settings := arr[1].([]any)

	// Trip type should be 1 (round-trip)
	if tripType, ok := settings[2].(float64); !ok || int(tripType) != 1 {
		t.Errorf("trip type: got %v, want 1 (round-trip)", settings[2])
	}

	// Cabin class should be 3 (business)
	if cabin, ok := settings[5].(float64); !ok || int(cabin) != 3 {
		t.Errorf("cabin class: got %v, want 3 (business)", settings[5])
	}

	// Should have 2 segments
	segments := settings[13].([]any)
	if len(segments) != 2 {
		t.Errorf("segments: got %d, want 2", len(segments))
	}

	// Sort by should be 2 (cheapest)
	if sortBy, ok := arr[2].(float64); !ok || int(sortBy) != 2 {
		t.Errorf("sort by: got %v, want 2 (cheapest)", arr[2])
	}
}

func TestBuildFilters_BagsUseArrayWireFormat(t *testing.T) {
	opts := SearchOptions{
		Adults:      1,
		CabinClass:  models.Economy,
		CarryOnBags: 1,
		CheckedBags: 2,
	}
	filters := buildFilters("HEL", "NRT", "2026-06-15", opts)

	data, err := json.Marshal(filters)
	if err != nil {
		t.Fatalf("marshal filters: %v", err)
	}

	var arr []any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("unmarshal filters: %v", err)
	}

	settings, ok := arr[1].([]any)
	if !ok {
		t.Fatalf("arr[1] not array")
	}

	got, ok := settings[10].([]any)
	if !ok {
		t.Fatalf("settings[10] type = %T, want []any", settings[10])
	}
	if len(got) != 2 {
		t.Fatalf("settings[10] len = %d, want 2", len(got))
	}
	if carryOn, ok := got[0].(float64); !ok || int(carryOn) != 1 {
		t.Errorf("carry-on bags: got %v, want 1", got[0])
	}
	if checked, ok := got[1].(float64); !ok || int(checked) != 2 {
		t.Errorf("checked bags: got %v, want 2", got[1])
	}
}

func TestBagsFilter(t *testing.T) {
	tests := []struct {
		name    string
		carryOn int
		checked int
		want    any
	}{
		{name: "unset", carryOn: 0, checked: 0, want: nil},
		{name: "carry on only", carryOn: 1, checked: 0, want: []any{1, 0}},
		{name: "checked only", carryOn: 0, checked: 1, want: []any{0, 1}},
		{name: "both", carryOn: 2, checked: 1, want: []any{2, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bagsFilter(tt.carryOn, tt.checked)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("bagsFilter(%d, %d) = %#v, want %#v", tt.carryOn, tt.checked, got, tt.want)
			}
		})
	}
}

func TestSearchFlights_MissingParams(t *testing.T) {
	_, err := SearchFlights(t.Context(), "", "NRT", "2026-06-15", SearchOptions{})
	if err == nil {
		t.Error("expected error for missing origin")
	}

	_, err = SearchFlights(t.Context(), "HEL", "", "2026-06-15", SearchOptions{})
	if err == nil {
		t.Error("expected error for missing destination")
	}

	_, err = SearchFlights(t.Context(), "HEL", "NRT", "", SearchOptions{})
	if err == nil {
		t.Error("expected error for missing date")
	}
}

// TestParseFlights_Aircraft verifies that leg[17] is parsed into Aircraft.
func TestParseFlights_Aircraft(t *testing.T) {
	rawFlights := loadGoldenResponse(t)
	flights := parseFlights(rawFlights)

	if len(flights) < 3 {
		t.Fatalf("expected ≥3 flights, got %d", len(flights))
	}

	tests := []struct {
		name   string
		flIdx  int
		legIdx int
		want   string
	}{
		{"finnair A350", 0, 0, "Airbus A350"},
		{"lufthansa A320 leg0", 1, 0, "Airbus A320"},
		{"lufthansa B747 leg1", 1, 1, "Boeing 747"},
		{"jal B787", 2, 0, "Boeing 787"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := flights[tt.flIdx]
			if tt.legIdx >= len(f.Legs) {
				t.Fatalf("flight %d has %d legs, want leg %d", tt.flIdx, len(f.Legs), tt.legIdx)
			}
			if got := f.Legs[tt.legIdx].Aircraft; got != tt.want {
				t.Errorf("aircraft: got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseFlights_Emissions verifies CO2 emission parsing from flightInfo[12].
func TestParseFlights_Emissions(t *testing.T) {
	rawFlights := loadGoldenResponse(t)
	flights := parseFlights(rawFlights)

	if len(flights) < 3 {
		t.Fatalf("expected ≥3 flights, got %d", len(flights))
	}

	// Flights 1 and 2 have emissions in golden file; flight 3 does not.
	if flights[0].Emissions != 95000 {
		t.Errorf("flight 1 emissions: got %d, want 95000", flights[0].Emissions)
	}
	if flights[1].Emissions != 140000 {
		t.Errorf("flight 2 emissions: got %d, want 140000", flights[1].Emissions)
	}
	if flights[2].Emissions != 0 {
		t.Errorf("flight 3 emissions: got %d, want 0 (absent)", flights[2].Emissions)
	}
}

// TestParseFlights_Layovers verifies layover computation for connecting flights.
func TestParseFlights_Layovers(t *testing.T) {
	rawFlights := loadGoldenResponse(t)
	flights := parseFlights(rawFlights)

	if len(flights) < 2 {
		t.Fatalf("expected ≥2 flights, got %d", len(flights))
	}

	// Flight 1 (nonstop): first leg has no layover.
	if flights[0].Legs[0].LayoverMinutes != 0 {
		t.Errorf("nonstop leg 0 layover: got %d, want 0", flights[0].Legs[0].LayoverMinutes)
	}

	// Flight 2 (HEL->FRA->NRT):
	//   leg 0 arrives FRA at 2026-06-15T10:30
	//   leg 1 departs FRA at 2026-06-15T13:45
	//   layover = 195 minutes
	f2 := flights[1]
	if f2.Legs[0].LayoverMinutes != 0 {
		t.Errorf("f2 leg 0 layover: got %d, want 0 (first leg)", f2.Legs[0].LayoverMinutes)
	}
	if f2.Legs[1].LayoverMinutes != 195 {
		t.Errorf("f2 leg 1 layover: got %d, want 195", f2.Legs[1].LayoverMinutes)
	}
}

// TestComputeLayovers_Direct tests the layover helper directly.
func TestComputeLayovers_Direct(t *testing.T) {
	legs := []models.FlightLeg{
		{ArrivalTime: "2026-06-15T10:30"},
		{DepartureTime: "2026-06-15T14:00"},
		{DepartureTime: "2026-06-16T08:00"},
	}
	// Seed arrival times for legs 1 and 2.
	legs[1].ArrivalTime = "2026-06-15T21:00"
	legs[2].ArrivalTime = "2026-06-16T20:00"
	// Fix dep time for leg 0 (not needed by computeLayovers but keep struct valid).
	legs[0].DepartureTime = "2026-06-15T07:00"

	computeLayovers(legs)

	// leg[0] layover must remain 0.
	if legs[0].LayoverMinutes != 0 {
		t.Errorf("leg 0 layover: got %d, want 0", legs[0].LayoverMinutes)
	}
	// leg[1]: arr[0]=10:30, dep[1]=14:00 → 210 min
	if legs[1].LayoverMinutes != 210 {
		t.Errorf("leg 1 layover: got %d, want 210", legs[1].LayoverMinutes)
	}
	// leg[2]: arr[1]=21:00, dep[2]=next day 08:00 → 11h = 660 min
	if legs[2].LayoverMinutes != 660 {
		t.Errorf("leg 2 layover: got %d, want 660", legs[2].LayoverMinutes)
	}
}

// TestComputeLayovers_MissingTimes verifies graceful handling of absent timestamps.
func TestComputeLayovers_MissingTimes(t *testing.T) {
	legs := []models.FlightLeg{
		{ArrivalTime: ""},                   // missing arrival on first
		{DepartureTime: "2026-06-15T14:00"}, // missing departure on second
	}
	computeLayovers(legs)
	if legs[1].LayoverMinutes != 0 {
		t.Errorf("expected 0 layover on missing time, got %d", legs[1].LayoverMinutes)
	}
}
