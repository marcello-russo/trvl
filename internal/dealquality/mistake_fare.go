package dealquality

// MIK-3085: mistake-fare detector layered on top of the existing
// per-route percentile baselines.
//
// A mistake fare is a price that is so far below the route's recent
// median that it is almost certainly an airline pricing error. We
// surface a 0..100 confidence score on every priced result and a
// "should alert now?" decision that respects a 24h per-route decay so
// the daemon does not re-page the user every 30 minutes during the
// short window before the airline pulls the fare.

import (
	"strings"
	"time"
)

// MistakeFareConfidence levels.
const (
	// MistakeFareThresholdRatio is the price/median ratio at which a
	// fare crosses into mistake territory. AC: <= 50% of median.
	MistakeFareThresholdRatio = 0.5
	// MistakeFareMinSamples is the minimum sample count required before
	// the detector trusts the median enough to fire.
	MistakeFareMinSamples = 10
	// MistakeFareDecayWindow is how long the alert is suppressed for the
	// same (route, kind) tuple after a previous alert fires.
	MistakeFareDecayWindow = 24 * time.Hour
)

// MistakeFare is the detector verdict for one priced result.
type MistakeFare struct {
	// Confidence is 0..100. >=90 means the daemon should alert NOW
	// (subject to the 24h decay window). 0 means "not a mistake fare".
	Confidence int
	// Median is the route median over the same sample slice — exposed
	// so callers can render "EUR 50 vs EUR 200 median" in the alert.
	Median float64
	// Samples is the sample count that backed this verdict.
	Samples int
	// Reason is a short user-facing tag.
	Reason string
}

// MistakeFareConfidence is the pure scoring entry point. Pre-filter
// `samples` to the same route+season+kind before calling.
func MistakeFareConfidence(price float64, samples []float64) MistakeFare {
	if len(samples) < MistakeFareMinSamples {
		return MistakeFare{Samples: len(samples), Reason: "insufficient history"}
	}
	median := percentileSorted(samples, 0.5)
	out := MistakeFare{Median: median, Samples: len(samples)}
	if median <= 0 || price > median*MistakeFareThresholdRatio {
		return out
	}
	ratio := price / median
	switch {
	case ratio <= 0.30:
		out.Confidence = 100
	case ratio <= 0.40:
		// Linear from 95 (at 0.40) to 100 (at 0.30).
		out.Confidence = lerp(ratio, 0.40, 0.30, 95, 100)
	case ratio <= 0.50:
		// Linear from 90 (at 0.50) to 95 (at 0.40).
		out.Confidence = lerp(ratio, 0.50, 0.40, 90, 95)
	}
	if out.Confidence > 0 {
		out.Reason = "price <= 50% of route median over last 90 days"
	}
	return out
}

// percentileSorted is a copy-then-sort wrapper around the package's
// internal percentile() so callers do not need to pre-sort.
func percentileSorted(samples []float64, q float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	// Reusing the helper from dealquality.go via the same package.
	sortFloat64s(sorted)
	return percentile(sorted, q)
}

// sortFloat64s exists so the test file does not need to import "sort"
// just to seed pre-sorted samples.
func sortFloat64s(s []float64) {
	// Insertion sort is fine — sample slices are small (≤200 typical).
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// alertKey is the (route, kind) lookup key used in the alert decay map.
func alertKey(route, kind string) string {
	return strings.ToLower(strings.TrimSpace(route)) + "|" + strings.ToLower(strings.TrimSpace(kind))
}

// MaybeAlertMistakeFare consults the store's history for the matching
// route+season+kind, computes the mistake-fare confidence, and decides
// whether the daemon should send an alert NOW. The return value's
// `ShouldAlert` is true only when:
//
//   - confidence >= 90, AND
//   - more than MistakeFareDecayWindow has elapsed since the last
//     alert for the same (route, kind) tuple.
//
// Callers that send an alert MUST then call Store.MarkAlerted to record
// the decay window. Decoupling decision from acknowledgement keeps the
// detector pure and lets dry-run callers inspect without resetting.
func (s *Store) MaybeAlertMistakeFare(route, kind string, tripDate time.Time, price float64, now time.Time) (mf MistakeFare, shouldAlert bool) {
	prices := s.Query(route, kind, SeasonOf(tripDate))
	mf = MistakeFareConfidence(price, prices)
	if mf.Confidence < 90 {
		return mf, false
	}
	s.mu.Lock()
	last, seen := s.alerted[alertKey(route, kind)]
	s.mu.Unlock()
	if seen && now.Sub(last) < MistakeFareDecayWindow {
		return mf, false
	}
	return mf, true
}

// MarkAlerted records that the daemon has sent an alert for the
// (route, kind) tuple at time `t`. Persists immediately so a daemon
// restart inside the decay window still suppresses duplicate pings.
func (s *Store) MarkAlerted(route, kind string, t time.Time) error {
	s.mu.Lock()
	if s.alerted == nil {
		s.alerted = make(map[string]time.Time)
	}
	s.alerted[alertKey(route, kind)] = t
	defer s.mu.Unlock()
	return s.saveLocked()
}

// LastAlertedAt returns the time the (route, kind) tuple was last
// alerted on, plus a presence flag. Used by tests and admin tooling.
func (s *Store) LastAlertedAt(route, kind string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.alerted[alertKey(route, kind)]
	return t, ok
}
