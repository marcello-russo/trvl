// Package opportunity composes the deal-quality, request-match, and
// profile-match signals into the single overall_score the
// opportunity-watch type uses to decide whether to fire a
// notification. Pure function — no I/O, no scheduler — so the math
// can be unit-tested without spinning up the watcher loop. The
// scheduler / Watch-type extension / MCP / CLI surfaces compose on
// top of this in a follow-up against the same Score signature.
//
// MIK-3065 (partial). Builds on the foundation packages this
// session shipped (internal/match, internal/dealquality, plus the
// implied profile-match signal which lives in the calendar+history
// adapter that ranks calendar-free windows against favourites).
package opportunity

import (
	"fmt"
	"sort"
	"strings"
)

// Signals carries the three signal scores the AC formula references.
// All three are 0..100 — callers normalise from the source packages
// (dealquality.Score.Total maps directly; match.Compute returns a
// 0..1 ratio that the caller scales by 100; profile-match is the
// adapter's calendar+history fitness signal also rendered 0..100).
type Signals struct {
	ProfileMatch float64
	RequestMatch float64
	DealQuality  float64
}

// Weights matches the AC: 0.4·ProfileMatch + 0.2·RequestMatch + 0.4·DealQuality.
// Exposed as a struct so callers / tests can inject alternative
// weights without re-implementing the linear combination.
type Weights struct {
	ProfileMatch float64
	RequestMatch float64
	DealQuality  float64
}

// DefaultWeights returns the AC-mandated weighting.
func DefaultWeights() Weights {
	return Weights{ProfileMatch: 0.4, RequestMatch: 0.2, DealQuality: 0.4}
}

// Candidate is one (destination, depart, return, computed-signals)
// tuple the scheduler enumerated. After Score it carries Overall and
// Reason so the caller can render or notify without re-running math.
type Candidate struct {
	Destination string
	DepartDate  string
	ReturnDate  string
	Nights      int
	Signals     Signals
	Overall     float64
	Reason      string
}

// Score computes the weighted Overall score and a one-line Reason
// for one Candidate. Pure function. Returns the candidate with
// Overall + Reason populated; original Signals are left untouched
// so callers can re-score under different weights without losing
// the input fidelity.
func Score(c Candidate, w Weights) Candidate {
	if w == (Weights{}) {
		w = DefaultWeights()
	}
	c.Overall = clamp01x100(w.ProfileMatch*c.Signals.ProfileMatch +
		w.RequestMatch*c.Signals.RequestMatch +
		w.DealQuality*c.Signals.DealQuality)
	c.Reason = describe(c.Signals, c.Overall)
	return c
}

func clamp01x100(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func describe(s Signals, overall float64) string {
	switch {
	case overall >= 95:
		return fmt.Sprintf("unicorn opportunity (overall %.0f, deal %.0f, profile %.0f)", overall, s.DealQuality, s.ProfileMatch)
	case overall >= 85:
		return fmt.Sprintf("excellent fit + deal (overall %.0f)", overall)
	case overall >= 70:
		return fmt.Sprintf("solid candidate (overall %.0f)", overall)
	default:
		return fmt.Sprintf("below-threshold (overall %.0f)", overall)
	}
}

// FilterAndRank scores every candidate, drops those below minScore,
// and returns the survivors sorted by Overall descending. minScore
// of 0 keeps every candidate (useful for inspect/debug surfaces).
// w==zero-value triggers DefaultWeights so callers can pass
// Weights{} for "AC defaults".
func FilterAndRank(candidates []Candidate, w Weights, minScore float64) []Candidate {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		scored := Score(c, w)
		if scored.Overall < minScore {
			continue
		}
		out = append(out, scored)
	}
	if len(out) == 0 {
		return nil
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Overall > out[j].Overall
	})
	return out
}

// FavouritesFromPreferences resolves the AC's "BucketList ∪
// PreviousTrips ∩ AirportAffinity≥3" rule into a deduplicated IATA
// slice. Pure helper exposed so the watcher can populate the
// opportunity Watch's Favourites field without re-implementing the
// set algebra. All inputs are case-insensitive; output is sorted +
// uppercased so storage stays deterministic.
func FavouritesFromPreferences(bucketList, previousTrips []string, airportAffinity map[string]int, minAffinity int) []string {
	if minAffinity < 1 {
		minAffinity = 3
	}
	seen := map[string]struct{}{}
	add := func(code string) {
		k := strings.ToUpper(strings.TrimSpace(code))
		if k == "" {
			return
		}
		seen[k] = struct{}{}
	}
	// Union: bucket list always counts.
	for _, c := range bucketList {
		add(c)
	}
	// PreviousTrips contribute only when AirportAffinity meets the
	// minimum — the AC's "∩ AirportAffinity≥3" intersection.
	for _, c := range previousTrips {
		k := strings.ToUpper(strings.TrimSpace(c))
		if affinityFor(airportAffinity, k) >= minAffinity {
			add(k)
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func affinityFor(m map[string]int, code string) int {
	if v, ok := m[code]; ok {
		return v
	}
	if v, ok := m[strings.ToUpper(code)]; ok {
		return v
	}
	if v, ok := m[strings.ToLower(code)]; ok {
		return v
	}
	return 0
}
