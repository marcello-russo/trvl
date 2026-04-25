package match

// MIK-3063: tests for RequestMatch scoring. Each axis (date, airport,
// nights, currency, guests) is exercised in isolation, then a combined
// case verifies penalties stack and the dominant-axis reason wins.

import (
	"strings"
	"testing"
	"time"
)

var anchor = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

func TestCompute_PerfectMatch(t *testing.T) {
	req := Request{
		DepartDate:    anchor,
		PrimaryOrigin: "AMS",
		Nights:        4,
		Currency:      "EUR",
		Guests:        2,
	}
	off := Offered{
		DepartDate: anchor,
		Origin:     "AMS",
		Nights:     4,
		Currency:   "EUR",
		Guests:     2,
	}
	sc := Compute(req, off)
	if sc.Total != 100 {
		t.Errorf("perfect match: total = %d, want 100", sc.Total)
	}
	if sc.Reason != "" {
		t.Errorf("perfect match: reason = %q, want empty", sc.Reason)
	}
}

func TestCompute_AllZeroRequest_NoPenalty(t *testing.T) {
	// User specified no constraints — every offered itinerary should score 100.
	off := Offered{
		DepartDate: anchor,
		Origin:     "EIN",
		Nights:     7,
		Currency:   "USD",
		Guests:     5,
	}
	sc := Compute(Request{}, off)
	if sc.Total != 100 {
		t.Errorf("zero-request: total = %d, want 100", sc.Total)
	}
}

func TestCompute_DateDriftBeyondWindow(t *testing.T) {
	cases := []struct {
		name       string
		windowDays int
		driftDays  int
		wantTotal  int
		wantReason bool
	}{
		{"in window no penalty", 3, 2, 100, false},
		{"exactly at window edge", 3, 3, 100, false},
		{"one day past window", 3, 4, 100 - 1*dateDriftPenaltyPerDay, true},
		{"week past window", 0, 7, 100 - 7*dateDriftPenaltyPerDay, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{DepartDate: anchor, DateWindowDays: tc.windowDays}
			off := Offered{DepartDate: anchor.AddDate(0, 0, tc.driftDays)}
			sc := Compute(req, off)
			if sc.Total != tc.wantTotal {
				t.Errorf("total = %d, want %d", sc.Total, tc.wantTotal)
			}
			if (sc.Reason != "") != tc.wantReason {
				t.Errorf("reason = %q, wantReason=%v", sc.Reason, tc.wantReason)
			}
			if sc.Components.DateDriftDays != tc.driftDays {
				t.Errorf("DateDriftDays = %d, want %d", sc.Components.DateDriftDays, tc.driftDays)
			}
		})
	}
}

func TestCompute_AirportSubstitution_AcceptedNeighbor(t *testing.T) {
	req := Request{PrimaryOrigin: "AMS", AcceptableOrigins: []string{"EIN", "RTM"}}
	off := Offered{Origin: "EIN"}
	sc := Compute(req, off)
	if sc.Total != 100-airportSubstitutionPenalty {
		t.Errorf("substitution: total = %d, want %d", sc.Total, 100-airportSubstitutionPenalty)
	}
	if !sc.Components.AirportSubstitution {
		t.Errorf("AirportSubstitution flag not set")
	}
	if !strings.Contains(sc.Reason, "alternate origin") {
		t.Errorf("reason = %q, want mention 'alternate origin'", sc.Reason)
	}
}

func TestCompute_AirportRejected_NotInAcceptableSet(t *testing.T) {
	req := Request{PrimaryOrigin: "AMS", AcceptableOrigins: []string{"EIN"}}
	off := Offered{Origin: "BRU"}
	sc := Compute(req, off)
	if sc.Total != 100-airportRejectedPenalty {
		t.Errorf("rejected: total = %d, want %d", sc.Total, 100-airportRejectedPenalty)
	}
	if !strings.Contains(sc.Reason, "not in your accepted set") {
		t.Errorf("reason = %q, want hard-miss text", sc.Reason)
	}
}

func TestCompute_AirportCaseInsensitive(t *testing.T) {
	req := Request{PrimaryOrigin: "ams"}
	off := Offered{Origin: "AMS"}
	sc := Compute(req, off)
	if sc.Total != 100 {
		t.Errorf("case-folded match: total = %d, want 100", sc.Total)
	}
}

