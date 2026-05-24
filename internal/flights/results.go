package flights

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/baggage"
	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

// currencyConverter converts amount from one currency to another, returning the
// converted amount and the resulting currency code. Injected for testability so
// the default offline test suite never hits the network.
type currencyConverter func(ctx context.Context, amount float64, from, to string) (float64, string)

// normalizeFlightCurrencies rewrites each flight's Price into the target currency
// so downstream resolution and ranking compare like with like. Conversion is
// best-effort: a flight is left unchanged if conversion fails or rates are
// unavailable. A no-op when target is empty or conv is nil.
func normalizeFlightCurrencies(ctx context.Context, flights []models.FlightResult, target string, conv currencyConverter) {
	if target == "" || conv == nil {
		return
	}
	for i := range flights {
		f := &flights[i]
		if f.Price <= 0 || f.Currency == "" || f.Currency == target {
			continue
		}
		amt, cur := conv(ctx, f.Price, f.Currency, target)
		if cur == target && amt > 0 {
			f.Price = amt
			f.Currency = target
		}
	}
}

const flightTimeLayout = "2006-01-02T15:04"

func mergeFlightResults(googleFlights, kiwiFlights, skiplaggedFlights []models.FlightResult, opts SearchOptions) []models.FlightResult {
	merged := make([]models.FlightResult, 0, len(googleFlights)+len(kiwiFlights)+len(skiplaggedFlights))
	merged = append(merged, googleFlights...)
	merged = append(merged, kiwiFlights...)
	merged = append(merged, skiplaggedFlights...)
	merged = filterFlightResults(merged, opts)
	// Collapse the same physical itinerary returned by multiple providers into
	// one result carrying every provider as a PriceSource (cheapest headline).
	merged = models.ResolveFlightSources(merged)
	// Carry-on-baseline comparable price (base fare + unavoidable carry-on fee on
	// overhead-only LCCs) so ranking is not fooled by bare LCC fares. FF benefits
	// + checked-bag preference refine this at the MCP layer (user profile).
	applyComparableBaseline(merged)
	sortFlightResults(merged, opts.SortBy)
	return merged
}

// applyComparableBaseline sets ComparablePrice for each flight using the
// no-status, carry-on-only all-in cost. Deterministic, no network/prefs.
func applyComparableBaseline(flights []models.FlightResult) {
	for i := range flights {
		f := &flights[i]
		if len(f.Legs) == 0 || f.Price <= 0 {
			continue
		}
		code := f.Legs[0].AirlineCode
		if code == "" {
			continue
		}
		allIn, breakdown := baggage.AllInCost(f.Price, code, false, true, nil)
		if allIn > f.Price {
			f.ComparablePrice = allIn
			f.ComparableBreakdown = breakdown
		}
	}
}

func filterFlightResults(flights []models.FlightResult, opts SearchOptions) []models.FlightResult {
	if len(flights) == 0 {
		return nil
	}

	filtered := make([]models.FlightResult, 0, len(flights))
	for _, f := range flights {
		if opts.MaxPrice > 0 && f.Price > float64(opts.MaxPrice) {
			continue
		}
		if opts.MaxDuration > 0 && f.Duration > opts.MaxDuration {
			continue
		}
		if opts.MaxStops == models.NonStop && f.Stops > 0 {
			continue
		}
		if opts.MaxStops == models.OneStop && f.Stops > 1 {
			continue
		}
		if !flightDepartsWithinWindow(f, opts.DepartAfter, opts.DepartBefore) {
			continue
		}
		filtered = append(filtered, f)
	}

	if len(opts.Airlines) > 0 {
		filtered = filterFlightsByAirline(filtered, opts.Airlines)
	}
	if opts.RequireCheckedBag {
		filtered = filterFlightsWithCheckedBag(filtered)
	}
	if len(opts.Alliances) > 0 {
		filtered = filterFlightsByAlliance(filtered, opts.Alliances)
	}

	return filtered
}

