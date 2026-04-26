// Package forecast turns observed price history into a buy-now-vs-wait
// recommendation. It is the read-side counterpart to internal/dealquality:
// dealquality answers "is this price a good deal right now?", forecast
// answers "is it likely to get cheaper if we wait?".
//
// MIK-3084. Empirical CDF over the same 90-day rolling window that
// dealquality maintains; no ML, no time-series fits — explainable by
// hand from the inputs and intentionally honest about what it cannot
// do (sparse routes, structural breaks, fare-class-specific quirks).
package forecast

import (
	"fmt"
	"math"
	"time"
)

// MinSamplesForForecast is the lower bound on history before we trust
// the empirical CDF enough to publish a verdict. Below this we return
// InsufficientData=true and a neutral confidence so callers can choose
// to suppress the field rather than mislead the user.
const MinSamplesForForecast = 30

// DefaultHorizonDays matches the AC's 7-day "wait window".
const DefaultHorizonDays = 7

// HistoryWindowDays mirrors dealquality.HistoryWindowDays. Kept as a
// local constant rather than imported so the forecast package stays
// dependency-free of dealquality and can be reused with any sample
// source the caller provides.
const HistoryWindowDays = 90

// Forecast is the verdict for one priced offer.
type Forecast struct {
	// CommitNowConfidence is 0..100. 100 = lowest historical price seen,
	// commit immediately. 0 = priced above every historical observation,
	// almost certain to drop within the horizon.
	CommitNowConfidence int
	// DropProbability is the empirical probability that any randomly
	// drawn historical price was below the current quote — i.e. the
	// CDF value F(currentPrice). Range [0,1].
	DropProbability float64
	// HorizonProbability is P(at least one observation below current is
	// seen in HorizonDays), assuming independent daily samples drawn
	// from the empirical distribution. Range [0,1].
	HorizonProbability float64
	// ExpectedSavingsIfWait is in the same currency as the input quote.
	// Computed as (currentPrice - meanBelow) * HorizonProbability and
	// clamped to >= 0 so callers never present a "wait to save -20".
	ExpectedSavingsIfWait float64
	// HorizonDays records the horizon used so JSON consumers can avoid
	// guessing what window the savings figure refers to.
	HorizonDays int
	// Samples is the count fed to the empirical CDF. Matches len(samples)
	// the caller passed in and is exposed so dashboards can show "based
	// on N observations".
	Samples int
	// InsufficientData is true when len(samples) < MinSamplesForForecast.
	// Callers should suppress the recommendation in that case.
	InsufficientData bool
	// Reason is a short human-readable label that explains the verdict.
	// Stable enough to be matched in tests but not part of any API
	// contract — UI layers should re-render based on the numeric fields.
	Reason string
}

// Curve is a small histogram of the empirical price distribution that
// `trvl forecast` can render. Buckets are even-width over the observed
// [min,max] range. Pure data; rendering lives in the CLI layer.
type Curve struct {
	Min, Max  float64
	Buckets   []int
	Threshold float64 // currentPrice plotted against the curve.
}

