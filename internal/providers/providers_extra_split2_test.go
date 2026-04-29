package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripUnresolvedPlaceholders_RemovesParam(t *testing.T) {
	got := stripUnresolvedPlaceholders("https://example.com/search?q=test&nflt=${nflt}")
	if got != "https://example.com/search?q=test" {
		t.Errorf("got %q", got)
	}
}

func TestStripUnresolvedPlaceholders_NoPlaceholders(t *testing.T) {
	input := "https://example.com/search?q=test"
	got := stripUnresolvedPlaceholders(input)
	if got != input {
		t.Errorf("got %q", got)
	}
}

func TestStripUnresolvedPlaceholders_MultiplePlaceholders(t *testing.T) {
	got := stripUnresolvedPlaceholders("https://example.com?a=1&b=${b}&c=${c}&d=4")
	// Both ${b} and ${c} and their &key= prefixes should be removed.
	if got != "https://example.com?a=1&d=4" {
		t.Errorf("got %q", got)
	}
}

// ============================================================
// toFloat64
// ============================================================

func TestToFloat64_Float(t *testing.T) {
	if got := toFloat64(42.5); got != 42.5 {
		t.Errorf("got %v", got)
	}
}

func TestToFloat64_Int(t *testing.T) {
	if got := toFloat64(42); got != 42.0 {
		t.Errorf("got %v", got)
	}
}

func TestToFloat64_String(t *testing.T) {
	if got := toFloat64("42.5"); got != 42.5 {
		t.Errorf("got %v", got)
	}
}

func TestToFloat64_StringWithCurrency(t *testing.T) {
	got := toFloat64("€ 61")
	if got != 61.0 {
		t.Errorf("got %v, want 61", got)
	}
}

func TestToFloat64_StringComposite(t *testing.T) {
	got := toFloat64("4.84 (25)")
	if got != 4.84 {
		t.Errorf("got %v, want 4.84", got)
	}
}

