package forecast

// MIK-3084: empirical-CDF price forecast tests.
//
// Each test guards a single behaviour. The constants in the package are
// stable enough that callers may rely on the exact thresholds, so the
// tests pin them rather than tolerating drift.

import (
	"math"
	"testing"
	"time"
)

func TestForecastAgainst_InsufficientHistoryReturnsNeutral(t *testing.T) {
	samples := make([]float64, MinSamplesForForecast-1)
	for i := range samples {
		samples[i] = float64(100 + i)
	}
	got := ForecastAgainst(150, samples, 7)
	if !got.InsufficientData {
		t.Fatalf("InsufficientData = false, want true")
	}
	if got.CommitNowConfidence != 50 {
		t.Errorf("CommitNowConfidence = %d, want 50", got.CommitNowConfidence)
	}
	if got.HorizonDays != 7 {
		t.Errorf("HorizonDays = %d, want 7", got.HorizonDays)
	}
	if got.ExpectedSavingsIfWait != 0 {
		t.Errorf("ExpectedSavingsIfWait = %v, want 0", got.ExpectedSavingsIfWait)
	}
}

func TestForecastAgainst_CurrentBelowAllReturnsCommitNowMax(t *testing.T) {
	samples := make([]float64, 30)
	for i := range samples {
		samples[i] = 200 + float64(i) // 200..229
	}
	got := ForecastAgainst(150, samples, 7)
	if got.InsufficientData {
		t.Fatalf("InsufficientData = true, want false (have %d samples)", len(samples))
	}
	if got.DropProbability != 0 {
		t.Errorf("DropProbability = %v, want 0", got.DropProbability)
	}
	if got.HorizonProbability != 0 {
		t.Errorf("HorizonProbability = %v, want 0", got.HorizonProbability)
	}
	if got.CommitNowConfidence != 100 {
		t.Errorf("CommitNowConfidence = %d, want 100", got.CommitNowConfidence)
	}
	if got.ExpectedSavingsIfWait != 0 {
		t.Errorf("ExpectedSavingsIfWait = %v, want 0", got.ExpectedSavingsIfWait)
	}
	if got.Reason == "" {
		t.Errorf("Reason should not be empty")
	}
}

func TestForecastAgainst_CurrentAboveAllReturnsCommitNowZero(t *testing.T) {
	samples := make([]float64, 30)
	for i := range samples {
		samples[i] = 100 + float64(i) // 100..129
	}
	got := ForecastAgainst(500, samples, 7)
	if got.DropProbability != 1 {
		t.Errorf("DropProbability = %v, want 1.0", got.DropProbability)
	}
	if got.HorizonProbability != 1 {
		t.Errorf("HorizonProbability = %v, want 1.0", got.HorizonProbability)
	}
	if got.CommitNowConfidence != 0 {
		t.Errorf("CommitNowConfidence = %d, want 0", got.CommitNowConfidence)
	}
	if got.ExpectedSavingsIfWait <= 0 {
		t.Errorf("ExpectedSavingsIfWait = %v, want > 0", got.ExpectedSavingsIfWait)
	}
	// Sanity: meanBelow = mean(100..129) = 114.5. gap = 500 - 114.5 = 385.5.
	// horizonProb = 1, so expected = 385.5 exactly.
	want := 500 - 114.5
	if math.Abs(got.ExpectedSavingsIfWait-want) > 0.01 {
		t.Errorf("ExpectedSavingsIfWait = %v, want ~%v", got.ExpectedSavingsIfWait, want)
	}
}

func TestForecastAgainst_HorizonScalesProbability(t *testing.T) {
	// 30 samples, half below current → dropProb = 0.5.
	samples := make([]float64, 30)
	for i := 0; i < 15; i++ {
		samples[i] = 100
	}
	for i := 15; i < 30; i++ {
		samples[i] = 200
	}
	short := ForecastAgainst(150, samples, 1)
	long := ForecastAgainst(150, samples, 7)
	if short.DropProbability != 0.5 {
		t.Errorf("dropProb = %v, want 0.5", short.DropProbability)
	}
	if short.HorizonProbability >= long.HorizonProbability {
		t.Errorf("horizonProb should grow with horizon: short=%v long=%v", short.HorizonProbability, long.HorizonProbability)
	}
	wantLong := 1 - math.Pow(0.5, 7)
	if math.Abs(long.HorizonProbability-wantLong) > 1e-9 {
		t.Errorf("long horizon prob = %v, want %v", long.HorizonProbability, wantLong)
	}
}