// ForecastAgainst is the pure form: caller pre-filters samples to the
// route+kind+season that matches the current quote, passes the slice
// (unsorted is fine) and the current quote price. horizonDays controls
// the temporal lift on DropProbability; pass DefaultHorizonDays for the
// AC-mandated 7d figure.
//
// negative or zero samples are treated as observations and not skipped:
// the caller decides what is meaningful price input. We do guard
// against horizonDays <= 0 (treated as 1) so the caller cannot get a
// 0% horizon probability through a typo.
func ForecastAgainst(currentPrice float64, samples []float64, horizonDays int) Forecast {
	if horizonDays <= 0 {
		horizonDays = 1
	}
	out := Forecast{
		HorizonDays: horizonDays,
		Samples:     len(samples),
	}
	if len(samples) < MinSamplesForForecast {
		out.InsufficientData = true
		out.CommitNowConfidence = 50 // neutral so callers don't accidentally tilt either way
		out.Reason = fmt.Sprintf("insufficient history (%d/%d samples)", len(samples), MinSamplesForForecast)
		return out
	}

	// Empirical CDF: drop probability = fraction of samples strictly
	// below the current quote. Strict inequality matters when many
	// observations cluster at one price point (sale floors); we want
	// "saw a *better* price", not "saw the same price".
	below := 0
	var sumBelow float64
	for _, s := range samples {
		if s < currentPrice {
			below++
			sumBelow += s
		}
	}
	dropProb := float64(below) / float64(len(samples))

	// Treat dropProb as the per-day probability of seeing one drop, then
	// compound across horizonDays under the i.i.d. assumption. This is
	// the strongest defensible step in the v1 stack: it converts "how
	// often does this price level get beaten in a 90-day window" into
	// "what's the chance we see one within N more days of polling".
	horizonProb := 1 - math.Pow(1-dropProb, float64(horizonDays))
	if horizonProb < 0 {
		horizonProb = 0
	}
	if horizonProb > 1 {
		horizonProb = 1
	}

	// Expected savings: gap between current and the conditional mean of
	// observed-below prices, scaled by the horizon-aware drop chance.
	var gap, expected float64
	if below > 0 {
		meanBelow := sumBelow / float64(below)
		gap = currentPrice - meanBelow
		if gap < 0 {
			gap = 0
		}
		expected = gap * horizonProb
	}

	out.DropProbability = dropProb
	out.HorizonProbability = horizonProb
	out.ExpectedSavingsIfWait = expected
	out.CommitNowConfidence = clamp01ToScore(1 - horizonProb)
	out.Reason = describe(currentPrice, samples, below, horizonProb)
	return out
}

// HistogramOf produces a fixed-bucket histogram of `samples` for the
// CLI curve renderer. Returns Buckets nil when len(samples) is 0 so
// callers can short-circuit "no data" cases without inspecting fields.
// buckets must be >= 2 — values below 2 are clamped to 2 because a
// single bucket has no shape information.
func HistogramOf(samples []float64, buckets int, threshold float64) Curve {
	if buckets < 2 {
		buckets = 2
	}
	if len(samples) == 0 {
		return Curve{Threshold: threshold}
	}
	min, max := samples[0], samples[0]
	for _, s := range samples[1:] {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}
	if max <= min {
		// Degenerate distribution — every observation at the same price.
		// Single-bucket histogram still holds N, lets callers render
		// "100% of samples at X" without dividing by zero.
		return Curve{Min: min, Max: max, Buckets: []int{len(samples)}, Threshold: threshold}
	}
	width := (max - min) / float64(buckets)
	out := Curve{Min: min, Max: max, Buckets: make([]int, buckets), Threshold: threshold}
	for _, s := range samples {
		idx := int((s - min) / width)
		if idx >= buckets {
			idx = buckets - 1
		}
		if idx < 0 {
			idx = 0
		}
		out.Buckets[idx]++
	}
	return out
}

// SeasonOf returns the calendar quarter ("Q1".."Q4"), kept here so the
// forecast caller does not have to import dealquality just to bucket
// by season. Behaviour matches dealquality.SeasonOf — keep them in
// sync if either changes.
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

func clamp01ToScore(p float64) int {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	return int(math.Round(p * 100))
}

func describe(currentPrice float64, samples []float64, below int, horizonProb float64) string {
	switch {
	case below == 0:
		return fmt.Sprintf("commit-now: current %.0f is the lowest of %d observations", currentPrice, len(samples))
	case horizonProb >= 0.80:
		return fmt.Sprintf("wait: %.0f%% chance of a lower price within horizon", horizonProb*100)
	case horizonProb <= 0.25:
		return fmt.Sprintf("commit-now: only %.0f%% chance of a lower price within horizon", horizonProb*100)
	default:
		return fmt.Sprintf("uncertain: %.0f%% chance of a lower price within horizon", horizonProb*100)
	}
}