func filterFlightsByAirline(flights []models.FlightResult, airlines []string) []models.FlightResult {
	if len(flights) == 0 {
		return nil
	}

	want := make(map[string]bool, len(airlines))
	for _, airline := range airlines {
		code := strings.TrimSpace(strings.ToUpper(airline))
		if code != "" {
			want[code] = true
		}
	}
	if len(want) == 0 {
		return flights
	}

	filtered := flights[:0]
	for _, f := range flights {
		matched := false
		for _, leg := range f.Legs {
			if want[strings.ToUpper(leg.AirlineCode)] {
				matched = true
				break
			}
		}
		if matched {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func flightDepartsWithinWindow(f models.FlightResult, after, before string) bool {
	if after == "" && before == "" {
		return true
	}
	if len(f.Legs) == 0 || len(f.Legs[0].DepartureTime) < len("2006-01-02T15:04") {
		return false
	}

	clock := f.Legs[0].DepartureTime[len("2006-01-02T"):]
	if after != "" && clock < after {
		return false
	}
	if before != "" && clock > before {
		return false
	}
	return true
}

func sortFlightResults(flights []models.FlightResult, sortBy models.SortBy) {
	sort.SliceStable(flights, func(i, j int) bool {
		left := flights[i]
		right := flights[j]

		switch sortBy {
		case models.SortDuration:
			if left.Duration != right.Duration {
				return left.Duration < right.Duration
			}
		case models.SortDepartureTime:
			if cmp := compareFlightTimes(flightDeparture(left), flightDeparture(right)); cmp != 0 {
				return cmp < 0
			}
		case models.SortArrivalTime:
			if cmp := compareFlightTimes(flightArrival(left), flightArrival(right)); cmp != 0 {
				return cmp < 0
			}
		default:
			if cmp := compareFlightPrices(left.PriceForRanking(), right.PriceForRanking()); cmp != 0 {
				return cmp < 0
			}
		}

		if cmp := compareFlightPrices(left.PriceForRanking(), right.PriceForRanking()); cmp != 0 {
			return cmp < 0
		}
		if left.Duration != right.Duration {
			return left.Duration < right.Duration
		}
		if cmp := compareFlightTimes(flightDeparture(left), flightDeparture(right)); cmp != 0 {
			return cmp < 0
		}
		if routeCmp := strings.Compare(flightSortKey(left), flightSortKey(right)); routeCmp != 0 {
			return routeCmp < 0
		}
		return strings.Compare(left.Provider, right.Provider) < 0
	})
}

func compareFlightPrices(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareFlightTimes(left, right time.Time) int {
	switch {
	case left.IsZero() && right.IsZero():
		return 0
	case left.IsZero():
		return 1
	case right.IsZero():
		return -1
	case left.Before(right):
		return -1
	case left.After(right):
		return 1
	default:
		return 0
	}
}

func flightDeparture(f models.FlightResult) time.Time {
	if len(f.Legs) == 0 {
		return time.Time{}
	}
	t, _ := time.Parse(flightTimeLayout, f.Legs[0].DepartureTime)
	return t
}

func flightArrival(f models.FlightResult) time.Time {
	if len(f.Legs) == 0 {
		return time.Time{}
	}
	t, _ := time.Parse(flightTimeLayout, f.Legs[len(f.Legs)-1].ArrivalTime)
	return t
}

func flightSortKey(f models.FlightResult) string {
	if len(f.Legs) == 0 {
		return ""
	}

	parts := []string{f.Legs[0].DepartureAirport.Code}
	for _, leg := range f.Legs {
		parts = append(parts, leg.ArrivalAirport.Code)
	}
	return strings.Join(parts, "->")
}

func flightSearchCurrency(result *models.FlightSearchResult) string {
	if result != nil {
		for _, f := range result.Flights {
			if f.Currency != "" {
				return f.Currency
			}
		}
	}
	return "EUR"
}

func tripTypeForSearch(opts SearchOptions) string {
	if opts.ReturnDate != "" {
		return "round_trip"
	}
	return "one_way"
}

func kiwiSearchEligible(client *batchexec.Client, opts SearchOptions) bool {
	if client == nil || client != batchexec.SharedClient() {
		return false
	}
	return kiwiEligibleOptions(opts)
}

// skiplaggedSearchEligible mirrors kiwiSearchEligible's client guard so the
// Skiplagged provider only fires under the production shared client.
// Tests that inject a custom batchexec client via SearchFlightsWithClient
// auto-skip Skiplagged, preserving deterministic offline test behaviour
// (matches Kiwi's existing pattern; see PR #N for context).
func skiplaggedSearchEligible(client *batchexec.Client, opts SearchOptions) bool {
	if client == nil || client != batchexec.SharedClient() {
		return false
	}
	return skiplaggedEligibleOptions(opts)
}

func skiplaggedEligibleOptions(opts SearchOptions) bool {
	// Skiplagged supports return dates, cabin class, max stops, sort, and
	// adults — but not airline/alliance filters or baggage requirements.
	if len(opts.Airlines) > 0 || len(opts.Alliances) > 0 {
		return false
	}
	if opts.CarryOnBags > 0 || opts.CheckedBags > 0 || opts.RequireCheckedBag {
		return false
	}
	if opts.ExcludeBasic || opts.LessEmissions {
		return false
	}
	return true
}

func kiwiEligibleOptions(opts SearchOptions) bool {
	if opts.ReturnDate != "" {
		return false
	}
	if len(opts.Airlines) > 0 || len(opts.Alliances) > 0 {
		return false
	}
	if opts.CarryOnBags > 0 || opts.CheckedBags > 0 {
		return false
	}
	if opts.RequireCheckedBag || opts.ExcludeBasic || opts.LessEmissions {
		return false
	}
	return true
}
