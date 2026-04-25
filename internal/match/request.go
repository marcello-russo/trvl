// Package match computes deterministic scores describing how well a
// returned trip option matches what the user actually asked for.
//
// This file holds the RequestMatch axis (MIK-3063): "how close is the
// offered itinerary to the user's literal ask?" — orthogonal to the
// future ProfileMatch axis (MIK-3062) which scores against the user's
// long-lived preferences. RequestMatch is pure, has no I/O, and is
// safe to call from any goroutine.
package match

import (
	"fmt"
	"strings"
	"time"
)

// Request describes what the user actually asked for. Zero values for
// optional fields mean "the user did not constrain this axis"; such axes
// contribute no penalty.
type Request struct {
	// DepartDate is the user's literal anchor depart date. When the user
	// supplied a range, this is the window center. Zero = no date anchor.
	DepartDate time.Time
	// DateWindowDays is the symmetric flex (in days) the user explicitly
	// allowed around DepartDate. The first DateWindowDays of drift incur
	// no penalty; drift beyond it is penalised linearly. Zero = no flex
	// allowed (every day of drift counts).
	DateWindowDays int
	// PrimaryOrigin is the user's preferred origin airport (IATA). Empty
	// means "any origin is fine".
	PrimaryOrigin string
	// AcceptableOrigins is the full set of origins the user has marked as
	// substitutable for PrimaryOrigin (typically PrimaryOrigin's nearby
	// airports). PrimaryOrigin is implicitly part of this set.
	AcceptableOrigins []string
	// Nights is the user-requested trip duration. Zero = unspecified.
	Nights int
	// Currency is the user's requested currency. Empty = unspecified.
	Currency string
	// Guests is the requested number of guests. Zero = unspecified.
	Guests int
}

// Offered describes the itinerary that the system actually found and is
// about to return to the user.
type Offered struct {
	DepartDate time.Time
	Origin     string
	Nights     int
	Currency   string
	Guests     int
}

// Components records the contribution of each axis to the final score.
// Exposed so callers can render a detailed "why" tooltip without having
// to re-derive the breakdown.
type Components struct {
	DateDriftDays       int
	AirportSubstitution bool
	NightsDrift         int
	CurrencyMismatch    bool
	GuestDelta          int
}

// Score is the result of a single RequestMatch evaluation.
type Score struct {
	// Total is the final score on 0..100. 100 = exact match on every
	// non-zero axis; 0 = floor (penalties capped so we never go negative).
	Total int
	// Reason names the axis that subtracted the most points. Empty when
	// every axis was either unconstrained or matched exactly.
	Reason     string
	Components Components
}

// Per-axis penalty weights. These are conservative: the AC requires that
// 50 ≈ "significant flex applied", so the dominant axes (airport
// substitution, currency mismatch) cap out around -25/-30 each, and a
// week of date drift adds another -28. Tuned by eyeballing the worst
// realistic case (different airport + 7-day drift) ≈ score 47.
const (
	// dateDriftPenaltyPerDay is the hit per day of drift beyond the
	// user's explicit DateWindowDays.
	dateDriftPenaltyPerDay = 4
	// airportSubstitutionPenalty applies when Offered.Origin is in
	// AcceptableOrigins but is not PrimaryOrigin.
	airportSubstitutionPenalty = 25
	// airportRejectedPenalty applies when Offered.Origin is not in the
	// AcceptableOrigins set at all (a hard miss).
	airportRejectedPenalty = 50
	// nightsDriftPenaltyPerNight is the hit per night of duration drift
	// from the user's requested Nights.
	nightsDriftPenaltyPerNight = 5
	// currencyMismatchPenalty applies when the offered currency differs
	// from the requested currency.
	currencyMismatchPenalty = 10
	// guestDeltaPenaltyPer is the hit per guest of difference between
	// the requested and offered guest counts.
	guestDeltaPenaltyPer = 10
	// scoreFloor caps how low a penalised score may go.
	scoreFloor = 0
	// scoreMax is the perfect-match anchor.
	scoreMax = 100
)