func TestForecastAgainst_NonPositiveHorizonClampedToOne(t *testing.T) {
	samples := make([]float64, 30)
	for i := range samples {
		samples[i] = 100
	}
	got := ForecastAgainst(120, samples, 0)
	if got.HorizonDays != 1 {
		t.Errorf("HorizonDays = %d, want 1 (clamped from 0)", got.HorizonDays)
	}
}

func TestForecastAgainst_StrictInequalityIgnoresEqualPrices(t *testing.T) {
	// All 30 samples exactly at current price → dropProb = 0 because we
	// only count strictly-below as "saw a better price".
	samples := make([]float64, 30)
	for i := range samples {
		samples[i] = 200
	}
	got := ForecastAgainst(200, samples, 7)
	if got.DropProbability != 0 {
		t.Errorf("DropProbability = %v, want 0 (strict <)", got.DropProbability)
	}
	if got.CommitNowConfidence != 100 {
		t.Errorf("CommitNowConfidence = %d, want 100", got.CommitNowConfidence)
	}
}

func TestForecastAgainst_PartialBelowProducesProportionalSavings(t *testing.T) {
	// 30 samples: 6 at 100 (below), 24 at 200 (at-or-above) → dropProb 0.2.
	samples := make([]float64, 0, 30)
	for i := 0; i < 6; i++ {
		samples = append(samples, 100)
	}
	for i := 0; i < 24; i++ {
		samples = append(samples, 200)
	}
	got := ForecastAgainst(180, samples, 7)
	if got.DropProbability != 0.2 {
		t.Errorf("DropProbability = %v, want 0.2", got.DropProbability)
	}
	// horizonProb = 1 - 0.8^7 ≈ 0.7903
	wantHorizon := 1 - math.Pow(0.8, 7)
	if math.Abs(got.HorizonProbability-wantHorizon) > 1e-9 {
		t.Errorf("HorizonProbability = %v, want %v", got.HorizonProbability, wantHorizon)
	}
	// gap = 180-100 = 80 → expected = 80*0.7903 ≈ 63.22
	wantExpected := 80 * wantHorizon
	if math.Abs(got.ExpectedSavingsIfWait-wantExpected) > 1e-6 {
		t.Errorf("ExpectedSavingsIfWait = %v, want %v", got.ExpectedSavingsIfWait, wantExpected)
	}
}

func TestHistogramOf_EmptyReturnsZeroValue(t *testing.T) {
	c := HistogramOf(nil, 10, 0)
	if c.Buckets != nil {
		t.Errorf("Buckets = %v, want nil", c.Buckets)
	}
}

func TestHistogramOf_FlatDistributionUsesSingleBucket(t *testing.T) {
	c := HistogramOf([]float64{100, 100, 100}, 10, 100)
	if len(c.Buckets) != 1 {
		t.Errorf("len(Buckets) = %d, want 1 (degenerate)", len(c.Buckets))
	}
	if c.Buckets[0] != 3 {
		t.Errorf("Buckets[0] = %d, want 3", c.Buckets[0])
	}
}

func TestHistogramOf_BucketCountsSumToInputLen(t *testing.T) {
	samples := []float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190}
	c := HistogramOf(samples, 5, 145)
	total := 0
	for _, b := range c.Buckets {
		total += b
	}
	if total != len(samples) {
		t.Errorf("bucket sum = %d, want %d", total, len(samples))
	}
	if c.Min != 100 || c.Max != 190 {
		t.Errorf("Min/Max = %v/%v, want 100/190", c.Min, c.Max)
	}
	if c.Threshold != 145 {
		t.Errorf("Threshold = %v, want 145", c.Threshold)
	}
}

func TestHistogramOf_BucketsBelowTwoClampedToTwo(t *testing.T) {
	c := HistogramOf([]float64{1, 2, 3, 4}, 1, 0)
	if len(c.Buckets) != 2 {
		t.Errorf("len(Buckets) = %d, want 2 (clamped)", len(c.Buckets))
	}
}

func TestSeasonOf_QuartersMatchDealquality(t *testing.T) {
	cases := map[string]string{
		"2026-01-15": "Q1",
		"2026-04-15": "Q2",
		"2026-07-15": "Q3",
		"2026-12-15": "Q4",
	}
	for date, want := range cases {
		ts, _ := time.Parse("2006-01-02", date)
		if got := SeasonOf(ts); got != want {
			t.Errorf("SeasonOf(%s) = %q, want %q", date, got, want)
		}
	}
}
