package hacks

// MIK-3078: hidden-city *matrix* search — generalises the existing
// detector (which scans only one origin+destination pair) into a full
// {origin in nearby_airports} × {hub_extension} grid so the optimizer
// can surface the cheapest hidden-city itinerary even when the user's
// home airport is not the cheapest origin.
//
// This file ships the pure expander + ranker only: it consumes priced
// HiddenCityOffer fixtures and returns ranked alternatives with cost,
// carrier, layover risk score, and a pre-filled booking URL. Live
// flight pricing + the MCP/CLI surface compose on top of this in a
// follow-up change so the math stays trivially testable.

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// HiddenCityOffer is one priced candidate in the matrix. Origin is the
// airport the user actually boards from (must be in their nearby
// allowlist). HubBeyond is the carrier destination listed on the
// ticket — the user disembarks at the intermediate Hub and skips the
// final leg, so HubBeyond must differ from Hub.
type HiddenCityOffer struct {
	Origin    string
	Hub       string
	HubBeyond string
	Carrier   string
	Price     float64
	Currency  string
	// CarryOnOnly is true when the offer guarantees no checked-baggage
	// transfer (boarding pass issued only to Hub). Hidden-city is
	// unsafe with checked bags because the airline routes them all the
	// way to HubBeyond.
	CarryOnOnly bool
	// SeparateTickets is true when the itinerary spans two PNRs — the
	// risk of a missed connection on the throwaway leg falls on the
	// user, and EU261 protections do not apply across the break.
	SeparateTickets bool
	// LayoverMinutes is the scheduled layover at Hub. Below 60 the
	// user has too little buffer for the disembark; above 240 the
	// layover itself becomes painful even when the disembark is safe.
	LayoverMinutes int
}

// HiddenCityCandidate is one ranked output of the matrix search.
type HiddenCityCandidate struct {
	Origin     string
	Hub        string
	HubBeyond  string
	Carrier    string
	Price      float64
	Currency   string
	SavingsEUR float64
	SavingsPct float64
	// LayoverRisk is a 0..100 score where 0 means comfortable and 100
	// means unsafe. Composite of layover length, carry-on enforcement,
	// ticket-separation. Below 25 = green; 25-50 = caution; 50+ =
	// decline unless risk_posture.hidden_city.acceptable is set.
	LayoverRisk int
	Reason      string
	BookingURL  string
}

// MatrixOptions tune the expander. Zero-value is a sensible baseline:
// permissive risk gate, no preference toggle, returns up to 3
// candidates per the AC.
type MatrixOptions struct {
	// AllowHiddenCity gates the entire response. Mirror the user's
	// preferences.RiskPosture.HiddenCity.Acceptable flag here so the
	// caller can short-circuit cheaply. When false, ExpandMatrix
	// returns nil regardless of other inputs.
	AllowHiddenCity bool
	// MaxLayoverRisk drops candidates whose composite risk score
	// exceeds the threshold. Defaults to 60 when zero — leaves the
	// "decline unless explicit" caution band visible.
	MaxLayoverRisk int
	// TopK clips the result. Defaults to 3 when zero (matches the AC).
	TopK int
	// DirectBaseline is the cheapest direct Origin->Hub price the
	// caller observed; used to compute SavingsEUR/SavingsPct. Zero
	// means no baseline available, in which case the candidate keeps
	// SavingsEUR at zero rather than reporting an invented figure.
	DirectBaseline float64
	// DepartDate is propagated into BookingURL templates (ISO 8601).
	DepartDate string
}

// ExpandMatrix takes a slice of priced offers (caller fans out the
// origin × hub-beyond grid via flights.SearchFlights and fixtures)
// and returns the top K candidates after risk-gating, sort, and
// savings recompute. Pure function — no I/O.
func ExpandMatrix(offers []HiddenCityOffer, opt MatrixOptions) []HiddenCityCandidate {
	if !opt.AllowHiddenCity {
		return nil
	}
	if len(offers) == 0 {
		return nil
	}
	if opt.MaxLayoverRisk == 0 {
		opt.MaxLayoverRisk = 60
	}
	if opt.TopK == 0 {
		opt.TopK = 3
	}
	out := make([]HiddenCityCandidate, 0, len(offers))
	for _, o := range offers {
		if !validOffer(o) {
			continue
		}
		risk := scoreLayoverRisk(o)
		if risk > opt.MaxLayoverRisk {
			continue
		}
		c := HiddenCityCandidate{
			Origin:      strings.ToUpper(strings.TrimSpace(o.Origin)),
			Hub:         strings.ToUpper(strings.TrimSpace(o.Hub)),
			HubBeyond:   strings.ToUpper(strings.TrimSpace(o.HubBeyond)),
			Carrier:     strings.ToUpper(strings.TrimSpace(o.Carrier)),
			Price:       o.Price,
			Currency:    o.Currency,
			LayoverRisk: risk,
			BookingURL:  bookingURLFor(o, opt.DepartDate),
		}
		c.Reason = describeRisk(o, risk, c.Carrier)
		applySavings(&c, opt.DirectBaseline)
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil
	}
	sort.SliceStable(out, func(i, j int) bool {
		// Prefer cheaper, then lower risk on ties so the deterministic
		// "best" candidate is the user's likely click.
		if out[i].Price != out[j].Price {
			return out[i].Price < out[j].Price
		}
		return out[i].LayoverRisk < out[j].LayoverRisk
	})
	if opt.TopK < len(out) {
		out = out[:opt.TopK]
	}
	return out
}

