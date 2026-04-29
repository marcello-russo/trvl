package hotels

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestParseReviewsFromRawEntries_WithReviews(t *testing.T) {
	// Build entries that contain review-like arrays.
	entries := []any{
		[]any{
			[]any{
				[]any{"This hotel was absolutely wonderful and amazing", 5.0, "Alice", "2 weeks ago"},
				[]any{"Terrible experience, would not recommend this", 1.0, "Bob", "3 days ago"},
			},
		},
	}
	result, err := parseReviewsFromRawEntries(entries, "test-id", ReviewOptions{Limit: 10, Sort: "highest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Reviews) < 1 {
		t.Errorf("expected reviews, got %d", len(result.Reviews))
	}
}

func TestParseReviewsFromRawEntries_LimitApplied(t *testing.T) {
	entries := []any{
		[]any{
			[]any{
				[]any{"Great place to stay for a family vacation", 4.0, "One", "1 week ago"},
				[]any{"Another amazing hotel right on the beach", 5.0, "Two", "2 weeks ago"},
				[]any{"Not bad but could be better with service", 3.0, "Three", "1 month ago"},
			},
		},
	}
	result, err := parseReviewsFromRawEntries(entries, "test-id", ReviewOptions{Limit: 1, Sort: "newest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Reviews) > 1 {
		t.Errorf("expected limit=1, got %d", len(result.Reviews))
	}
}

// ---------------------------------------------------------------------------
// extractTotalAvailable — more data patterns
// ---------------------------------------------------------------------------

func TestCov_ExtractTotalAvailable_Key416343588(t *testing.T) {
	// Build nested structure matching the expected path.
	data := []any{
		[]any{
			[]any{
				[]any{nil, []any{
					[]any{nil, map[string]any{
						"416343588": []any{42.0},
					}},
				}},
			},
		},
	}
	got := extractTotalAvailable(data)
	if got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestCov_ExtractTotalAvailable_Key410579159(t *testing.T) {
	data := []any{
		[]any{
			[]any{
				[]any{nil, []any{
					[]any{nil, map[string]any{
						"410579159": []any{"cursor", "", 100.0, 1.0, 20.0},
					}},
				}},
			},
		},
	}
	got := extractTotalAvailable(data)
	if got != 100 {
		t.Errorf("expected 100, got %d", got)
	}
}

func TestExtractTotalAvailable_NilData(t *testing.T) {
	if got := extractTotalAvailable(nil); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// tryParseOneReview — nested rating and sub-array author
// ---------------------------------------------------------------------------

func TestTryParseOneReview_NestedRating(t *testing.T) {
	arr := []any{
		"Author Name",
		"This is a really wonderful hotel with amazing views",
		[]any{4.0}, // nested rating
		"2 weeks ago",
	}
	r, ok := tryParseOneReview(arr)
	if !ok {
		t.Fatal("expected to parse review")
	}
	if r.Rating != 4.0 {
		t.Errorf("Rating = %v, want 4.0", r.Rating)
	}
}

func TestCov_TryParseOneReview_TooShort(t *testing.T) {
	arr := []any{"short", 3.0}
	_, ok := tryParseOneReview(arr)
	if ok {
		t.Error("should reject array with < 3 elements")
	}
}

func TestTryParseOneReview_NoTextNoRating(t *testing.T) {
	arr := []any{42, 43, 44}
	_, ok := tryParseOneReview(arr)
	if ok {
		t.Error("should reject array without text and rating")
	}
}

// ---------------------------------------------------------------------------
// extractHotelNameFromCallback — deeper paths
// ---------------------------------------------------------------------------

func TestExtractHotelNameFromCallback_DeepPath(t *testing.T) {
	// Hotel name at [0][0][0].
	data := []any{
		[]any{
			[]any{
				"Deep Nested Hotel Name Here",
			},
		},
	}
	name := extractHotelNameFromCallback(data)
	if name != "Deep Nested Hotel Name Here" {
		t.Errorf("got %q", name)
	}
}

func TestExtractHotelNameFromCallback_NoValidName(t *testing.T) {
	// Only short strings and numbers.
	data := []any{[]any{[]any{42}}}
	name := extractHotelNameFromCallback(data)
	if name != "" {
		t.Errorf("expected empty, got %q", name)
	}
}

// ---------------------------------------------------------------------------
// extractOrganicPrice — additional patterns
// ---------------------------------------------------------------------------

func TestExtractOrganicPrice_NilInner(t *testing.T) {
	got, cur := extractOrganicPrice(nil)
	if got != 0 || cur != "" {
		t.Errorf("expected (0, \"\"), got (%v, %q)", got, cur)
	}
}

func TestCov_ExtractOrganicPrice_NotArray(t *testing.T) {
	got, _ := extractOrganicPrice("not-array")
	if got != 0 {
		t.Errorf("expected 0, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// parseReviewsFromBatchResponse — with valid payload
// ---------------------------------------------------------------------------

func TestParseReviewsFromBatchResponse_FallbackToRaw(t *testing.T) {
	// Entries without "ocp93e" marker — falls back to parseReviewsFromRawEntries.
	entries := []any{
		[]any{
			[]any{
				[]any{"Really great experience staying at this hotel", 5.0, "Guest", "1 week ago"},
				[]any{"Could have been better but overall it was ok", 3.0, "Visitor", "2 months ago"},
			},
		},
	}
	result, err := parseReviewsFromBatchResponse(entries, "test-id", ReviewOptions{Limit: 10, Sort: "newest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

// ---------------------------------------------------------------------------
// parseReviewsFromPage — limit and sort branches
// ---------------------------------------------------------------------------

func TestParseReviewsFromPage_LimitApplied(t *testing.T) {
	// Multiple reviews via text extraction, limit should truncate.
	page := "AF_initDataCallback({key: 'ds:0', data:[\"empty\"]});\n" +
		`<script>{"reviewRating":{"ratingValue":"5.0"},"reviewBody":"Wonderful hotel with great amenities and pool","author":{"name":"A"}}</script>` + "\n" +
		`<script>{"reviewRating":{"ratingValue":"4.0"},"reviewBody":"Good location near the city center and shops","author":{"name":"B"}}</script>` + "\n" +
		`<script>{"reviewRating":{"ratingValue":"3.0"},"reviewBody":"Average stay nothing special about this place","author":{"name":"C"}}</script>`
	result, err := parseReviewsFromPage(page, "test-id", ReviewOptions{Limit: 1, Sort: "highest"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Reviews) > 1 {
		t.Errorf("expected limit=1, got %d", len(result.Reviews))
	}
}

// ---------------------------------------------------------------------------
// findRoomData — single room entry (not list)
// ---------------------------------------------------------------------------

func TestFindRoomData_SingleEntry(t *testing.T) {
	data := []any{
		"Deluxe King Room", 299.0, "EUR", "Booking.com",
	}
	rooms := findRoomData(data, "EUR", 0)
	if len(rooms) != 1 {
		t.Errorf("expected 1 room from single entry, got %d", len(rooms))
	}
}

// ---------------------------------------------------------------------------
// extractLocationFromSearchData — placeID without 0x prefix
// ---------------------------------------------------------------------------

func TestExtractLocationFromSearchData_NonHexPlaceID(t *testing.T) {
	// placeID without "0x" prefix should NOT match.
	data := []any{nil, "SomeCity", "notahexid"}
	loc := extractLocationFromSearchData(data, 0)
	if loc != "" {
		t.Errorf("expected empty for non-hex placeID, got %q", loc)
	}
}

func TestExtractLocationFromSearchData_ShortCity(t *testing.T) {
	// City name with 1 char should not match (< 2 chars).
	data := []any{nil, "X", "0xabc"}
	loc := extractLocationFromSearchData(data, 0)
	if loc != "" {
		t.Errorf("expected empty for short city, got %q", loc)
	}
}

// ---------------------------------------------------------------------------
// isGoogleConsentPage
// ---------------------------------------------------------------------------

func TestIsGoogleConsentPage_Positive(t *testing.T) {
	for _, marker := range []string{
		"consent.google.com",
		`action="https://consent.google."`,
		`id="SOCS"`,
		"SOCS blah consentheading",
	} {
		page := []byte("<html>" + marker + "</html>")
		if !isGoogleConsentPage(page) {
			t.Errorf("expected consent page for marker: %s", marker)
		}
	}
}

func TestIsGoogleConsentPage_Negative(t *testing.T) {
	page := []byte("<html>normal hotel search results</html>")
	if isGoogleConsentPage(page) {
		t.Error("should not be consent page")
	}
}

// ---------------------------------------------------------------------------
// buildHotelBookingURL
// ---------------------------------------------------------------------------

func TestBuildHotelBookingURL(t *testing.T) {
	url := buildHotelBookingURL("Paris", "2026-07-01", "2026-07-05")
	if !strings.Contains(url, "Paris") || !strings.Contains(url, "2026-07-01") {
		t.Errorf("unexpected URL: %s", url)
	}
}

// ---------------------------------------------------------------------------
// Haversine
// ---------------------------------------------------------------------------

func TestCov_Haversine_SamePoint(t *testing.T) {
	d := Haversine(60.17, 24.93, 60.17, 24.93)
	if d != 0 {
		t.Errorf("expected 0 for same point, got %v", d)
	}
}

func TestHaversine_KnownDistance(t *testing.T) {
	// Helsinki to Tallinn is approximately 80km.
	d := Haversine(60.17, 24.93, 59.43, 24.75)
	if d < 50 || d > 120 {
		t.Errorf("Helsinki-Tallinn should be ~80km, got %v", d)
	}
}

// ---------------------------------------------------------------------------
// parseTrivagoAccommodations — fallback key paths
// ---------------------------------------------------------------------------

func TestParseTrivagoAccommodations_NestedKey(t *testing.T) {
	// Accommodations under "hotels" key instead of "accommodations".
	raw := json.RawMessage(`{"hotels":[{"accommodation_name":"Nested Hotel","price_per_night":"150","currency":"EUR"}]}`)
	results, err := parseTrivagoAccommodations(raw, "EUR")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if results[0].Name != "Nested Hotel" {
		t.Errorf("Name = %q", results[0].Name)
	}
}

func TestParseTrivagoAccommodations_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`not json`)
	_, err := parseTrivagoAccommodations(raw, "EUR")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseTrivagoAccommodations_EmptyAccommodations(t *testing.T) {
	raw := json.RawMessage(`{"accommodations":[]}`)
	results, err := parseTrivagoAccommodations(raw, "EUR")
	// Empty accommodations = empty slice, no error (obscure location).
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty, got %d results", len(results))
	}
}

// ---------------------------------------------------------------------------
// tryAmenityGroup — edge cases
// ---------------------------------------------------------------------------

func TestTryAmenityGroup_ValidGroup(t *testing.T) {
	arr := []any{
		"Popular",
		[]any{
			[]any{"Free WiFi"},
			[]any{"Pool"},
			[]any{"Air conditioning"},
		},
	}
	names := tryAmenityGroup(arr)
	if len(names) != 3 {
		t.Errorf("expected 3, got %d: %v", len(names), names)
	}
}

func TestTryAmenityGroup_NotGroupName(t *testing.T) {
	arr := []any{"SomeRandomString", []any{[]any{"WiFi"}}}
	names := tryAmenityGroup(arr)
	if names != nil {
		t.Errorf("expected nil for non-group name, got %v", names)
	}
}

func TestTryAmenityGroup_TooShort(t *testing.T) {
	if names := tryAmenityGroup([]any{"Popular"}); names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

func TestTryAmenityGroup_SecondNotArray(t *testing.T) {
	arr := []any{"Popular", "not-an-array"}
	if names := tryAmenityGroup(arr); names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

// ---------------------------------------------------------------------------
// tryAmenityCodePairs — edge cases
// ---------------------------------------------------------------------------

func TestTryAmenityCodePairs_ValidPairs(t *testing.T) {
	// Pairs of [available, code] where available=1 means present.
	arr := []any{
		[]any{1.0, 7.0},  // Pool
		[]any{1.0, 9.0},  // Gym
		[]any{0.0, 12.0}, // Not available
	}
	names := tryAmenityCodePairs(arr)
	if len(names) < 1 {
		t.Errorf("expected amenity names, got %d: %v", len(names), names)
	}
}

func TestTryAmenityCodePairs_TooFewElements(t *testing.T) {
	if names := tryAmenityCodePairs([]any{[]any{1.0, 7.0}, []any{1.0, 9.0}}); names != nil {
		t.Errorf("expected nil for < 3 elements, got %v", names)
	}
}

func TestCov_TryAmenityCodePairs_NotPairs(t *testing.T) {
	// Elements are not 2-element arrays.
	arr := []any{[]any{1.0, 7.0, "extra"}, []any{1.0}, []any{1.0, 9.0}}
	if names := tryAmenityCodePairs(arr); names != nil {
		t.Errorf("expected nil for invalid pairs, got %v", names)
	}
}

// ---------------------------------------------------------------------------
// tryFlatAmenityList — edge cases
// ---------------------------------------------------------------------------

func TestTryFlatAmenityList_ValidList(t *testing.T) {
	arr := []any{
		[]any{"Free WiFi"},
		[]any{"Pool"},
		[]any{"Spa"},
		[]any{"Gym"},
	}
	names := tryFlatAmenityList(arr)
	if len(names) != 4 {
		t.Errorf("expected 4, got %d: %v", len(names), names)
	}
}

func TestTryFlatAmenityList_NotAmenityLike(t *testing.T) {
	arr := []any{
		[]any{"Random String One"},
		[]any{"Random String Two"},
		[]any{"Random String Three"},
	}
	names := tryFlatAmenityList(arr)
	if names != nil {
		t.Errorf("expected nil for non-amenity strings, got %v", names)
	}
}

// ---------------------------------------------------------------------------
// parseDetailAmenities — page with amenity groups
// ---------------------------------------------------------------------------

func TestParseDetailAmenities_WithGroups(t *testing.T) {
	// Build a page with callbacks containing amenity group data.
	data := []any{
		"Popular",
		[]any{
			[]any{"Free WiFi"},
			[]any{"Pool"},
			[]any{"Breakfast included"},
		},
	}
	jsonBytes, _ := json.Marshal([]any{data})
	page := fmt.Sprintf("AF_initDataCallback({key: 'ds:0', data:%s});", string(jsonBytes))
	amenities, err := parseDetailAmenities(page)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(amenities) < 3 {
		t.Errorf("expected at least 3 amenities, got %d: %v", len(amenities), amenities)
	}
}

func TestCov_ParseDetailAmenities_NoCallbacks(t *testing.T) {
	if _, err := parseDetailAmenities("<html>nothing</html>"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCov_ParseDetailAmenities_NoAmenities(t *testing.T) {
	page := "AF_initDataCallback({key: 'ds:0', data:[\"not amenity data\"]});"
	if _, err := parseDetailAmenities(page); err == nil {
		t.Fatal("expected error for no amenities found")
	}
}

// ---------------------------------------------------------------------------
// mapTrivagoAccommodations — edge cases
// ---------------------------------------------------------------------------

func TestMapTrivagoAccommodations_MixedData(t *testing.T) {
	accoms := []trivagoAccommodation{
		{
			AccommodationName: "Good Hotel",
			PricePerNight:     "\u20ac200",
			Currency:          "EUR",
			HotelRating:       4,
			ReviewRating:      "8.5",
			ReviewCount:       150,
			Lat:               48.85,
			Lon:               2.35,
			AccommodationURL:  "https://trivago.com/book/good",
		},
		{
			AccommodationName: "No Price Hotel",
			PricePerNight:     "",
			AccommodationURL:  "https://trivago.com/book/noprice",
		},
	}
	results := mapTrivagoAccommodations(accoms, "EUR")
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	if results[0].Name != "Good Hotel" {
		t.Errorf("Name = %q", results[0].Name)
	}
}

// ---------------------------------------------------------------------------
// extractSponsoredAmenities — edge cases
// ---------------------------------------------------------------------------

func TestCov_ExtractSponsoredAmenities_Nil(t *testing.T) {
	got := extractSponsoredAmenities(nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestExtractSponsoredAmenities_Valid(t *testing.T) {
	// Amenities are flat float64 codes that map to amenityCodeMap entries.
	// 7=fitness_center, 4=pool, 2=free_wifi
	data := []any{float64(7), float64(4), float64(2)}
	got := extractSponsoredAmenities(data)
	if len(got) < 1 {
		t.Errorf("expected amenities, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// extractSponsoredHotels — edge cases for coverage
// ---------------------------------------------------------------------------

func TestExtractSponsoredHotels_NilData(t *testing.T) {
	got := extractSponsoredHotels(nil, "USD")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// parseHotelsFromPageFull — sponsored section
// ---------------------------------------------------------------------------

func TestParseHotelsFromPageFull_EmptyPage(t *testing.T) {
	result := parseHotelsFromPageFull("", "USD")
	if len(result.Hotels) != 0 {
		t.Errorf("expected 0 hotels, got %d", len(result.Hotels))
	}
}

// ---------------------------------------------------------------------------
// sortHotels — distance sort
// ---------------------------------------------------------------------------

func TestCov_SortHotels_Distance(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "Far", Lat: 61.0, Lon: 25.0},
		{Name: "Near", Lat: 60.18, Lon: 24.94},
	}
	sortHotels(hotels, "distance", 60.17, 24.93)
	if hotels[0].Name != "Near" {
		t.Errorf("expected Near first, got %q", hotels[0].Name)
	}
}

func TestCov_SortHotels_Stars(t *testing.T) {
	hotels := []models.HotelResult{
		{Name: "3star", Stars: 3},
		{Name: "5star", Stars: 5},
	}
	sortHotels(hotels, "stars", 0, 0)
	if hotels[0].Name != "5star" {
		t.Errorf("expected 5star first, got %q", hotels[0].Name)
	}
}
