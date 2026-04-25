package dealquality

// MIK-3064: tests for DealQuality scoring + Store persistence.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSeasonOf(t *testing.T) {
	cases := map[string]string{
		"2026-01-15": "Q1",
		"2026-03-31": "Q1",
		"2026-04-01": "Q2",
		"2026-06-30": "Q2",
		"2026-07-01": "Q3",
		"2026-09-30": "Q3",
		"2026-10-01": "Q4",
		"2026-12-31": "Q4",
	}
	for date, want := range cases {
		t.Run(date, func(t *testing.T) {
			tt, _ := time.Parse("2006-01-02", date)
			if got := SeasonOf(tt); got != want {
				t.Errorf("SeasonOf(%s) = %s, want %s", date, got, want)
			}
		})
	}
}

func TestScoreAgainst_SparseHistoryReturnsNeutral(t *testing.T) {
	for n := 0; n < MinSamplesForScoring; n++ {
		samples := make([]float64, n)
		for i := range samples {
			samples[i] = 100
		}
		got := ScoreAgainst(50, samples)
		if got.Total != 50 {
			t.Errorf("n=%d: Total = %d, want 50 (neutral)", n, got.Total)
		}
		if !strings.Contains(got.Reason, "insufficient history") {
			t.Errorf("n=%d: Reason = %q, want 'insufficient history'", n, got.Reason)
		}
	}
}

func TestScoreAgainst_DenseDistribution(t *testing.T) {
	// 100 samples, evenly spaced 100..199. p10≈109.9, p20≈119.8, p50≈149.5.
	samples := make([]float64, 100)
	for i := range samples {
		samples[i] = float64(100 + i)
	}

	cases := []struct {
		name       string
		price      float64
		minTotal   int
		maxTotal   int
		wantPhrase string
	}{
		{"unicorn_at_zero", 0, 100, 100, "unicorn"},
		{"unicorn_at_p10", 109, 95, 100, "unicorn"},
		{"excellent_just_above_p10", 115, 80, 95, "excellent"},
		{"good_at_p35", 134, 50, 80, "good"},
		{"above_median", 175, 0, 50, "above median"},
		{"way_above_median", 1000, 0, 0, "above median"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScoreAgainst(tc.price, samples)
			if got.Total < tc.minTotal || got.Total > tc.maxTotal {
				t.Errorf("price=%v: Total = %d, want [%d..%d]", tc.price, got.Total, tc.minTotal, tc.maxTotal)
			}
			if !strings.Contains(got.Reason, tc.wantPhrase) {
				t.Errorf("price=%v: Reason = %q, want phrase %q", tc.price, got.Reason, tc.wantPhrase)
			}
		})
	}
}

func TestScoreAgainst_OutlierLow(t *testing.T) {
	// 49 samples around 200, one outlier at 50.
	samples := make([]float64, 50)
	for i := range samples {
		samples[i] = 200
	}
	samples[0] = 50
	got := ScoreAgainst(60, samples)
	if got.Total < 95 {
		t.Errorf("outlier-bracket price 60: Total = %d, want >= 95 (≤ p10 because outlier dominates)", got.Total)
	}
}

func TestScoreAgainst_BoundaryClamps(t *testing.T) {
	samples := make([]float64, 20)
	for i := range samples {
		samples[i] = 100
	}
	// Identical samples → all percentiles = 100.
	got := ScoreAgainst(100, samples)
	if got.Total < 80 || got.Total > 100 {
		t.Errorf("constant 100s priced at 100: Total = %d, want >= 80 (≤ p20)", got.Total)
	}
}

