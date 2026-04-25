// Package dealquality computes a 0..100 score that quantifies how good
// an offered price is relative to recent history for the same route +
// season. Used by the opportunity watcher (MIK-3065) to fire only on
// genuine deals (filter out routine prices that the user doesn't want
// pinged about).
//
// The scoring is intentionally simple — empirical CDF over a 90-day
// rolling window per route+season+kind — so it stays explainable and
// resists the obvious failure mode of overfitting tiny samples.
package dealquality

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Sample is one observed price point. Route is "ORIG-DEST" (IATA codes
// for flights, city slugs for hotels — opaque to this package). Kind is
// "flight" or "hotel" so the percentiles never mix the two markets.
type Sample struct {
	Route     string    `json:"route"`
	Kind      string    `json:"kind"`
	Date      string    `json:"date"` // ISO 8601 calendar date of the trip
	Price     float64   `json:"price"`
	Currency  string    `json:"currency"`
	Season    string    `json:"season"` // "Q1".."Q4"
	Timestamp time.Time `json:"ts"`     // when we observed this price
}

// Score is the computed deal-quality verdict for one priced offer.
type Score struct {
	// Total is 0..100. 95-100 = unicorn (≤p10), 80-95 = excellent (≤p20),
	// 50-80 = average (≤p50), <50 = above-median (linear ramp to 0).
	Total int
	// Reason describes the classification in human-readable form.
	Reason string
	// Samples is the count of samples that backed the percentile bucket.
	// Sparse data (<10) forces a neutral score regardless of price.
	Samples int
	// P10/P20/P50 are the percentile thresholds derived from the matching
	// samples. Zero when Samples < 10.
	P10, P20, P50 float64
}

// Constants chosen to match the AC bands (≤p10 → 95-100, ≤p20 → 80-95,
// ≤p50 → 50-80, >p50 → ramp to 0).
const (
	// MinSamplesForScoring is the minimum sample count required before
	// we trust the percentiles. Below this we return a neutral 50.
	MinSamplesForScoring = 10
	// HistoryWindowDays is how far back we keep samples in memory and
	// on disk. Older samples are pruned at every Append call.
	HistoryWindowDays = 90
	// neutralScore is returned when sample count is below the threshold.
	neutralScore = 50
)

// SeasonOf returns the calendar quarter ("Q1".."Q4") for the given date.
// Pure helper exposed for tests and CLI use.
func SeasonOf(t time.Time) string {
	switch m := t.Month(); {
	case m <= time.March:
		return "Q1"
	case m <= time.June:
		return "Q2"
	case m <= time.September:
		return "Q3"
	default:
		return "Q4"
	}
}

// Score classifies `price` against `samples` (which the caller is
// responsible for pre-filtering to the right route+season+kind). The
// pure function — no I/O — so it stays cheap to call inline on every
// returned result.
func ScoreAgainst(price float64, samples []float64) Score {
	if len(samples) < MinSamplesForScoring {
		return Score{Total: neutralScore, Reason: "insufficient history", Samples: len(samples)}
	}
	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)
	p10 := percentile(sorted, 0.10)
	p20 := percentile(sorted, 0.20)
	p50 := percentile(sorted, 0.50)

	out := Score{Samples: len(sorted), P10: p10, P20: p20, P50: p50}
	switch {
	case price <= p10:
		// 95..100; lower price scores higher. Anchor: at p10 exactly → 95;
		// price → 0 → 100.
		out.Total = lerp(price, p10, 0, 95, 100)
		out.Reason = fmt.Sprintf("unicorn deal — price ≤ p10 (%.0f vs %.0f)", price, p10)
	case price <= p20:
		out.Total = lerp(price, p20, p10, 80, 95)
		out.Reason = fmt.Sprintf("excellent deal — price ≤ p20 (%.0f vs %.0f)", price, p20)
	case price <= p50:
		out.Total = lerp(price, p50, p20, 50, 80)
		out.Reason = fmt.Sprintf("good deal — price ≤ p50 (%.0f vs %.0f)", price, p50)
	default:
		// Above median: linear ramp from 50 (at p50) toward 0 (at 2x p50).
		ceil := p50 * 2
		if ceil <= p50 {
			ceil = p50 + 1
		}
		out.Total = lerp(price, p50, ceil, 50, 0)
		out.Reason = fmt.Sprintf("above median — price > p50 (%.0f vs %.0f)", price, p50)
	}
	if out.Total < 0 {
		out.Total = 0
	}
	if out.Total > 100 {
		out.Total = 100
	}
	return out
}