func TestCompute_NightsDrift(t *testing.T) {
	cases := []struct {
		reqNights, offNights int
		wantPenalty          int
	}{
		{0, 5, 0},      // unspecified → no penalty
		{4, 0, 0},      // unspecified offered → no penalty
		{4, 4, 0},      // exact
		{4, 5, 1 * nightsDriftPenaltyPerNight},
		{7, 4, 3 * nightsDriftPenaltyPerNight},
	}
	for _, tc := range cases {
		req := Request{Nights: tc.reqNights}
		off := Offered{Nights: tc.offNights}
		sc := Compute(req, off)
		if sc.Total != 100-tc.wantPenalty {
			t.Errorf("req=%d off=%d: total = %d, want %d",
				tc.reqNights, tc.offNights, sc.Total, 100-tc.wantPenalty)
		}
	}
}

func TestCompute_CurrencyMismatch(t *testing.T) {
	req := Request{Currency: "EUR"}
	off := Offered{Currency: "USD"}
	sc := Compute(req, off)
	if sc.Total != 100-currencyMismatchPenalty {
		t.Errorf("currency mismatch: total = %d, want %d", sc.Total, 100-currencyMismatchPenalty)
	}

	// Same currency case-insensitive.
	off.Currency = "eur"
	sc = Compute(req, off)
	if sc.Total != 100 {
		t.Errorf("case-insensitive currency: total = %d, want 100", sc.Total)
	}
}

func TestCompute_GuestCountDelta(t *testing.T) {
	req := Request{Guests: 2}
	off := Offered{Guests: 4}
	sc := Compute(req, off)
	if sc.Total != 100-2*guestDeltaPenaltyPer {
		t.Errorf("guest delta: total = %d, want %d", sc.Total, 100-2*guestDeltaPenaltyPer)
	}
	if sc.Components.GuestDelta != 2 {
		t.Errorf("GuestDelta = %d, want 2", sc.Components.GuestDelta)
	}
}

func TestCompute_StackedPenaltiesDominantWins(t *testing.T) {
	// Combined: 7 day drift, accepted-neighbour airport, nights mismatch.
	// Expected dominant = date_drift if 7d drift > airportSubstitutionPenalty.
	// 7 * 4 = 28 > 25 → date_drift wins.
	req := Request{
		DepartDate:    anchor,
		PrimaryOrigin: "AMS",
		AcceptableOrigins: []string{"EIN"},
		Nights:        4,
	}
	off := Offered{
		DepartDate: anchor.AddDate(0, 0, 7),
		Origin:     "EIN",
		Nights:     5,
	}
	sc := Compute(req, off)
	expected := 100 - 7*dateDriftPenaltyPerDay - airportSubstitutionPenalty - 1*nightsDriftPenaltyPerNight
	if sc.Total != expected {
		t.Errorf("stacked: total = %d, want %d", sc.Total, expected)
	}
	if !strings.Contains(sc.Reason, "date drifted 7") {
		t.Errorf("reason = %q, want date_drift dominant", sc.Reason)
	}
}

func TestCompute_FloorAtZero(t *testing.T) {
	// Maximum-pessimal scenario: every axis breached, deep into rejection.
	req := Request{
		DepartDate:    anchor,
		PrimaryOrigin: "AMS",
		AcceptableOrigins: []string{},
		Nights:        4,
		Currency:      "EUR",
		Guests:        2,
	}
	off := Offered{
		DepartDate: anchor.AddDate(0, 0, 30),
		Origin:     "BRU",
		Nights:     14,
		Currency:   "USD",
		Guests:     8,
	}
	sc := Compute(req, off)
	if sc.Total < 0 {
		t.Errorf("total went negative: %d", sc.Total)
	}
	if sc.Total != 0 {
		t.Errorf("expected score floored at 0, got %d", sc.Total)
	}
}

// TestCompute_PrimaryAirportInAcceptableList ensures matching the
// primary doesn't accidentally trigger the substitution penalty even
// when the primary is also in AcceptableOrigins.
func TestCompute_PrimaryAirportInAcceptableList(t *testing.T) {
	req := Request{PrimaryOrigin: "AMS", AcceptableOrigins: []string{"AMS", "EIN"}}
	off := Offered{Origin: "AMS"}
	sc := Compute(req, off)
	if sc.Total != 100 {
		t.Errorf("primary-also-in-acceptable: total = %d, want 100", sc.Total)
	}
}

func TestDayDelta_TimeOfDayIgnored(t *testing.T) {
	a := time.Date(2026, 5, 1, 23, 59, 0, 0, time.UTC)
	b := time.Date(2026, 5, 2, 0, 1, 0, 0, time.UTC)
	if got := dayDelta(a, b); got != 1 {
		t.Errorf("dayDelta time-of-day ignored: got %d, want 1", got)
	}
	if got := dayDelta(b, a); got != 1 {
		t.Errorf("dayDelta abs-value: got %d, want 1", got)
	}
}
