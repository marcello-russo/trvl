// Package cards ranks the user's payment cards by post-rewards net cost
// for one specific booking. Lets the LLM answer "which card should I
// tap on this offer?" without leaking PII — the package operates
// purely on the user-supplied multipliers and FX-fee descriptors.
// (MIK-3083.)
package cards

import (
	"fmt"
	"sort"
	"strings"
)

// Card describes one payment card the user holds. Stored under
// preferences.PaymentCards by the user.
type Card struct {
	// Name is a free-form label such as "FB Amex Gold".
	Name string `json:"name"`
	// MCCMultipliers maps merchant-category keys (e.g. "travel",
	// "airline", "hotel", "default") to the points-per-EUR rate the card
	// earns on that category. Keys are case-insensitive at lookup time.
	MCCMultipliers map[string]float64 `json:"mcc_multipliers"`
	// PointValueEUR is how much one earned point is worth to this user
	// when redeemed (in EUR). Without it, multipliers cannot be compared
	// against fees in money terms.
	PointValueEUR float64 `json:"point_value_eur"`
	// IntroOffer is a short descriptor surfaced verbatim in reasoning.
	IntroOffer string `json:"intro_offer,omitempty"`
	// FXFeePct is the foreign-transaction fee as a percentage (e.g. 2.5
	// for 2.5%). Applied only when ForeignCurrency=true at scoring time.
	FXFeePct float64 `json:"fx_fee_pct,omitempty"`
}

// Booking describes the one transaction we are ranking against.
type Booking struct {
	// PriceEUR is the EUR-equivalent total price the user would pay.
	PriceEUR float64
	// MCC is the merchant-category key used to look up multipliers.
	MCC string
	// ForeignCurrency is true when the merchant settles in a non-home
	// currency, triggering the card's FXFeePct.
	ForeignCurrency bool
}

// Ranking is one card's score on a specific booking, sorted ascending
// by NetCostEUR by Rank.
type Ranking struct {
	Card           Card
	EarnedPoints   float64
	RewardValueEUR float64
	FXFeeEUR       float64
	NetCostEUR     float64
	Reasoning      string
}

// Rank computes a sorted []Ranking for `cards` against `booking`. The
// best card is at index 0. Cards with absent category multipliers fall
// back to the "default" key. Pure: no I/O, deterministic, safe for
// concurrent callers.
func Rank(cards []Card, booking Booking) []Ranking {
	out := make([]Ranking, 0, len(cards))
	for _, c := range cards {
		mult := lookupMultiplier(c.MCCMultipliers, booking.MCC)
		earned := booking.PriceEUR * mult
		reward := earned * c.PointValueEUR
		var fxFee float64
		if booking.ForeignCurrency {
			fxFee = booking.PriceEUR * c.FXFeePct / 100.0
		}
		net := booking.PriceEUR - reward + fxFee
		r := Ranking{
			Card:           c,
			EarnedPoints:   earned,
			RewardValueEUR: reward,
			FXFeeEUR:       fxFee,
			NetCostEUR:     net,
			Reasoning:      explain(c, booking, mult, reward, fxFee, net),
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].NetCostEUR < out[j].NetCostEUR
	})
	return out
}

// Recommend returns the top-ranked card in `rankings`. Returns nil when
// the list is empty. Convenience wrapper for callers that only need
// the "use this card" answer.
func Recommend(rankings []Ranking) *Ranking {
	if len(rankings) == 0 {
		return nil
	}
	r := rankings[0]
	return &r
}

// lookupMultiplier resolves the multiplier for `mcc` against the card's
// multiplier map, falling back to "default" then 1.0 when no key
// matches. Lookup is case-insensitive on the key.
func lookupMultiplier(m map[string]float64, mcc string) float64 {
	if m == nil {
		return 1.0
	}
	key := strings.ToLower(strings.TrimSpace(mcc))
	if key != "" {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if v, ok := m["default"]; ok {
		return v
	}
	return 1.0
}

// explain renders a short user-facing reasoning string. Mentions the
// multiplier hit, FX fee when applicable, and the intro offer when
// present.
func explain(c Card, booking Booking, mult, reward, fxFee, net float64) string {
	parts := []string{
		fmt.Sprintf("%s: earns %.2f pts (×%.2f on %s, %.4f EUR/pt → %.2f EUR)",
			c.Name, booking.PriceEUR*mult, mult, mccLabel(booking.MCC), c.PointValueEUR, reward),
	}
	if fxFee > 0 {
		parts = append(parts, fmt.Sprintf("FX fee +%.2f EUR (%.2f%%)", fxFee, c.FXFeePct))
	}
	parts = append(parts, fmt.Sprintf("net cost %.2f EUR", net))
	if c.IntroOffer != "" {
		parts = append(parts, fmt.Sprintf("(intro: %s)", c.IntroOffer))
	}
	return strings.Join(parts, "; ")
}

// mccLabel keeps an empty MCC readable in the reasoning string.
func mccLabel(mcc string) string {
	if strings.TrimSpace(mcc) == "" {
		return "uncategorised purchase"
	}
	return mcc
}
