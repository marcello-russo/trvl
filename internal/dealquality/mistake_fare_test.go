package dealquality

// MIK-3085: tests for the mistake-fare detector + alert decay window.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMistakeFareConfidence_SparseHistoryReturnsZero(t *testing.T) {
	// Below the min-samples bar, confidence MUST be 0 — even when the
	// price is dramatically below the median of the few samples we have.
	for n := 0; n < MistakeFareMinSamples; n++ {
		samples := make([]float64, n)
		for i := range samples {
			samples[i] = 200
		}
		got := MistakeFareConfidence(10, samples)
		if got.Confidence != 0 {
			t.Errorf("n=%d: confidence = %d, want 0 (sparse history)", n, got.Confidence)
		}
	}
}

func TestMistakeFareConfidence_AboveThresholdReturnsZero(t *testing.T) {
	// 30 samples around 200; price 150 = 75% of median → not a mistake.
	samples := make([]float64, 30)
	for i := range samples {
		samples[i] = 200
	}
	got := MistakeFareConfidence(150, samples)
	if got.Confidence != 0 {
		t.Errorf("price 150 vs median 200 (75%%): confidence = %d, want 0", got.Confidence)
	}
	if got.Median != 200 {
		t.Errorf("Median = %v, want 200", got.Median)
	}
}

func TestMistakeFareConfidence_BandedRanges(t *testing.T) {
	// 30 samples around 200 → median = 200.
	samples := make([]float64, 30)
	for i := range samples {
		samples[i] = 200
	}
	cases := []struct {
		name     string
		price    float64
		minConf  int
		maxConf  int
		wantTag  string
	}{
		{"at_50pct", 100, 90, 90, "price <="},
		{"between_50_and_40", 90, 90, 95, "price <="},
		{"at_40pct", 80, 95, 95, "price <="},
		{"between_40_and_30", 70, 95, 100, "price <="},
		{"at_30pct", 60, 100, 100, "price <="},
		{"way_below", 10, 100, 100, "price <="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MistakeFareConfidence(tc.price, samples)
			if got.Confidence < tc.minConf || got.Confidence > tc.maxConf {
				t.Errorf("price=%v: confidence = %d, want [%d..%d]", tc.price, got.Confidence, tc.minConf, tc.maxConf)
			}
			if !strings.Contains(got.Reason, tc.wantTag) {
				t.Errorf("price=%v: Reason = %q, want phrase %q", tc.price, got.Reason, tc.wantTag)
			}
		})
	}
}

func TestMistakeFareConfidence_ZeroOrNegativeMedian(t *testing.T) {
	// Pathological: 10 zero-price samples → median 0 → never fires.
	samples := make([]float64, 10)
	got := MistakeFareConfidence(0, samples)
	if got.Confidence != 0 {
		t.Errorf("zero median: confidence = %d, want 0", got.Confidence)
	}
}

func seedStoreForRoute(t *testing.T, store *Store, route, kind, season string, prices []float64) {
	t.Helper()
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	for i, p := range prices {
		err := store.Append(Sample{
			Route: route, Kind: kind, Season: season,
			Date:      "2026-05-15",
			Price:     p,
			Currency:  "EUR",
			Timestamp: now.Add(time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestStore_MaybeAlertMistakeFare_FiresAtConfidenceThreshold(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "deal-history.json"))
	if err != nil {
		t.Fatal(err)
	}
	// 20 distinct values clustered around 200 so the dedup-on-identical
	// branch in Append does not collapse them into a single sample.
	prices := make([]float64, 20)
	for i := range prices {
		prices[i] = 190 + float64(i)
	}
	seedStoreForRoute(t, store, "AMS-PRG", "flight", "Q2", prices)

	tripDate, _ := time.Parse("2006-01-02", "2026-05-15")
	now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)

	// Above-threshold price → should NOT alert.
	mf, ok := store.MaybeAlertMistakeFare("AMS-PRG", "flight", tripDate, 150, now)
	if ok {
		t.Errorf("price 150 (75%%) shouldAlert = true, want false")
	}
	if mf.Confidence != 0 {
		t.Errorf("confidence = %d, want 0", mf.Confidence)
	}

	// 30%-of-median price → should alert.
	mf, ok = store.MaybeAlertMistakeFare("AMS-PRG", "flight", tripDate, 60, now)
	if !ok {
		t.Errorf("price 60 (30%%) shouldAlert = false, want true")
	}
	if mf.Confidence < 90 {
		t.Errorf("confidence = %d, want >= 90", mf.Confidence)
	}
}

func TestStore_MaybeAlertMistakeFare_DecayWindowSuppresses(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "deal-history.json"))
	if err != nil {
		t.Fatal(err)
	}
	prices := make([]float64, 20)
	for i := range prices {
		prices[i] = 190 + float64(i)
	}
	seedStoreForRoute(t, store, "AMS-PRG", "flight", "Q2", prices)
	tripDate, _ := time.Parse("2006-01-02", "2026-05-15")

	t0 := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	// First call: should alert.
	if _, ok := store.MaybeAlertMistakeFare("AMS-PRG", "flight", tripDate, 60, t0); !ok {
		t.Fatal("first call: shouldAlert = false, want true")
	}
	// Daemon would now call MarkAlerted; simulate that.
	if err := store.MarkAlerted("AMS-PRG", "flight", t0); err != nil {
		t.Fatal(err)
	}

	// 12 hours later (within decay window) → suppress.
	t12 := t0.Add(12 * time.Hour)
	if _, ok := store.MaybeAlertMistakeFare("AMS-PRG", "flight", tripDate, 60, t12); ok {
		t.Errorf("12h later within %v decay: shouldAlert = true, want false", MistakeFareDecayWindow)
	}

	// 25 hours later (past decay window) → fire again.
	t25 := t0.Add(25 * time.Hour)
	if _, ok := store.MaybeAlertMistakeFare("AMS-PRG", "flight", tripDate, 60, t25); !ok {
		t.Errorf("25h later past %v decay: shouldAlert = false, want true", MistakeFareDecayWindow)
	}
}

func TestStore_MarkAlerted_PersistsAcrossReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deal-history.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC).UTC()
	if err := store.MarkAlerted("AMS-PRG", "flight", now); err != nil {
		t.Fatal(err)
	}

	// Reload from disk.
	store2, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := store2.LastAlertedAt("AMS-PRG", "flight")
	if !ok {
		t.Fatal("LastAlertedAt missing after reload")
	}
	if !got.Equal(now) {
		t.Errorf("LastAlertedAt = %v, want %v", got, now)
	}
}

func TestAlertKey_CaseFoldedAndTrimmed(t *testing.T) {
	if alertKey(" AMS-PRG ", "FLIGHT") != alertKey("ams-prg", "flight") {
		t.Errorf("alertKey not case/trim-folded")
	}
}