// Compute returns the RequestMatch score for one (Request, Offered)
// pair. Pure: no I/O, deterministic, safe across goroutines.
func Compute(req Request, off Offered) Score {
	var sc Score
	sc.Total = scoreMax

	type axis struct {
		name    string
		penalty int
	}
	var axes []axis

	// Date drift. Skipped when the user did not anchor a depart date.
	if !req.DepartDate.IsZero() && !off.DepartDate.IsZero() {
		drift := dayDelta(req.DepartDate, off.DepartDate)
		excess := drift - req.DateWindowDays
		if excess < 0 {
			excess = 0
		}
		sc.Components.DateDriftDays = drift
		if excess > 0 {
			p := excess * dateDriftPenaltyPerDay
			sc.Total -= p
			axes = append(axes, axis{"date_drift", p})
		}
	}

	// Airport substitution / rejection. Skipped when the user accepted
	// any origin (PrimaryOrigin == "").
	if req.PrimaryOrigin != "" && off.Origin != "" {
		if !equalIATA(req.PrimaryOrigin, off.Origin) {
			if originAcceptable(off.Origin, req.AcceptableOrigins, req.PrimaryOrigin) {
				sc.Components.AirportSubstitution = true
				sc.Total -= airportSubstitutionPenalty
				axes = append(axes, axis{"airport_substitution", airportSubstitutionPenalty})
			} else {
				sc.Components.AirportSubstitution = true
				sc.Total -= airportRejectedPenalty
				axes = append(axes, axis{"airport_rejected", airportRejectedPenalty})
			}
		}
	}

	// Nights drift. Skipped when the user did not specify Nights.
	if req.Nights > 0 && off.Nights > 0 && req.Nights != off.Nights {
		delta := absInt(req.Nights - off.Nights)
		sc.Components.NightsDrift = delta
		p := delta * nightsDriftPenaltyPerNight
		sc.Total -= p
		axes = append(axes, axis{"nights_drift", p})
	}

	// Currency mismatch.
	if req.Currency != "" && off.Currency != "" &&
		!strings.EqualFold(strings.TrimSpace(req.Currency), strings.TrimSpace(off.Currency)) {
		sc.Components.CurrencyMismatch = true
		sc.Total -= currencyMismatchPenalty
		axes = append(axes, axis{"currency_mismatch", currencyMismatchPenalty})
	}

	// Guest count mismatch. Skipped when either side did not specify.
	if req.Guests > 0 && off.Guests > 0 && req.Guests != off.Guests {
		delta := absInt(req.Guests - off.Guests)
		sc.Components.GuestDelta = delta
		p := delta * guestDeltaPenaltyPer
		sc.Total -= p
		axes = append(axes, axis{"guest_count_mismatch", p})
	}

	if sc.Total < scoreFloor {
		sc.Total = scoreFloor
	}

	// Pick the dominant penalty axis (largest negative contributor) for
	// the user-facing reason string. Stable — first axis wins ties.
	var dominant axis
	for _, a := range axes {
		if a.penalty > dominant.penalty {
			dominant = a
		}
	}
	if dominant.penalty > 0 {
		sc.Reason = describeReason(dominant, sc.Components)
	}
	return sc
}

// dayDelta returns the absolute difference between two dates in calendar
// days, ignoring the time-of-day component.
func dayDelta(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	da := time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)
	db := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	delta := int(db.Sub(da).Hours() / 24)
	if delta < 0 {
		delta = -delta
	}
	return delta
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// equalIATA compares two IATA-style codes case-insensitively after
// trimming whitespace. Returns false for empty inputs.
func equalIATA(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(a, b)
}

// originAcceptable returns true when off is in the substitutable set or
// equals the user's primary. The primary is always implicitly acceptable.
func originAcceptable(off string, acceptable []string, primary string) bool {
	if equalIATA(off, primary) {
		return true
	}
	for _, a := range acceptable {
		if equalIATA(off, a) {
			return true
		}
	}
	return false
}

func describeReason(dominant struct {
	name    string
	penalty int
}, c Components) string {
	switch dominant.name {
	case "date_drift":
		return fmt.Sprintf("date drifted %d day(s) from requested center", c.DateDriftDays)
	case "airport_substitution":
		return "alternate origin airport (substitutable)"
	case "airport_rejected":
		return "origin airport not in your accepted set"
	case "nights_drift":
		return fmt.Sprintf("%d night(s) different from requested duration", c.NightsDrift)
	case "currency_mismatch":
		return "result quoted in a different currency than requested"
	case "guest_count_mismatch":
		return fmt.Sprintf("%d guest(s) different from your booking party", c.GuestDelta)
	default:
		return ""
	}
}