func validOffer(o HiddenCityOffer) bool {
	if o.Price <= 0 {
		return false
	}
	if strings.TrimSpace(o.Origin) == "" {
		return false
	}
	if strings.TrimSpace(o.Hub) == "" {
		return false
	}
	if strings.TrimSpace(o.HubBeyond) == "" {
		return false
	}
	if strings.EqualFold(o.Hub, o.HubBeyond) {
		// Throwaway leg must exist; identical codes are a degenerate
		// fixture that we silently drop rather than panic on.
		return false
	}
	if strings.EqualFold(o.Origin, o.Hub) {
		return false
	}
	return true
}

// scoreLayoverRisk composes three signals into one 0..100 score.
// Layover length: <60min adds 60 (likely missed at boarding-pass
// pickup); 60-90 adds 20; 90-180 adds 0; 180-240 adds 10; >240 adds
// 25. Not carry-on-only adds 35 (checked bag will fly to HubBeyond,
// defeating the whole hack). Separate tickets adds 20 (no protection
// across the disembark; the user owns any missed-connection redirect).
// Capped at 100 to keep the rendering layer simple.
func scoreLayoverRisk(o HiddenCityOffer) int {
	score := 0
	switch {
	case o.LayoverMinutes < 60:
		score += 60
	case o.LayoverMinutes <= 90:
		score += 20
	case o.LayoverMinutes <= 180:
		// comfort zone; no penalty
	case o.LayoverMinutes <= 240:
		score += 10
	default:
		score += 25
	}
	if !o.CarryOnOnly {
		score += 35
	}
	if o.SeparateTickets {
		score += 20
	}
	if score > 100 {
		score = 100
	}
	return score
}

func describeRisk(o HiddenCityOffer, risk int, carrier string) string {
	switch {
	case risk == 0:
		return fmt.Sprintf("clean disembark at %s; carry-on enforced; %d-min layover", o.Hub, o.LayoverMinutes)
	case risk < 25:
		return fmt.Sprintf("low-risk hidden-city via %s on %s", o.Hub, carrier)
	case risk < 50:
		return fmt.Sprintf("caution: layover %d min, carry-on %v, separate-tickets %v", o.LayoverMinutes, o.CarryOnOnly, o.SeparateTickets)
	default:
		return fmt.Sprintf("high-risk: %d/100 — only proceed if explicitly opted-in", risk)
	}
}

func applySavings(c *HiddenCityCandidate, baseline float64) {
	if baseline <= 0 {
		return
	}
	gap := baseline - c.Price
	if gap < 0 {
		gap = 0
	}
	c.SavingsEUR = gap
	c.SavingsPct = (gap / baseline) * 100
}

// bookingURLFor produces a deep-link the user can click straight
// through to a pre-filled booking page. Format is carrier-specific.
// Carriers without a template fall back to a Google Flights search
// URL, which is at least useful as a populated browse target.
func bookingURLFor(o HiddenCityOffer, departDate string) string {
	carrier := strings.ToUpper(strings.TrimSpace(o.Carrier))
	switch carrier {
	case "KL", "AF", "DL":
		// AFKLM and Delta share the Offers UI shape.
		v := url.Values{}
		v.Set("origin", o.Origin)
		v.Set("destination", o.HubBeyond)
		v.Set("date", departDate)
		v.Set("paxAdt", "1")
		v.Set("cabinClass", "ECONOMY")
		return "https://www.airfrance.com/booking/select?" + v.Encode()
	case "AY":
		v := url.Values{}
		v.Set("from", o.Origin)
		v.Set("to", o.HubBeyond)
		v.Set("departure", departDate)
		v.Set("adults", "1")
		return "https://www.finnair.com/en-fi/book?" + v.Encode()
	case "LH", "OS", "LX":
		// Lufthansa group share a search URL pattern.
		v := url.Values{}
		v.Set("flightOrigin", o.Origin)
		v.Set("flightDestination", o.HubBeyond)
		v.Set("departureDate", departDate)
		v.Set("paxAdt", "1")
		return "https://www.lufthansa.com/de/en/flight-search?" + v.Encode()
	default:
		// Google Flights deep-link as a universal fallback. The
		// fragment-style query is what the public site honours when
		// the user follows the link.
		v := url.Values{}
		v.Set("hl", "en")
		return fmt.Sprintf("https://www.google.com/travel/flights?%s#flt=%s.%s.%s",
			v.Encode(), o.Origin, o.HubBeyond, departDate)
	}
}