func TestToFloat64_Nil(t *testing.T) {
	if got := toFloat64(nil); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestToFloat64_Bool(t *testing.T) {
	if got := toFloat64(true); got != 0 {
		t.Errorf("got %v, want 0 (unsupported type)", got)
	}
}

func TestToFloat64_EmptyString(t *testing.T) {
	if got := toFloat64(""); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// ============================================================
// normalizePrice
// ============================================================

func TestNormalizePrice_SameCurrency(t *testing.T) {
	got := normalizePrice(100, "EUR", "EUR")
	if got != 100 {
		t.Errorf("got %v, want 100", got)
	}
}

func TestNormalizePrice_EmptyFrom(t *testing.T) {
	got := normalizePrice(100, "", "USD")
	if got != 100 {
		t.Errorf("got %v, want 100", got)
	}
}

func TestNormalizePrice_EmptyTo(t *testing.T) {
	got := normalizePrice(100, "EUR", "")
	if got != 100 {
		t.Errorf("got %v, want 100", got)
	}
}

// ============================================================
// NewRegistryAt — loading existing JSON files
// ============================================================

func TestNewRegistryAt_LoadsExistingJSON(t *testing.T) {
	dir := t.TempDir()
	// Write a valid config file.
	data := `{"id":"preloaded","name":"Pre","category":"hotel","endpoint":"https://a.com","response_mapping":{"results_path":"r"}}`
	if err := os.WriteFile(filepath.Join(dir, "preloaded.json"), []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}
	got := reg.Get("preloaded")
	if got == nil {
		t.Fatal("expected preloaded config to be loaded")
	}
	if got.Name != "Pre" {
		t.Errorf("Name = %q, want 'Pre'", got.Name)
	}
}

func TestNewRegistryAt_SkipsNonJSON_Extra(t *testing.T) {
	dir := t.TempDir()
	// Write a non-JSON file.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}
	if len(reg.List()) != 0 {
		t.Error("expected 0 configs for non-JSON files")
	}
}

func TestNewRegistryAt_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := NewRegistryAt(dir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ============================================================
// Valid categories — all categories accepted
// ============================================================

func TestValidate_AllCategories(t *testing.T) {
	for _, cat := range []string{"hotel", "hotels", "flight", "flights", "ground", "restaurant", "restaurants", "review", "reviews"} {
		cfg := ProviderConfig{
			ID: "cat-test", Name: "Test", Category: cat,
			Endpoint:        "https://api.example.com",
			ResponseMapping: ResponseMapping{ResultsPath: "r"},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("category %q should be valid, got error: %v", cat, err)
		}
	}
}

// ============================================================
// extractNeighborhood — deterministic JSON paths
// ============================================================

func TestExtractNeighborhood_PrimaryPath(t *testing.T) {
	raw := map[string]any{
		"basicPropertyData": map[string]any{
			"location": map[string]any{
				"neighbourhood": map[string]any{
					"name": "Kreuzberg",
				},
			},
		},
	}
	got := extractNeighborhood(raw)
	if got != "Kreuzberg" {
		t.Errorf("got %q, want 'Kreuzberg'", got)
	}
}

func TestExtractNeighborhood_FallbackPath(t *testing.T) {
	raw := map[string]any{
		"basicPropertyData": map[string]any{
			"neighbourhood": map[string]any{
				"name": "Mitte",
			},
		},
	}
	got := extractNeighborhood(raw)
	if got != "Mitte" {
		t.Errorf("got %q, want 'Mitte'", got)
	}
}

func TestExtractNeighborhood_NoData(t *testing.T) {
	raw := map[string]any{
		"basicPropertyData": map[string]any{},
	}
	got := extractNeighborhood(raw)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ============================================================
// extractBlocksPriceSpread — deterministic JSON paths
// ============================================================

func TestExtractBlocksPriceSpread_WithBlocks(t *testing.T) {
	raw := map[string]any{
		"blocks": []any{
			map[string]any{
				"finalPrice": map[string]any{"amount": 120.0},
				"blockId":    map[string]any{"roomId": "101"},
			},
			map[string]any{
				"finalPrice": map[string]any{"amount": 280.0},
				"blockId":    map[string]any{"roomId": "102"},
			},
			map[string]any{
				"finalPrice": map[string]any{"amount": 150.0},
				"blockId":    map[string]any{"roomId": "101"}, // duplicate roomId
			},
		},
	}
	maxPrice, roomCount := extractBlocksPriceSpread(raw)
	if maxPrice != 280 {
		t.Errorf("maxPrice = %v, want 280", maxPrice)
	}
	if roomCount != 2 {
		t.Errorf("roomCount = %d, want 2", roomCount)
	}
}

func TestExtractBlocksPriceSpread_NoBlocks(t *testing.T) {
	raw := map[string]any{}
	maxPrice, roomCount := extractBlocksPriceSpread(raw)
	if maxPrice != 0 || roomCount != 0 {
		t.Errorf("got (%v, %d), want (0, 0)", maxPrice, roomCount)
	}
}

func TestExtractBlocksPriceSpread_EmptyBlocks(t *testing.T) {
	raw := map[string]any{
		"blocks": []any{},
	}
	maxPrice, roomCount := extractBlocksPriceSpread(raw)
	if maxPrice != 0 || roomCount != 0 {
		t.Errorf("got (%v, %d), want (0, 0)", maxPrice, roomCount)
	}
}

// ============================================================
// extractImageURL — deterministic JSON paths
// ============================================================

func TestExtractImageURL_HighResRelative(t *testing.T) {
	raw := map[string]any{
		"basicPropertyData": map[string]any{
			"photos": map[string]any{
				"main": map[string]any{
					"highResUrl": map[string]any{
						"relativeUrl": "/images/hotel/max1024x768/123.jpg",
					},
				},
			},
		},
	}
	got := extractImageURL(raw)
	if got != "https://cf.bstatic.com/images/hotel/max1024x768/123.jpg" {
		t.Errorf("got %q", got)
	}
}

func TestExtractImageURL_HighResString(t *testing.T) {
	raw := map[string]any{
		"basicPropertyData": map[string]any{
			"photos": map[string]any{
				"main": map[string]any{
					"highResUrl": "https://example.com/photo.jpg",
				},
			},
		},
	}
	got := extractImageURL(raw)
	if got != "https://example.com/photo.jpg" {
		t.Errorf("got %q", got)
	}
}

func TestExtractImageURL_LowResFallback(t *testing.T) {
	raw := map[string]any{
		"basicPropertyData": map[string]any{
			"photos": map[string]any{
				"main": map[string]any{
					"lowResUrl": "https://example.com/thumb.jpg",
				},
			},
		},
	}
	got := extractImageURL(raw)
	if got != "https://example.com/thumb.jpg" {
		t.Errorf("got %q", got)
	}
}

func TestExtractImageURL_NoPhoto(t *testing.T) {
	raw := map[string]any{}
	got := extractImageURL(raw)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ============================================================
// extractDescription — deterministic JSON paths
// ============================================================

func TestExtractDescription_PropertyDescription(t *testing.T) {
	raw := map[string]any{
		"propertyDescription": "A lovely boutique hotel",
	}
	got := extractDescription(raw)
	if got != "A lovely boutique hotel" {
		t.Errorf("got %q", got)
	}
}

func TestExtractDescription_Tagline(t *testing.T) {
	raw := map[string]any{
		"basicPropertyData": map[string]any{
			"tagline": "Budget-friendly in the city center",
		},
	}
	got := extractDescription(raw)
	if got != "Budget-friendly in the city center" {
		t.Errorf("got %q", got)
	}
}

func TestExtractDescription_Empty(t *testing.T) {
	raw := map[string]any{}
	got := extractDescription(raw)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ============================================================
// jsonPath — wildcard segment, array traversal
// ============================================================

func TestJsonPath_SimpleNested(t *testing.T) {
	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "hello",
			},
		},
	}
	got, ok := jsonPath(data, "a.b.c").(string)
	if !ok || got != "hello" {
		t.Errorf("got %v", got)
	}
}

func TestJsonPath_EmptyPath(t *testing.T) {
	data := "hello"
	got := jsonPath(data, "")
	if got != "hello" {
		t.Errorf("got %v", got)
	}
}

func TestJsonPath_MissingKey(t *testing.T) {
	data := map[string]any{"a": "b"}
	got := jsonPath(data, "x.y")
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestJsonPath_WildcardSegment(t *testing.T) {
	data := map[string]any{
		"searchQueries": map[string]any{
			"search({input:123})": map[string]any{
				"results": []any{"r1", "r2"},
			},
		},
	}
	got := jsonPath(data, "searchQueries.search*.results")
	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Errorf("got %v, want [r1, r2]", got)
	}
}

func TestJsonPath_ArrayTraversal(t *testing.T) {
	data := map[string]any{
		"sections": []any{
			map[string]any{"listings": []any{}},         // empty
			map[string]any{"listings": []any{"a", "b"}}, // non-empty
		},
	}
	got := jsonPath(data, "sections.listings")
	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Errorf("got %v, want [a, b]", got)
	}
}

// ============================================================
// Registry.ReloadIfChanged — file modified after load
// ============================================================

func TestRegistry_ReloadIfChanged_FileModified(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}
	cfg := &ProviderConfig{
		ID: "ric-mod", Name: "Original", Category: "hotel",
		Endpoint:        "https://api.example.com",
		ResponseMapping: ResponseMapping{ResultsPath: "r"},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Modify file directly to simulate external edit.
	cfg.Name = "Modified"
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save modified: %v", err)
	}

	got := reg.ReloadIfChanged("ric-mod")
	if got == nil {
		t.Fatal("expected non-nil config")
	}
	if got.Name != "Modified" {
		t.Errorf("Name = %q, want 'Modified'", got.Name)
	}
}