// percentile returns the linear-interpolation percentile at q (0..1)
// over a pre-sorted ascending slice. Caller guarantees len > 0.
func percentile(sorted []float64, q float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(pos)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := pos - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}

// lerp maps a value `v` from input range [a..b] to output range
// [outA..outB]. Endpoints are clamped. Returns the integer-rounded
// result for clean Score.Total emission.
func lerp(v, a, b, outA, outB float64) int {
	if a == b {
		return int(outA)
	}
	t := (v - a) / (b - a)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	out := outA + t*(outB-outA)
	return int(out + 0.5)
}

// Store persists samples to ~/.trvl/deal-history.json. Append, Query,
// and Score are goroutine-safe.
type Store struct {
	mu      sync.Mutex
	path    string
	samples []Sample
	// alerted tracks the last time MIK-3085 sent a mistake-fare alert
	// for a given (route, kind) tuple. Used by MaybeAlertMistakeFare's
	// 24h decay rule. Persisted alongside samples in deal-history.json.
	alerted map[string]time.Time
}

// NewStore constructs a Store rooted at the given file path. If the
// file exists it is loaded; missing → empty store.
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// DefaultStore returns a Store at ~/.trvl/deal-history.json.
func DefaultStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return NewStore(filepath.Join(home, ".trvl", "deal-history.json"))
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("dealquality: read %s: %w", s.path, err)
	}
	if len(data) == 0 {
		return nil
	}
	var wrapper struct {
		Samples []Sample             `json:"samples"`
		Alerted map[string]time.Time `json:"mistake_alerts,omitempty"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("dealquality: parse %s: %w", s.path, err)
	}
	s.samples = wrapper.Samples
	s.alerted = wrapper.Alerted
	return nil
}

// Save writes the current samples to disk atomically (write-temp + rename).
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("dealquality: mkdir: %w", err)
	}
	tmp := s.path + ".tmp"
	wrapper := struct {
		Samples []Sample             `json:"samples"`
		Alerted map[string]time.Time `json:"mistake_alerts,omitempty"`
	}{Samples: s.samples, Alerted: s.alerted}
	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("dealquality: marshal: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("dealquality: write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("dealquality: rename: %w", err)
	}
	return nil
}

// Append records one sample, prunes anything older than HistoryWindowDays,
// and persists. Idempotent within the same (route, kind, date, price)
// tuple emitted in the same Append call (de-dup avoids the watch
// scheduler hammering the same observation N times per hour).
func (s *Store) Append(sample Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sample.Timestamp.IsZero() {
		sample.Timestamp = time.Now().UTC()
	}
	if sample.Season == "" && sample.Date != "" {
		if t, err := time.Parse("2006-01-02", sample.Date); err == nil {
			sample.Season = SeasonOf(t)
		}
	}
	// De-dup against the most recent matching tuple.
	if n := len(s.samples); n > 0 {
		last := s.samples[n-1]
		if last.Route == sample.Route && last.Kind == sample.Kind &&
			last.Date == sample.Date && last.Price == sample.Price {
			return nil
		}
	}
	s.samples = append(s.samples, sample)
	s.prune(time.Now().UTC())
	return s.saveLocked()
}

// prune drops samples whose Timestamp is older than HistoryWindowDays
// before `now`. Caller holds s.mu.
func (s *Store) prune(now time.Time) {
	cutoff := now.AddDate(0, 0, -HistoryWindowDays)
	kept := s.samples[:0]
	for _, sm := range s.samples {
		if !sm.Timestamp.Before(cutoff) {
			kept = append(kept, sm)
		}
	}
	s.samples = kept
}

// Query returns prices for the matching route+season+kind. Pure read
// path; safe for concurrent callers.
func (s *Store) Query(route, kind, season string) []float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []float64
	for _, sm := range s.samples {
		if sm.Route == route && sm.Kind == kind && sm.Season == season {
			out = append(out, sm.Price)
		}
	}
	return out
}

// Score is the convenience wrapper that pulls the relevant samples from
// the Store and runs ScoreAgainst.
func (s *Store) Score(route, kind string, tripDate time.Time, price float64) Score {
	prices := s.Query(route, kind, SeasonOf(tripDate))
	return ScoreAgainst(price, prices)
}