func TestStore_AppendAndQuery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deal-history.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	prices := []float64{120, 130, 110, 100, 150}
	for i, p := range prices {
		err := store.Append(Sample{
			Route:    "AMS-PRG",
			Kind:     "flight",
			Date:     "2026-05-01",
			Price:    p,
			Currency: "EUR",
			// Make each sample unique by varying the timestamp; otherwise
			// the de-dup branch would drop sample[0] vs sample[N>1] when
			// they collide. Setting tiny offsets is fine — pruning
			// uses the same field.
			Timestamp: time.Date(2026, 4, 25, 12, i, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got := store.Query("AMS-PRG", "flight", "Q2")
	if len(got) != len(prices) {
		t.Errorf("query returned %d, want %d", len(got), len(prices))
	}

	// Reload from disk and query again.
	store2, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got2 := store2.Query("AMS-PRG", "flight", "Q2")
	if len(got2) != len(prices) {
		t.Errorf("after reload: %d samples, want %d", len(got2), len(prices))
	}
}

func TestStore_DedupesIdenticalConsecutive(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "deal-history.json"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		err := store.Append(Sample{
			Route: "AMS-PRG", Kind: "flight", Date: "2026-05-01",
			Price: 150, Currency: "EUR",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := store.Query("AMS-PRG", "flight", "Q2"); len(got) != 1 {
		t.Errorf("duplicate appends: %d kept, want 1", len(got))
	}
}

func TestStore_PrunesOldSamples(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "deal-history.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -120)
	store.samples = []Sample{
		{Route: "X", Kind: "flight", Season: "Q1", Price: 99, Timestamp: old},
		{Route: "X", Kind: "flight", Season: "Q1", Price: 99, Timestamp: now},
	}
	if err := store.Append(Sample{
		Route: "X", Kind: "flight", Season: "Q1",
		Date: "2026-02-01", Price: 100, Timestamp: now,
	}); err != nil {
		t.Fatal(err)
	}
	got := store.Query("X", "flight", "Q1")
	if len(got) != 2 {
		t.Errorf("after prune: %d samples, want 2 (old should be dropped)", len(got))
	}
}

func TestStore_ScoreEndToEnd(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "deal-history.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		_ = store.Append(Sample{
			Route: "AMS-PRG", Kind: "flight", Date: "2026-05-15",
			Price: float64(150 + i), Currency: "EUR",
			Timestamp: now.Add(time.Duration(i) * time.Minute),
		})
	}
	tripDate, _ := time.Parse("2006-01-02", "2026-05-15")
	score := store.Score("AMS-PRG", "flight", tripDate, 100)
	if score.Total < 95 {
		t.Errorf("price 100 vs samples 150..179: Total = %d, want >= 95 (unicorn)", score.Total)
	}
	if !strings.Contains(score.Reason, "unicorn") {
		t.Errorf("Reason = %q, want unicorn", score.Reason)
	}
}

func TestStore_ConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "deal-history.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = store.Append(Sample{
				Route: "AMS-PRG", Kind: "flight", Date: "2026-05-15",
				Price: float64(100 + i), Currency: "EUR",
				Timestamp: time.Now().UTC().Add(time.Duration(i) * time.Microsecond),
			})
		}(i)
	}
	wg.Wait()
	if got := len(store.Query("AMS-PRG", "flight", "Q2")); got < 1 {
		t.Errorf("concurrent appends produced 0 samples")
	}
}

func TestDefaultStore_RootsAtHome(t *testing.T) {
	s, err := DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore: %v", err)
	}
	if !strings.Contains(s.path, ".trvl") || !strings.HasSuffix(s.path, "deal-history.json") {
		t.Errorf("DefaultStore path = %q, want .trvl/deal-history.json", s.path)
	}
}

func TestStore_Save_ExplicitWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "explicit.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.samples = []Sample{
		{Route: "AMS-PRG", Kind: "flight", Season: "Q2", Price: 200, Timestamp: time.Now().UTC()},
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist after Save: %v", path, err)
	}
}

func TestNewStore_MalformedFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deal-history.json")
	if err := writeFile(path, "{not-json"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path); err == nil {
		t.Error("expected error from malformed file, got nil")
	}
}

// writeFile is a tiny test helper used by TestNewStore_MalformedFileReturnsError
// to seed a corrupt history file before NewStore reads it.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
