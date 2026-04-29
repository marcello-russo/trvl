package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/visa"
)

func TestArgString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
		key  string
		want string
	}{
		{"nil args", nil, "x", ""},
		{"missing key", map[string]any{"a": "b"}, "x", ""},
		{"non-string value", map[string]any{"x": 42}, "x", ""},
		{"empty string", map[string]any{"x": ""}, "x", ""},
		{"normal string", map[string]any{"x": "hello"}, "x", "hello"},
		{"bool value returns empty", map[string]any{"x": true}, "x", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := argString(tt.args, tt.key); got != tt.want {
				t.Errorf("argString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================================
// argInt
// ============================================================

func TestArgInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
		key  string
		def  int
		want int
	}{
		{"nil args", nil, "x", 5, 5},
		{"missing key", map[string]any{}, "x", 5, 5},
		{"float64 value", map[string]any{"x": float64(42)}, "x", 0, 42},
		{"int value", map[string]any{"x": 7}, "x", 0, 7},
		{"string value returns default", map[string]any{"x": "hello"}, "x", 10, 10},
		{"bool value returns default", map[string]any{"x": true}, "x", 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := argInt(tt.args, tt.key, tt.def); got != tt.want {
				t.Errorf("argInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestArgInt_JSONNumber2(t *testing.T) {
	t.Parallel()
	// Simulate JSON number parsing.
	var m map[string]any
	raw := `{"x": 99}`
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		t.Fatal(err)
	}
	if got := argInt(m, "x", 0); got != 99 {
		t.Errorf("argInt(json.Number) = %d, want 99", got)
	}
}

func TestArgInt_JSONNumberInvalid2(t *testing.T) {
	t.Parallel()
	// json.Number that is not a valid int64.
	m := map[string]any{"x": json.Number("3.14")}
	if got := argInt(m, "x", 7); got != 7 {
		t.Errorf("argInt(non-int json.Number) = %d, want 7", got)
	}
}

// ============================================================
// argFloat
// ============================================================

func TestArgFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
		key  string
		def  float64
		want float64
	}{
		{"nil args", nil, "x", 1.5, 1.5},
		{"missing key", map[string]any{}, "x", 1.5, 1.5},
		{"float64 value", map[string]any{"x": 3.14}, "x", 0, 3.14},
		{"int value", map[string]any{"x": 7}, "x", 0, 7.0},
		{"string value returns default", map[string]any{"x": "hello"}, "x", 9.9, 9.9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := argFloat(tt.args, tt.key, tt.def); got != tt.want {
				t.Errorf("argFloat() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestArgFloat_JSONNumber2(t *testing.T) {
	t.Parallel()
	var m map[string]any
	raw := `{"x": 3.14}`
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		t.Fatal(err)
	}
	got := argFloat(m, "x", 0)
	if got < 3.13 || got > 3.15 {
		t.Errorf("argFloat(json.Number) = %f, want ~3.14", got)
	}
}

func TestArgFloat_JSONNumberInvalid2(t *testing.T) {
	t.Parallel()
	m := map[string]any{"x": json.Number("not-a-number")}
	if got := argFloat(m, "x", 7.7); got != 7.7 {
		t.Errorf("argFloat(invalid json.Number) = %f, want 7.7", got)
	}
}

// ============================================================
// argStringSlice
// ============================================================

func TestArgStringSlice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
		key  string
		want []string
	}{
		{"nil args", nil, "x", nil},
		{"missing key", map[string]any{}, "x", nil},
		{"comma-separated string", map[string]any{"x": "a, b, c"}, "x", []string{"a", "b", "c"}},
		{"single value string", map[string]any{"x": "one"}, "x", []string{"one"}},
		{"empty string", map[string]any{"x": ""}, "x", nil},
		{"JSON array", map[string]any{"x": []any{"a", "b"}}, "x", []string{"a", "b"}},
		{"JSON array with non-strings", map[string]any{"x": []any{42, "b"}}, "x", []string{"b"}},
		{"int value returns nil", map[string]any{"x": 42}, "x", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := argStringSlice(tt.args, tt.key)
			if len(got) != len(tt.want) {
				t.Errorf("argStringSlice() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("argStringSlice()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestArgStringSlice_TrimsWhitespace(t *testing.T) {
	t.Parallel()
	args := map[string]any{"x": "  a , b , c  "}
	got := argStringSlice(args, "x")
	expected := []string{"a", "b", "c"}
	if len(got) != len(expected) {
		t.Fatalf("len = %d, want %d", len(got), len(expected))
	}
	for i, v := range got {
		if v != expected[i] {
			t.Errorf("[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestArgStringSlice_SkipsEmptyParts(t *testing.T) {
	t.Parallel()
	args := map[string]any{"x": "a,,b,"}
	got := argStringSlice(args, "x")
	expected := []string{"a", "b"}
	if len(got) != len(expected) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(expected), got)
	}
}

// ============================================================
// argBool
// ============================================================

func TestArgBool(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
		key  string
		def  bool
		want bool
	}{
		{"nil args", nil, "x", true, true},
		{"missing key", map[string]any{}, "x", false, false},
		{"true value", map[string]any{"x": true}, "x", false, true},
		{"false value", map[string]any{"x": false}, "x", true, false},
		{"non-bool returns default", map[string]any{"x": "yes"}, "x", true, true},
		{"int returns default", map[string]any{"x": 1}, "x", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := argBool(tt.args, tt.key, tt.def); got != tt.want {
				t.Errorf("argBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// extractBestFlightPrice
// ============================================================

func TestExtractBestFlightPrice_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		m        map[string]interface{}
		wantP    float64
		wantCurr string
	}{
		{
			"no flights key",
			map[string]interface{}{},
			0, "",
		},
		{
			"flights not array",
			map[string]interface{}{"flights": "bad"},
			0, "",
		},
		{
			"empty flights",
			map[string]interface{}{"flights": []interface{}{}},
			0, "",
		},
		{
			"single flight",
			map[string]interface{}{
				"flights": []interface{}{
					map[string]interface{}{"price": 150.0, "currency": "EUR"},
				},
			},
			150, "EUR",
		},
		{
			"multiple flights picks cheapest",
			map[string]interface{}{
				"flights": []interface{}{
					map[string]interface{}{"price": 300.0, "currency": "EUR"},
					map[string]interface{}{"price": 150.0, "currency": "EUR"},
					map[string]interface{}{"price": 200.0, "currency": "USD"},
				},
			},
			150, "EUR",
		},
		{
			"zero price flights ignored",
			map[string]interface{}{
				"flights": []interface{}{
					map[string]interface{}{"price": 0.0, "currency": "EUR"},
					map[string]interface{}{"price": 250.0, "currency": "EUR"},
				},
			},
			250, "EUR",
		},
		{
			"non-map flight entries ignored",
			map[string]interface{}{
				"flights": []interface{}{"not a map", 42},
			},
			0, "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, c := extractBestFlightPrice(tt.m)
			if p != tt.wantP || c != tt.wantCurr {
				t.Errorf("extractBestFlightPrice() = (%.0f, %q), want (%.0f, %q)", p, c, tt.wantP, tt.wantCurr)
			}
		})
	}
}

// ============================================================
// extractBestHotelPrice
// ============================================================

func TestExtractBestHotelPrice_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		m        map[string]interface{}
		wantP    float64
		wantCurr string
	}{
		{
			"no hotels key",
			map[string]interface{}{},
			0, "",
		},
		{
			"hotels not array",
			map[string]interface{}{"hotels": "bad"},
			0, "",
		},
		{
			"empty hotels",
			map[string]interface{}{"hotels": []interface{}{}},
			0, "",
		},
		{
			"multiple hotels picks cheapest",
			map[string]interface{}{
				"hotels": []interface{}{
					map[string]interface{}{"price": 200.0, "currency": "EUR"},
					map[string]interface{}{"price": 80.0, "currency": "EUR"},
					map[string]interface{}{"price": 120.0, "currency": "USD"},
				},
			},
			80, "EUR",
		},
		{
			"non-map hotel entries ignored",
			map[string]interface{}{
				"hotels": []interface{}{"not a map"},
			},
			0, "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, c := extractBestHotelPrice(tt.m)
			if p != tt.wantP || c != tt.wantCurr {
				t.Errorf("extractBestHotelPrice() = (%.0f, %q), want (%.0f, %q)", p, c, tt.wantP, tt.wantCurr)
			}
		})
	}
}

// ============================================================
// buildAnnotatedContentBlocks
// ============================================================

func TestBuildAnnotatedContentBlocks_Coverage(t *testing.T) {
	t.Parallel()
	data := map[string]string{"key": "value"}
	blocks, err := buildAnnotatedContentBlocks("summary text", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	// First block: user-facing text.
	if blocks[0].Type != "text" {
		t.Errorf("block[0].Type = %q, want text", blocks[0].Type)
	}
	if blocks[0].Text != "summary text" {
		t.Errorf("block[0].Text = %q, want summary text", blocks[0].Text)
	}
	if blocks[0].Annotations == nil {
		t.Fatal("block[0].Annotations is nil")
	}
	if len(blocks[0].Annotations.Audience) != 1 || blocks[0].Annotations.Audience[0] != "user" {
		t.Errorf("block[0] audience = %v, want [user]", blocks[0].Annotations.Audience)
	}
	if blocks[0].Annotations.Priority != 1.0 {
		t.Errorf("block[0] priority = %f, want 1.0", blocks[0].Annotations.Priority)
	}

	// Second block: assistant-facing JSON.
	if blocks[1].Type != "text" {
		t.Errorf("block[1].Type = %q, want text", blocks[1].Type)
	}
	if !strings.Contains(blocks[1].Text, `"key"`) {
		t.Errorf("block[1] missing JSON key")
	}
	if blocks[1].Annotations.Priority != 0.5 {
		t.Errorf("block[1] priority = %f, want 0.5", blocks[1].Annotations.Priority)
	}
}

// ============================================================
// watchURIFromQuery
// ============================================================

func TestWatchURIFromQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		query string
		want  string
	}{
		{"HEL->BCN 2026-07-01", "trvl://watch/HEL-BCN-2026-07-01"},
		{"HEL->BCN 2026-07-01 (round-trip return 2026-07-08)", "trvl://watch/HEL-BCN-2026-07-01"},
		{"short", ""},
		{"", ""},
		{"NOARROW 2026-07-01", ""},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			if got := watchURIFromQuery(tt.query); got != tt.want {
				t.Errorf("watchURIFromQuery(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

// ============================================================
// priceCache
// ============================================================

func TestPriceCache(t *testing.T) {
	t.Parallel()
	c := newPriceCache()

	// Miss.
	_, ok := c.get("HEL-BCN-2026-07-01")
	if ok {
		t.Error("expected miss for new cache")
	}

	// Set and hit.
	c.set("HEL-BCN-2026-07-01", 199.0)
	v, ok := c.get("HEL-BCN-2026-07-01")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if v != 199.0 {
		t.Errorf("cache value = %f, want 199", v)
	}

	// Overwrite.
	c.set("HEL-BCN-2026-07-01", 180.0)
	v, _ = c.get("HEL-BCN-2026-07-01")
	if v != 180.0 {
		t.Errorf("cache value after update = %f, want 180", v)
	}
}

// ============================================================
// toolExecutionError / toolResultError
// ============================================================

func TestToolExecutionError(t *testing.T) {
	t.Parallel()
	err := toolExecutionError("Flight search", nil)
	if err == nil || !strings.Contains(err.Error(), "Flight search failed") {
		t.Errorf("toolExecutionError = %v", err)
	}
}

func TestToolResultError(t *testing.T) {
	t.Parallel()
	err := toolResultError("Hotel search", "no results")
	if err == nil || !strings.Contains(err.Error(), "no results") {
		t.Errorf("toolResultError = %v", err)
	}
}

// ============================================================
// flightProviderSummaryLabel
// ============================================================

func TestFlightProviderSummaryLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		provider string
		want     string
	}{
		{"google_flights", "Google Flights"},
		{"Google_Flights", "Google Flights"},
		{"kiwi", "Kiwi"},
		{"KIWI", "Kiwi"},
		{"other_provider", "other_provider"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			if got := flightProviderSummaryLabel(tt.provider); got != tt.want {
				t.Errorf("flightProviderSummaryLabel(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

// ============================================================
// pluralSuffix
// ============================================================

func TestPluralSuffix(t *testing.T) {
	t.Parallel()
	if got := pluralSuffix(1); got != "" {
		t.Errorf("pluralSuffix(1) = %q, want empty", got)
	}
	if got := pluralSuffix(0); got != "s" {
		t.Errorf("pluralSuffix(0) = %q, want s", got)
	}
	if got := pluralSuffix(5); got != "s" {
		t.Errorf("pluralSuffix(5) = %q, want s", got)
	}
}

// ============================================================
// buildVisaSummary
// ============================================================

func TestBuildVisaSummary_Success(t *testing.T) {
	t.Parallel()
	result := visa.Result{
		Success: true,
		Requirement: visa.Requirement{
			Passport:    "FI",
			Destination: "JP",
			Status:      "visa-free",
			MaxStay:     "90 days",
			Notes:       "Tourism only",
		},
	}
	got := buildVisaSummary(result)
	if !strings.Contains(got, "visa-free") {
		t.Errorf("expected visa-free in summary")
	}
	if !strings.Contains(got, "90 days") {
		t.Errorf("expected max stay in summary")
	}
	if !strings.Contains(got, "Tourism only") {
		t.Errorf("expected notes in summary")
	}
	if !strings.Contains(got, "FI") {
		t.Errorf("expected passport code in summary")
	}
}
