// Package cabinarb implements the cabin-class arbitrage detector
// from MIK-3090 polish pass: when the premium-economy fare is less
// than 15%% over the economy fare on the same itinerary, the
// upgrade is a no-brainer and the planner should highlight it. The
// same threshold logic generalises to business-vs-premium and
// first-vs-business so a long-haul user paying near-flat upgrades
// gets a clean signal. Pure functions — no I/O — so the math is
// trivially testable; the wiring into mcp/tools_flights and the
// CLI render layer composes on top of this in a follow-up.
package cabinarb

import (
	"fmt"
	"sort"
	"strings"
)

// Cabin labels the four ICAO cabin tiers in the order an upgrade
// ladder visits them. Strings rather than ints so the storage shape
// matches what providers already return.
type Cabin string

const (
	CabinEconomy        Cabin = "economy"
	CabinPremiumEconomy Cabin = "premium_economy"
	CabinBusiness       Cabin = "business"
	CabinFirst          Cabin = "first"
)

// CabinFare is one priced offer in a particular cabin. Currency is
// informational; callers must pre-normalise mixed-currency offers
// before passing them in.
type CabinFare struct {
	Cabin    Cabin
	Price    float64
	Currency string
	Carrier  string
}

// UpgradeRecommendation is one recommended cabin shift the detector
// surfaced. The user's Baseline cabin is what they were quoted (or
// the cheapest available) and Target is the cabin the detector
// thinks they should consider given the threshold.
type UpgradeRecommendation struct {
	Baseline       Cabin
	Target         Cabin
	BaselinePrice  float64
	TargetPrice    float64
	UpsellAbsolute float64
	UpsellPercent  float64
	Reason         string
}

// DefaultUpsellThresholdPct is the AC-mandated 15%% — below this the
// upgrade is recommended. Exposed as a const so tests can validate
// the boundary; callers can override via DetectWithThreshold.
const DefaultUpsellThresholdPct = 15.0

// Detect runs the AC's "recommended" check against the supplied
// CabinFares for one itinerary: when premium-economy is within 15%%
// of economy, surface the upgrade. Pure function. Returns nil when
// the input lacks the needed cabin pair.
//
// Detect always evaluates economy -> premium-economy plus the two
// derived ladders (premium -> business, business -> first) using
// the same threshold so long-haul travellers see all near-flat
// upgrades the route offers.
func Detect(fares []CabinFare) []UpgradeRecommendation {
	return DetectWithThreshold(fares, DefaultUpsellThresholdPct)
}

// DetectWithThreshold is Detect with a caller-supplied cap.
// Threshold is in percent (15 means 15%%). Negative or zero
// threshold returns nil since "0%% upsell" is not a meaningful
// upgrade gate.
func DetectWithThreshold(fares []CabinFare, thresholdPct float64) []UpgradeRecommendation {
	if thresholdPct <= 0 {
		return nil
	}
	if len(fares) == 0 {
		return nil
	}
	cheapest := cheapestPerCabin(fares)
	out := make([]UpgradeRecommendation, 0, 3)
	for _, step := range upgradeLadder() {
		baseline, baseOK := cheapest[step.from]
		target, tgtOK := cheapest[step.to]
		if !baseOK || !tgtOK {
			continue
		}
		if baseline <= 0 || target <= 0 {
			continue
		}
		if target <= baseline {
			// Target priced at or below baseline — strictly cheaper; surface
			// as a 'no-brainer downgrade-resistant' upgrade.
			out = append(out, UpgradeRecommendation{
				Baseline:       step.from,
				Target:         step.to,
				BaselinePrice:  baseline,
				TargetPrice:    target,
				UpsellAbsolute: target - baseline,
				UpsellPercent:  0,
				Reason:         fmt.Sprintf("%s priced <= %s — strictly upgrade", step.to, step.from),
			})
			continue
		}
		gap := target - baseline
		gapPct := gap / baseline * 100
		if gapPct > thresholdPct {
			continue
		}
		out = append(out, UpgradeRecommendation{
			Baseline:       step.from,
			Target:         step.to,
			BaselinePrice:  baseline,
			TargetPrice:    target,
			UpsellAbsolute: gap,
			UpsellPercent:  gapPct,
			Reason:         fmt.Sprintf("%.1f%% upsell to %s — within %s%% threshold", gapPct, step.to, formatPct(thresholdPct)),
		})
	}
	if len(out) == 0 {
		return nil
	}
	// Best-value recommendation first: lowest UpsellPercent.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpsellPercent < out[j].UpsellPercent
	})
	return out
}

type ladderStep struct {
	from Cabin
	to   Cabin
}

func upgradeLadder() []ladderStep {
	return []ladderStep{
		{CabinEconomy, CabinPremiumEconomy},
		{CabinPremiumEconomy, CabinBusiness},
		{CabinBusiness, CabinFirst},
	}
}

func cheapestPerCabin(fares []CabinFare) map[Cabin]float64 {
	out := map[Cabin]float64{}
	for _, f := range fares {
		c := Cabin(strings.ToLower(strings.TrimSpace(string(f.Cabin))))
		if c == "" {
			continue
		}
		if f.Price <= 0 {
			continue
		}
		current, present := out[c]
		if !present || f.Price < current {
			out[c] = f.Price
		}
	}
	return out
}

func formatPct(pct float64) string {
	whole := int(pct + 0.5)
	if pct == float64(whole) {
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%.1f", pct)
}
