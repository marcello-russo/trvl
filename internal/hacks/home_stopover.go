package hacks

// MIK-3077: long-layover home-stopover detector.
//
// Scans flight-result connections for layovers of 12..48 hours at any
// of the user's home airports and marks them as "free accommodation
// at home" opportunities — a virtual saving equal to one avg hotel
// night per layover-day. Same detector also recognises publisher
// free-stopover programs (Icelandair / TAP / Finnair / Singapore /
// Air China) when the layover hub appears in the published table.

import (
	"fmt"
	"strings"
)

// LayoverConnection is the minimal data the detector needs about one
// stop in a multi-leg itinerary.
type LayoverConnection struct {
	// Airport is the IATA code where the user lays over.
	Airport string
	// Hours is the elapsed layover duration in hours (between previous
	// arrival and next departure).
	Hours float64
	// Carrier is the operating carrier of the next outbound leg, used
	// to look up publisher free-stopover programs.
	Carrier string
}

// HomeStopoverKind classifies the opportunity so callers can render
// distinct user-facing messages.
type HomeStopoverKind string

const (
	// HomeStopoverHome — layover at one of the user's home airports.
	HomeStopoverHome HomeStopoverKind = "home"
	// HomeStopoverPublisher — layover at a hub the operating carrier
	// allows as a free stopover (Icelandair / TAP / etc).
	HomeStopoverPublisher HomeStopoverKind = "publisher_free"
)

// HomeStopoverCandidate is one detected opportunity.
type HomeStopoverCandidate struct {
	Airport    string
	Hours      float64
	Carrier    string
	Kind       HomeStopoverKind
	SavingsEUR float64
	Reason     string
}

// Detector bounds. AC: layover >=12h surfaces; cap at 48h since beyond
// that the trip is effectively a separate visit, not a layover.
const (
	minStopoverHours = 12.0
	maxStopoverHours = 48.0
)

// publisherFreeStopover is the published list of carriers that grant a
// free stopover at their hub. Free here means "no extra airfare for
// the stopover leg" — the user still pays for accommodation but flight
// continuity is preserved.
var publisherFreeStopover = map[string]string{
	"FI": "REK", // Icelandair → KEF
	"TP": "LIS", // TAP Portugal → Lisbon
	"AY": "HEL", // Finnair → Helsinki
	"SQ": "SIN", // Singapore Airlines → Singapore
	"CA": "PEK", // Air China → Beijing
	"OS": "VIE", // Austrian → Vienna
}

// DetectHomeStopover scans `connections` and returns candidates that
// either (a) lay over at one of the user's `homeAirports` for 12-48h,
// or (b) match a publisher-free-stopover route. Sorted by Hours
// ascending for deterministic output.
//
// `avgHotelNightEUR` is the user's recent average hotel-night cost —
// used as the per-night virtual saving figure on home-stopover hits.
func DetectHomeStopover(connections []LayoverConnection, homeAirports []string, avgHotelNightEUR float64) []HomeStopoverCandidate {
	if len(connections) == 0 {
		return nil
	}
	homes := make(map[string]struct{}, len(homeAirports))
	for _, a := range homeAirports {
		homes[strings.ToUpper(strings.TrimSpace(a))] = struct{}{}
	}
	if avgHotelNightEUR < 0 {
		avgHotelNightEUR = 0
	}
	var out []HomeStopoverCandidate
	for _, c := range connections {
		if c.Hours < minStopoverHours || c.Hours > maxStopoverHours {
			continue
		}
		ap := strings.ToUpper(strings.TrimSpace(c.Airport))

		// Home-stopover takes priority over publisher-free since the
		// user-value is real (free hotel) vs nominal (continuity).
		if _, ok := homes[ap]; ok {
			nights := nightsCovered(c.Hours)
			out = append(out, HomeStopoverCandidate{
				Airport:    ap,
				Hours:      c.Hours,
				Carrier:    c.Carrier,
				Kind:       HomeStopoverHome,
				SavingsEUR: avgHotelNightEUR * float64(nights),
				Reason:     fmt.Sprintf("layover at home (%s) for %.1fh ≈ %d free hotel night(s)", ap, c.Hours, nights),
			})
			continue
		}
		if hub, ok := publisherFreeStopover[strings.ToUpper(strings.TrimSpace(c.Carrier))]; ok && hub == ap {
			out = append(out, HomeStopoverCandidate{
				Airport: ap,
				Hours:   c.Hours,
				Carrier: c.Carrier,
				Kind:    HomeStopoverPublisher,
				// Publisher free-stopover saves the cost of a separate
				// inbound flight to the hub, conservatively scored at
				// one hotel-night equivalent so it ranks below a true
				// home-stopover.
				SavingsEUR: avgHotelNightEUR,
				Reason:     fmt.Sprintf("%s allows free stopover at %s — no fare uplift for the layover", c.Carrier, ap),
			})
		}
	}
	return out
}

// nightsCovered estimates how many overnight stays a given layover
// duration spans. Conservative rounding: a 12h layover stretching
// midnight is one night; a 36h layover is two; a 48h layover is two.
func nightsCovered(hours float64) int {
	if hours < minStopoverHours {
		return 0
	}
	// Floor((hours - 4) / 24) + 1, where the -4h slack accounts for
	// arrival in late afternoon / departure in early morning bracketing
	// the same overnight stay.
	n := int((hours-4)/24) + 1
	if n < 1 {
		n = 1
	}
	return n
}
